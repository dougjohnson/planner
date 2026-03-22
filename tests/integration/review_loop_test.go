package integration

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/providers"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/dougflynn/flywheel-planner/internal/workflow/stages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLoopTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Loop Test', '', 'active', 'prd_review', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-gpt', 'openai', 'gpt-4o', 1, datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-opus', 'anthropic', 'claude-opus', 1, datetime('now'), datetime('now'))`)

	// Setup canonical artifact with fragments.
	now := "2026-01-01T00:00:00Z"
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('f-1', 'p-1', 'prd', 'Introduction', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('f-2', 'p-1', 'prd', 'Architecture', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('fv-1', 'f-1', 'Intro content.', 'ck1', '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('fv-2', 'f-2', 'Arch content.', 'ck2', '` + now + `')`)
	tdb.Exec(`INSERT INTO artifacts (id, project_id, artifact_type, version_label, is_canonical, created_at)
		VALUES ('art-canon', 'p-1', 'prd', 'prd.v01', 1, '` + now + `')`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
		VALUES ('art-canon', 'fv-1', 0)`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
		VALUES ('art-canon', 'fv-2', 1)`)

	return tdb, ctx
}

func TestReviewLoop_ReviewProducesOperations(t *testing.T) {
	tdb, ctx := setupLoopTest(t)
	logger := testutil.NewTestLogger(t)

	mock := providers.NewMockGPT("")
	mock.SetOverride(mock.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "review-1",
		ToolCalls: []models.ToolCall{
			{ID: "tc-1", Name: "update_fragment", Arguments: map[string]any{
				"fragment_id": "f-1", "new_content": "Better intro.", "rationale": "Clarity",
			}},
			{ID: "tc-2", Name: "submit_review_summary", Arguments: map[string]any{
				"summary": "Improved introduction",
			}},
		},
		Usage: models.UsageMetadata{TotalTokens: 3000},
	})

	result, err := stages.ExecuteReview(ctx, tdb.DB, mock, stages.PRDReviewConfig("gpt"), "p-1", "mc-gpt", logger)
	require.NoError(t, err)
	assert.Equal(t, 1, result.OperationCount)
	assert.False(t, result.NoChanges)
}

func TestReviewLoop_CommitAppliesOperations(t *testing.T) {
	tdb, ctx := setupLoopTest(t)

	ops := []workflow.FragmentOperation{
		{Type: "update", FragmentID: "f-1", NewContent: "Updated intro.", Rationale: "Better"},
	}

	commitResult, err := workflow.CommitFragmentOperations(ctx, tdb.DB, "p-1", "art-canon", ops, "prd_commit", "run-1", "prd")
	require.NoError(t, err)
	assert.Equal(t, 1, commitResult.UpdateCount)
	assert.Equal(t, 1, commitResult.UnchangedCount) // f-2 unchanged
	assert.NotEmpty(t, commitResult.ArtifactID)
}

func TestReviewLoop_ConvergenceDetectedOnZeroOps(t *testing.T) {
	commitResult := &workflow.CommitResult{NoChanges: true}
	conv := workflow.CheckConvergence(commitResult, 3, 4)
	assert.Equal(t, workflow.ConvergenceDetected, conv.Status)
	assert.Equal(t, 1, conv.RemainingLoops)
}

func TestReviewLoop_LoopEngineFullCycle(t *testing.T) {
	cfg := workflow.PRDLoopConfig()
	cfg.MaxIterations = 3
	ls := workflow.NewLoopState(cfg)

	// Iteration 1: changes.
	action := ls.NextAction(&workflow.CommitResult{UpdateCount: 2})
	assert.Equal(t, workflow.LoopActionContinue, action)

	// Iteration 2: changes.
	action = ls.NextAction(&workflow.CommitResult{UpdateCount: 1})
	assert.Equal(t, workflow.LoopActionContinue, action)

	// Iteration 3: exhausted.
	action = ls.NextAction(&workflow.CommitResult{UpdateCount: 1})
	assert.Equal(t, workflow.LoopActionExhausted, action)
}

func TestReviewLoop_ModelRotationAtMidpoint(t *testing.T) {
	ls := workflow.NewLoopState(workflow.PRDLoopConfig())

	assert.Equal(t, "gpt", ls.ModelFamilyForIteration(1))
	assert.Equal(t, "opus", ls.ModelFamilyForIteration(2)) // midpoint of 4
	assert.Equal(t, "gpt", ls.ModelFamilyForIteration(3))
	assert.Equal(t, "gpt", ls.ModelFamilyForIteration(4))
}

func TestReviewLoop_ConvergenceAcceptSkipsRemaining(t *testing.T) {
	tdb, ctx := setupLoopTest(t)

	cfg := workflow.PRDLoopConfig()
	cfg.MaxIterations = 4
	ls := workflow.NewLoopState(cfg)

	// Run 2 iterations with changes.
	ls.NextAction(&workflow.CommitResult{UpdateCount: 2})
	ls.NextAction(&workflow.CommitResult{UpdateCount: 1})

	// Iteration 3: convergence.
	action := ls.NextAction(&workflow.CommitResult{NoChanges: true})
	assert.Equal(t, workflow.LoopActionConverged, action)

	// User accepts convergence.
	conv := workflow.CheckConvergence(&workflow.CommitResult{NoChanges: true}, 3, 4)
	err := workflow.AcceptConvergence(ctx, tdb.DB, "p-1", conv)
	require.NoError(t, err)

	// Verify guard now passes.
	guardResult, err := workflow.EvaluateGuard(ctx, tdb.DB, "loopConverged", "p-1")
	require.NoError(t, err)
	assert.True(t, guardResult.Passed)
}

func TestReviewLoop_ChangeHistoryBuilds(t *testing.T) {
	tdb, ctx := setupLoopTest(t)

	// Add completed review runs.
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-1', 'p-1', 'prd_review', 'mc-gpt', 'completed', '2026-01-01T01:00:00Z')`)
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-2', 'p-1', 'prd_review', 'mc-opus', 'completed', '2026-01-01T02:00:00Z')`)

	history, err := workflow.BuildChangeHistory(ctx, tdb.DB, "p-1", "prd")
	require.NoError(t, err)
	assert.Equal(t, 2, history.TotalIters)
	assert.Equal(t, "gpt", history.Entries[0].ModelFamily)
	assert.Equal(t, "opus", history.Entries[1].ModelFamily)

	md := workflow.RenderChangeHistoryMarkdown(history)
	assert.Contains(t, md, "Prior Review History")
	assert.Contains(t, md, "Iteration 1 (gpt)")
}
