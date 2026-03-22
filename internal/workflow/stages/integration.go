package stages

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/google/uuid"
)

// IntegrationConfig configures an integration stage (Stage 5 or Stage 12).
type IntegrationConfig struct {
	DocumentType       string
	StageID            string
	PromptTemplateName string
	SynthesisStageID   string
}

// PRDIntegrationConfig returns config for Stage 5.
func PRDIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		DocumentType:       "prd",
		StageID:            "prd_integration",
		PromptTemplateName: "OPUS_PRD_INTEGRATION_V1",
		SynthesisStageID:   "prd_synthesis",
	}
}

// PlanIntegrationConfig returns config for Stage 12.
func PlanIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		DocumentType:       "plan",
		StageID:            "plan_integration",
		PromptTemplateName: "OPUS_PLAN_INTEGRATION_V1",
		SynthesisStageID:   "plan_synthesis",
	}
}

// IntegrationResult holds the outcome of an integration stage.
type IntegrationResult struct {
	RunID            string `json:"run_id"`
	ArtifactID       string `json:"artifact_id"`
	VersionLabel     string `json:"version_label"`
	AgreementCount   int    `json:"agreement_count"`
	DisagreementCount int   `json:"disagreement_count"`
	HasDisagreements bool   `json:"has_disagreements"`
}

// ExecuteIntegration runs an integration stage (Stage 5 or 12).
func ExecuteIntegration(
	ctx context.Context,
	db *sql.DB,
	provider models.Provider,
	config IntegrationConfig,
	projectID string,
	modelConfigID string,
	logger *slog.Logger,
) (*IntegrationResult, error) {
	runRepo := workflow.NewRunRepository(db)

	run, err := runRepo.Create(ctx, projectID, config.StageID, modelConfigID)
	if err != nil {
		return nil, fmt.Errorf("creating integration run: %w", err)
	}

	if err := runRepo.UpdateStatus(ctx, run.ID, workflow.RunRunning); err != nil {
		return nil, fmt.Errorf("starting integration run: %w", err)
	}

	tools := models.IntegrationTools()
	toolDefs := make([]models.ToolDefinition, len(tools))
	for i, t := range tools {
		toolDefs[i] = models.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Required:    t.Required,
		}
	}

	req := models.SessionRequest{
		ModelID: provider.Models()[0].ModelID,
		Tools:   toolDefs,
		Messages: []models.Message{
			{Role: "user", Content: fmt.Sprintf("Integrate the %s revisions for project %s", config.DocumentType, projectID)},
		},
	}

	resp, err := provider.Execute(ctx, req)
	if err != nil {
		runRepo.SetError(ctx, run.ID, err.Error())
		runRepo.UpdateStatus(ctx, run.ID, workflow.RunFailed)
		return nil, fmt.Errorf("provider execution failed: %w", err)
	}

	runRepo.SetProviderRequestID(ctx, run.ID, resp.ProviderID)

	result := &IntegrationResult{RunID: run.ID}
	var submittedContent string
	now := time.Now().UTC().Format(time.RFC3339Nano)

	for _, tc := range resp.ToolCalls {
		switch tc.Name {
		case "submit_document":
			if content, ok := tc.Arguments["content"].(string); ok {
				submittedContent = content
			}

		case "report_agreement":
			result.AgreementCount++
			eventID := uuid.New().String()
			db.ExecContext(ctx,
				`INSERT INTO workflow_events (id, project_id, workflow_run_id, event_type, payload_json, created_at)
				 VALUES (?, ?, ?, 'agreement_reported', '{}', ?)`,
				eventID, projectID, run.ID, now)

		case "report_disagreement":
			result.DisagreementCount++
			// Create a review_item linked to the fragment.
			reviewID := uuid.New().String()
			fragID, _ := tc.Arguments["fragment_id"].(string)
			summary, _ := tc.Arguments["summary"].(string)
			db.ExecContext(ctx,
				`INSERT INTO review_items (id, project_id, stage, change_id, fragment_id, classification, summary, status, created_at)
				 VALUES (?, ?, ?, ?, ?, 'disagreement', ?, 'pending', ?)`,
				reviewID, projectID, config.StageID, tc.ID, fragID, summary, now)

			eventID := uuid.New().String()
			db.ExecContext(ctx,
				`INSERT INTO workflow_events (id, project_id, workflow_run_id, event_type, payload_json, created_at)
				 VALUES (?, ?, ?, 'disagreement_reported', '{}', ?)`,
				eventID, projectID, run.ID, now)
		}
	}

	if submittedContent == "" {
		runRepo.SetError(ctx, run.ID, "model did not call submit_document")
		runRepo.UpdateStatus(ctx, run.ID, workflow.RunFailed)
		return nil, fmt.Errorf("integration failed: model did not submit a document")
	}

	// Create artifact.
	artifactID := uuid.New().String()
	var artCount int
	db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifacts WHERE project_id = ? AND artifact_type = ?`,
		projectID, config.DocumentType).Scan(&artCount)
	versionLabel := fmt.Sprintf("%s.v%02d.integrated", config.DocumentType, artCount+1)

	_, err = db.ExecContext(ctx,
		`INSERT INTO artifacts (id, project_id, artifact_type, version_label, source_stage, source_model, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		artifactID, projectID, config.DocumentType, versionLabel, config.StageID, "opus", now)
	if err != nil {
		return nil, fmt.Errorf("creating integration artifact: %w", err)
	}

	// Record lineage.
	sourceRows, _ := db.QueryContext(ctx,
		`SELECT id FROM artifacts WHERE project_id = ? AND source_stage = ?`,
		projectID, config.SynthesisStageID)
	if sourceRows != nil {
		defer sourceRows.Close()
		for sourceRows.Next() {
			var sourceID string
			sourceRows.Scan(&sourceID)
			relID := uuid.New().String()
			db.ExecContext(ctx,
				`INSERT INTO artifact_relations (id, artifact_id, related_artifact_id, relation_type, created_at)
				 VALUES (?, ?, ?, 'integrated_from', ?)`,
				relID, artifactID, sourceID, now)
		}
	}

	// Record usage.
	usageID := uuid.New().String()
	db.ExecContext(ctx,
		`INSERT INTO usage_records (id, workflow_run_id, provider, model_name, input_tokens, output_tokens, recorded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		usageID, run.ID, string(provider.Name()), provider.Models()[0].ModelID,
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, now)

	runRepo.UpdateStatus(ctx, run.ID, workflow.RunCompleted)

	result.ArtifactID = artifactID
	result.VersionLabel = versionLabel
	result.HasDisagreements = result.DisagreementCount > 0

	logger.Info("integration completed",
		"run_id", run.ID,
		"artifact_id", artifactID,
		"document_type", config.DocumentType,
		"agreements", result.AgreementCount,
		"disagreements", result.DisagreementCount,
	)

	return result, nil
}
