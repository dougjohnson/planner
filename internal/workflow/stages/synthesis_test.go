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

func setupSynthesisTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', 'prd_synthesis', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-gpt', 'openai', 'gpt-4o', datetime('now'), datetime('now'))`)

	return tdb, ctx
}

func TestExecuteSynthesis_PRD_Success(t *testing.T) {
	tdb, ctx := setupSynthesisTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockGPT("")
	// Override to return a proper synthesis response.
	mock.SetOverride(mock.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "synth-response-1",
		Text:       "Synthesized PRD",
		ToolCalls: []models.ToolCall{
			{ID: "tc-1", Name: "submit_document", Arguments: map[string]any{
				"content":        "## Introduction\n\nSynthesized intro.\n\n## Architecture\n\nSynthesized arch.\n",
				"change_summary": "Merged competing PRDs into unified version",
			}},
			{ID: "tc-2", Name: "submit_change_rationale", Arguments: map[string]any{
				"section_id":  "Architecture",
				"change_type": "modified",
				"rationale":   "Combined strengths of both approaches",
			}},
		},
		Usage: models.UsageMetadata{PromptTokens: 5000, CompletionTokens: 2000, TotalTokens: 7000},
	})

	result, err := ExecuteSynthesis(ctx, tdb.DB, mock, PRDSynthesisConfig(), "p-1", "mc-gpt", logger)
	require.NoError(t, err)

	assert.NotEmpty(t, result.RunID)
	assert.NotEmpty(t, result.ArtifactID)
	assert.Contains(t, result.VersionLabel, "prd")
	assert.Contains(t, result.VersionLabel, "synthesized")
	assert.Equal(t, 1, result.ChangeRationales)
	assert.Equal(t, "synth-response-1", result.ProviderResponseID)

	// Verify run completed.
	var runStatus string
	tdb.DB.QueryRowContext(ctx,
		`SELECT status FROM workflow_runs WHERE id = ?`, result.RunID).Scan(&runStatus)
	assert.Equal(t, "completed", runStatus)

	// Verify artifact created.
	var artType string
	tdb.DB.QueryRowContext(ctx,
		`SELECT artifact_type FROM artifacts WHERE id = ?`, result.ArtifactID).Scan(&artType)
	assert.Equal(t, "prd", artType)

	// Verify usage recorded.
	var inputTokens int
	tdb.DB.QueryRowContext(ctx,
		`SELECT input_tokens FROM usage_records WHERE workflow_run_id = ?`, result.RunID).Scan(&inputTokens)
	assert.Equal(t, 5000, inputTokens)
}

func TestExecuteSynthesis_Plan_Success(t *testing.T) {
	tdb, ctx := setupSynthesisTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockGPT("")

	result, err := ExecuteSynthesis(ctx, tdb.DB, mock, PlanSynthesisConfig(), "p-1", "mc-gpt", logger)
	require.NoError(t, err)
	assert.Contains(t, result.VersionLabel, "plan")
}

func TestExecuteSynthesis_NoDocument_Fails(t *testing.T) {
	tdb, ctx := setupSynthesisTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockGPT("")
	mock.SetOverride(mock.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "empty-response",
		Text:       "I couldn't generate anything",
		ToolCalls:  nil, // no tool calls
		Usage:      models.UsageMetadata{TotalTokens: 100},
	})

	_, err := ExecuteSynthesis(ctx, tdb.DB, mock, PRDSynthesisConfig(), "p-1", "mc-gpt", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "did not submit a document")

	// Verify run is failed.
	var count int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_runs WHERE project_id = 'p-1' AND status = 'failed'`).Scan(&count)
	assert.Equal(t, 1, count)
}

func TestExecuteSynthesis_ProviderError(t *testing.T) {
	tdb, ctx := setupSynthesisTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockGPT("")
	// Cancel context to simulate provider failure.
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err := ExecuteSynthesis(cancelCtx, tdb.DB, mock, PRDSynthesisConfig(), "p-1", "mc-gpt", logger)
	assert.Error(t, err)
}
