package stages

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/providers"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIntegrationTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', 'plan_integration', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-opus', 'anthropic', 'claude-opus', datetime('now'), datetime('now'))`)
	// Add a fragment for disagreement references.
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('frag-arch', 'p-1', 'plan', 'Architecture', 2, datetime('now'))`)

	return tdb, ctx
}

func TestExecuteIntegration_PlanSuccess(t *testing.T) {
	tdb, ctx := setupIntegrationTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockOpus("")
	mock.SetOverride(mock.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "integ-resp-1",
		ToolCalls: []models.ToolCall{
			{ID: "tc-1", Name: "submit_document", Arguments: map[string]any{
				"content":        "## Architecture\n\nIntegrated architecture.\n",
				"change_summary": "Incorporated GPT synthesis improvements",
			}},
			{ID: "tc-2", Name: "report_agreement", Arguments: map[string]any{
				"fragment_id": "frag-arch",
				"category":    "wholeheartedly_agrees",
				"rationale":   "Strong architectural direction",
			}},
			{ID: "tc-3", Name: "report_disagreement", Arguments: map[string]any{
				"fragment_id":      "frag-arch",
				"severity":         "moderate",
				"summary":          "Missing error handling strategy",
				"rationale":        "The architecture should address failure modes",
				"suggested_change": "Add an error handling section",
			}},
		},
		Usage: models.UsageMetadata{PromptTokens: 8000, CompletionTokens: 3000, TotalTokens: 11000},
	})

	result, err := ExecuteIntegration(ctx, tdb.DB, mock, PlanIntegrationConfig(), "p-1", "mc-opus", logger)
	require.NoError(t, err)

	assert.NotEmpty(t, result.RunID)
	assert.NotEmpty(t, result.ArtifactID)
	assert.Contains(t, result.VersionLabel, "plan")
	assert.Contains(t, result.VersionLabel, "integrated")
	assert.Equal(t, 1, result.AgreementCount)
	assert.Equal(t, 1, result.DisagreementCount)
	assert.True(t, result.HasDisagreements)

	// Verify review_item created for disagreement.
	var reviewCount int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM review_items WHERE project_id = 'p-1' AND classification = 'disagreement'`).Scan(&reviewCount)
	assert.Equal(t, 1, reviewCount)
}

func TestExecuteIntegration_PRD_NoDisagreements(t *testing.T) {
	tdb, ctx := setupIntegrationTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockOpus("")
	mock.SetOverride(mock.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "integ-resp-2",
		ToolCalls: []models.ToolCall{
			{ID: "tc-1", Name: "submit_document", Arguments: map[string]any{
				"content":        "## Clean Document\n\nAll good.\n",
				"change_summary": "Approved as-is",
			}},
			{ID: "tc-2", Name: "report_agreement", Arguments: map[string]any{
				"fragment_id": "frag-arch",
				"category":    "wholeheartedly_agrees",
				"rationale":   "Everything looks great",
			}},
		},
		Usage: models.UsageMetadata{TotalTokens: 5000},
	})

	result, err := ExecuteIntegration(ctx, tdb.DB, mock, PRDIntegrationConfig(), "p-1", "mc-opus", logger)
	require.NoError(t, err)
	assert.False(t, result.HasDisagreements)
	assert.Equal(t, 1, result.AgreementCount)
	assert.Equal(t, 0, result.DisagreementCount)
}

func TestExecuteIntegration_NoDocument_Fails(t *testing.T) {
	tdb, ctx := setupIntegrationTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockOpus("")
	mock.SetOverride(mock.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "empty",
		ToolCalls:  nil,
		Usage:      models.UsageMetadata{TotalTokens: 100},
	})

	_, err := ExecuteIntegration(ctx, tdb.DB, mock, PlanIntegrationConfig(), "p-1", "mc-opus", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "did not submit a document")
}
