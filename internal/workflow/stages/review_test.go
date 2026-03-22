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

func setupReviewTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', 'prd_review', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-gpt', 'openai', 'gpt-4o', datetime('now'), datetime('now'))`)

	return tdb, ctx
}

func TestExecuteReview_WithOperations(t *testing.T) {
	tdb, ctx := setupReviewTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockGPT("")
	mock.SetOverride(mock.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "review-resp-1",
		ToolCalls: []models.ToolCall{
			{ID: "tc-1", Name: "update_fragment", Arguments: map[string]any{
				"fragment_id": "frag_001",
				"new_content": "Improved introduction with more context.",
				"rationale":   "Added missing background information",
			}},
			{ID: "tc-2", Name: "add_fragment", Arguments: map[string]any{
				"after_fragment_id": "frag_001",
				"heading":           "Prerequisites",
				"content":           "System requirements and dependencies.",
				"rationale":         "Missing prerequisite section",
			}},
			{ID: "tc-3", Name: "submit_review_summary", Arguments: map[string]any{
				"summary":      "Document improved with better intro and added prerequisites",
				"key_findings": []any{"Missing prerequisites section", "Weak introduction"},
			}},
		},
		Usage: models.UsageMetadata{PromptTokens: 6000, CompletionTokens: 1500},
	})

	result, err := ExecuteReview(ctx, tdb.DB, mock, PRDReviewConfig("gpt"), "p-1", "mc-gpt", logger)
	require.NoError(t, err)

	assert.Equal(t, 2, result.OperationCount)
	assert.False(t, result.NoChanges)
	assert.Equal(t, "update", result.Operations[0].Type)
	assert.Equal(t, "add", result.Operations[1].Type)
	assert.NotEmpty(t, result.ReviewSummary)
	assert.Len(t, result.KeyFindings, 2)

	// Verify fragment events recorded.
	var eventCount int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_events WHERE workflow_run_id = ?`, result.RunID).Scan(&eventCount)
	assert.Equal(t, 2, eventCount) // update + add
}

func TestExecuteReview_ZeroOperations(t *testing.T) {
	tdb, ctx := setupReviewTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockGPT("")
	mock.SetOverride(mock.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "review-resp-2",
		ToolCalls: []models.ToolCall{
			{ID: "tc-1", Name: "submit_review_summary", Arguments: map[string]any{
				"summary": "Document is well-structured, no changes warranted",
			}},
		},
		Usage: models.UsageMetadata{TotalTokens: 3000},
	})

	result, err := ExecuteReview(ctx, tdb.DB, mock, PRDReviewConfig("gpt"), "p-1", "mc-gpt", logger)
	require.NoError(t, err)
	assert.True(t, result.NoChanges)
	assert.Equal(t, 0, result.OperationCount)
	assert.Contains(t, result.ReviewSummary, "no changes")
}

func TestExecuteReview_PlanConfig(t *testing.T) {
	tdb, ctx := setupReviewTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockGPT("")

	result, err := ExecuteReview(ctx, tdb.DB, mock, PlanReviewConfig("gpt"), "p-1", "mc-gpt", logger)
	require.NoError(t, err)
	assert.NotEmpty(t, result.RunID)
}

func TestExecuteReview_OpusVariant(t *testing.T) {
	cfg := PRDReviewConfig("opus")
	assert.Equal(t, "OPUS_PRD_REVIEW_V1", cfg.PromptTemplateName)

	cfg = PlanReviewConfig("opus")
	assert.Equal(t, "OPUS_PLAN_REVIEW_V1", cfg.PromptTemplateName)
}
