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

// ReviewConfig configures a review stage (Stage 7 or Stage 14).
type ReviewConfig struct {
	DocumentType       string
	StageID            string
	PromptTemplateName string
}

// PRDReviewConfig returns config for Stage 7 with GPT.
func PRDReviewConfig(modelFamily string) ReviewConfig {
	tmpl := "GPT_PRD_REVIEW_V1"
	if modelFamily == "opus" {
		tmpl = "OPUS_PRD_REVIEW_V1"
	}
	return ReviewConfig{
		DocumentType:       "prd",
		StageID:            "prd_review",
		PromptTemplateName: tmpl,
	}
}

// PlanReviewConfig returns config for Stage 14.
func PlanReviewConfig(modelFamily string) ReviewConfig {
	tmpl := "GPT_PLAN_REVIEW_V1"
	if modelFamily == "opus" {
		tmpl = "OPUS_PLAN_REVIEW_V1"
	}
	return ReviewConfig{
		DocumentType:       "plan",
		StageID:            "plan_review",
		PromptTemplateName: tmpl,
	}
}

// ReviewResult holds the outcome of a review stage.
type ReviewResult struct {
	RunID          string                     `json:"run_id"`
	Operations     []workflow.FragmentOperation `json:"operations"`
	ReviewSummary  string                     `json:"review_summary,omitempty"`
	KeyFindings    []string                   `json:"key_findings,omitempty"`
	OperationCount int                        `json:"operation_count"`
	NoChanges      bool                       `json:"no_changes"`
}

// ExecuteReview runs a review stage (Stage 7 or 14). Always uses a fresh
// session — never continues a prior context window.
func ExecuteReview(
	ctx context.Context,
	db *sql.DB,
	provider models.Provider,
	config ReviewConfig,
	projectID string,
	modelConfigID string,
	logger *slog.Logger,
) (*ReviewResult, error) {
	runRepo := workflow.NewRunRepository(db)

	run, err := runRepo.Create(ctx, projectID, config.StageID, modelConfigID)
	if err != nil {
		return nil, fmt.Errorf("creating review run: %w", err)
	}

	if err := runRepo.UpdateStatus(ctx, run.ID, workflow.RunRunning); err != nil {
		return nil, fmt.Errorf("starting review run: %w", err)
	}

	// Always fresh session for review.
	runRepo.SetSessionHandle(ctx, run.ID, "", "fresh")

	tools := models.ReviewTools()
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
			{Role: "user", Content: fmt.Sprintf("Review the %s for project %s", config.DocumentType, projectID)},
		},
	}

	resp, err := provider.Execute(ctx, req)
	if err != nil {
		runRepo.SetError(ctx, run.ID, err.Error())
		runRepo.UpdateStatus(ctx, run.ID, workflow.RunFailed)
		return nil, fmt.Errorf("provider execution failed: %w", err)
	}

	runRepo.SetProviderRequestID(ctx, run.ID, resp.ProviderID)

	result := &ReviewResult{RunID: run.ID}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	for _, tc := range resp.ToolCalls {
		switch tc.Name {
		case "update_fragment":
			fragID, _ := tc.Arguments["fragment_id"].(string)
			newContent, _ := tc.Arguments["new_content"].(string)
			rationale, _ := tc.Arguments["rationale"].(string)
			result.Operations = append(result.Operations, workflow.FragmentOperation{
				Type:       "update",
				FragmentID: fragID,
				NewContent: newContent,
				Rationale:  rationale,
			})
			recordFragmentEvent(ctx, db, projectID, run.ID, "fragment_updated", now)

		case "add_fragment":
			afterID, _ := tc.Arguments["after_fragment_id"].(string)
			heading, _ := tc.Arguments["heading"].(string)
			content, _ := tc.Arguments["content"].(string)
			rationale, _ := tc.Arguments["rationale"].(string)
			result.Operations = append(result.Operations, workflow.FragmentOperation{
				Type:            "add",
				AfterFragmentID: afterID,
				Heading:         heading,
				NewContent:      content,
				Rationale:       rationale,
			})
			recordFragmentEvent(ctx, db, projectID, run.ID, "fragment_added", now)

		case "remove_fragment":
			fragID, _ := tc.Arguments["fragment_id"].(string)
			rationale, _ := tc.Arguments["rationale"].(string)
			result.Operations = append(result.Operations, workflow.FragmentOperation{
				Type:       "remove",
				FragmentID: fragID,
				Rationale:  rationale,
			})
			recordFragmentEvent(ctx, db, projectID, run.ID, "fragment_removed", now)

		case "submit_review_summary":
			if s, ok := tc.Arguments["summary"].(string); ok {
				result.ReviewSummary = s
			}
			if findings, ok := tc.Arguments["key_findings"].([]any); ok {
				for _, f := range findings {
					if s, ok := f.(string); ok {
						result.KeyFindings = append(result.KeyFindings, s)
					}
				}
			}
		}
	}

	result.OperationCount = len(result.Operations)
	result.NoChanges = result.OperationCount == 0

	// Record usage.
	usageID := uuid.New().String()
	db.ExecContext(ctx,
		`INSERT INTO usage_records (id, workflow_run_id, provider, model_name, input_tokens, output_tokens, recorded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		usageID, run.ID, string(provider.Name()), provider.Models()[0].ModelID,
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, now)

	runRepo.UpdateStatus(ctx, run.ID, workflow.RunCompleted)

	logger.Info("review completed",
		"run_id", run.ID,
		"document_type", config.DocumentType,
		"operations", result.OperationCount,
		"no_changes", result.NoChanges,
		"has_summary", result.ReviewSummary != "",
	)

	return result, nil
}

func recordFragmentEvent(ctx context.Context, db *sql.DB, projectID, runID, eventType, now string) {
	eventID := uuid.New().String()
	db.ExecContext(ctx,
		`INSERT INTO workflow_events (id, project_id, workflow_run_id, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, '{}', ?)`,
		eventID, projectID, runID, eventType, now)
}
