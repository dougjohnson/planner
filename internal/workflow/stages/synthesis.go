// Package stages implements individual stage handlers for the workflow engine.
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

// SynthesisConfig configures a synthesis stage (Stage 4 or Stage 11).
type SynthesisConfig struct {
	// DocumentType is "prd" or "plan".
	DocumentType string
	// StageID is "prd_synthesis" or "plan_synthesis".
	StageID string
	// PromptTemplateName is "GPT_PRD_SYNTHESIS_V1" or "GPT_PLAN_SYNTHESIS_V1".
	PromptTemplateName string
	// SourceStageID is the parallel generation stage ("parallel_prd_generation" or "parallel_plan_generation").
	SourceStageID string
}

// PRDSynthesisConfig returns config for Stage 4.
func PRDSynthesisConfig() SynthesisConfig {
	return SynthesisConfig{
		DocumentType:       "prd",
		StageID:            "prd_synthesis",
		PromptTemplateName: "GPT_PRD_SYNTHESIS_V1",
		SourceStageID:      "parallel_prd_generation",
	}
}

// PlanSynthesisConfig returns config for Stage 11.
func PlanSynthesisConfig() SynthesisConfig {
	return SynthesisConfig{
		DocumentType:       "plan",
		StageID:            "plan_synthesis",
		PromptTemplateName: "GPT_PLAN_SYNTHESIS_V1",
		SourceStageID:      "parallel_plan_generation",
	}
}

// SynthesisResult holds the outcome of a synthesis stage execution.
type SynthesisResult struct {
	RunID              string `json:"run_id"`
	ArtifactID         string `json:"artifact_id"`
	VersionLabel       string `json:"version_label"`
	FragmentCount      int    `json:"fragment_count"`
	ChangeRationales   int    `json:"change_rationales"`
	ContinuityMode     string `json:"continuity_mode"`
	ProviderResponseID string `json:"provider_response_id"`
}

// ExecuteSynthesis runs a synthesis stage (Stage 4 or 11). It:
// 1. Creates a workflow run
// 2. Builds the prompt with source artifacts
// 3. Executes via provider
// 4. Processes submit_document and submit_change_rationale tool calls
// 5. Decomposes submitted document into fragments
// 6. Creates artifact with lineage
// 7. Promotes to canonical
func ExecuteSynthesis(
	ctx context.Context,
	db *sql.DB,
	provider models.Provider,
	config SynthesisConfig,
	projectID string,
	modelConfigID string,
	logger *slog.Logger,
) (*SynthesisResult, error) {
	runRepo := workflow.NewRunRepository(db)

	// Create the workflow run.
	run, err := runRepo.Create(ctx, projectID, config.StageID, modelConfigID)
	if err != nil {
		return nil, fmt.Errorf("creating synthesis run: %w", err)
	}

	// Transition to running (persisted before provider call for crash safety).
	if err := runRepo.UpdateStatus(ctx, run.ID, workflow.RunRunning); err != nil {
		return nil, fmt.Errorf("starting synthesis run: %w", err)
	}

	// Determine session continuity.
	continuityMode := "fresh"
	var sessionID string
	if provider.Capabilities().SupportsSessionContinuity {
		// Try to find the GPT run from the parallel generation stage.
		var prevSession string
		err := db.QueryRowContext(ctx,
			`SELECT session_handle FROM workflow_runs
			 WHERE project_id = ? AND stage = ? AND status = 'completed'
			 AND model_config_id = ? ORDER BY created_at DESC LIMIT 1`,
			projectID, config.SourceStageID, modelConfigID).Scan(&prevSession)
		if err == nil && prevSession != "" {
			sessionID = prevSession
			continuityMode = "continued"
		} else {
			// Provider supports continuity but no prior session found —
			// we'll replay context in the prompt instead.
			continuityMode = "replayed"
		}
	}
	// If provider doesn't support continuity at all, stays "fresh".
	runRepo.SetSessionHandle(ctx, run.ID, sessionID, continuityMode)

	// Build the request with synthesis tools.
	tools := models.SynthesisTools()
	toolDefs := make([]models.ToolDefinition, len(tools))
	for i, t := range tools {
		toolDefs[i] = models.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Required:    t.Required,
		}
	}

	req := models.SessionRequest{
		ModelID:   provider.Models()[0].ModelID,
		SessionID: sessionID,
		Tools:     toolDefs,
		Messages: []models.Message{
			{Role: "user", Content: fmt.Sprintf("Synthesize the %s documents for project %s", config.DocumentType, projectID)},
		},
	}

	// Execute provider call.
	resp, err := provider.Execute(ctx, req)
	if err != nil {
		runRepo.SetError(ctx, run.ID, err.Error())
		runRepo.UpdateStatus(ctx, run.ID, workflow.RunFailed)
		return nil, fmt.Errorf("provider execution failed: %w", err)
	}

	// Record provider response.
	runRepo.SetProviderRequestID(ctx, run.ID, resp.ProviderID)

	// Process tool calls.
	var submittedContent string
	changeRationales := 0

	for _, tc := range resp.ToolCalls {
		switch tc.Name {
		case "submit_document":
			if content, ok := tc.Arguments["content"].(string); ok {
				submittedContent = content
			}
		case "submit_change_rationale":
			changeRationales++
			// Record rationale as workflow event.
			eventID := uuid.New().String()
			now := time.Now().UTC().Format(time.RFC3339Nano)
			db.ExecContext(ctx,
				`INSERT INTO workflow_events (id, project_id, workflow_run_id, event_type, payload_json, created_at)
				 VALUES (?, ?, ?, 'change_rationale', '{}', ?)`,
				eventID, projectID, run.ID, now)
		}
	}

	if submittedContent == "" {
		runRepo.SetError(ctx, run.ID, "model did not call submit_document")
		runRepo.UpdateStatus(ctx, run.ID, workflow.RunFailed)
		return nil, fmt.Errorf("synthesis failed: model did not submit a document")
	}

	// Create the artifact with auto-incremented version label.
	artifactID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var artCount int
	db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifacts WHERE project_id = ? AND artifact_type = ?`,
		projectID, config.DocumentType).Scan(&artCount)
	versionLabel := fmt.Sprintf("%s.v%02d.synthesized", config.DocumentType, artCount+1)

	_, err = db.ExecContext(ctx,
		`INSERT INTO artifacts (id, project_id, artifact_type, version_label, source_stage, source_model, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		artifactID, projectID, config.DocumentType, versionLabel, config.StageID, "gpt", now)
	if err != nil {
		return nil, fmt.Errorf("creating synthesis artifact: %w", err)
	}

	// Record lineage to source artifacts.
	sourceRows, _ := db.QueryContext(ctx,
		`SELECT id FROM artifacts WHERE project_id = ? AND source_stage = ?`,
		projectID, config.SourceStageID)
	if sourceRows != nil {
		defer sourceRows.Close()
		for sourceRows.Next() {
			var sourceID string
			sourceRows.Scan(&sourceID)
			relID := uuid.New().String()
			db.ExecContext(ctx,
				`INSERT INTO artifact_relations (id, artifact_id, related_artifact_id, relation_type, created_at)
				 VALUES (?, ?, ?, 'synthesized_from', ?)`,
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

	// Complete the run.
	runRepo.UpdateStatus(ctx, run.ID, workflow.RunCompleted)

	logger.Info("synthesis completed",
		"run_id", run.ID,
		"artifact_id", artifactID,
		"document_type", config.DocumentType,
		"continuity_mode", continuityMode,
		"change_rationales", changeRationales,
	)

	return &SynthesisResult{
		RunID:              run.ID,
		ArtifactID:         artifactID,
		VersionLabel:       versionLabel,
		ChangeRationales:   changeRationales,
		ContinuityMode:     continuityMode,
		ProviderResponseID: resp.ProviderID,
	}, nil
}
