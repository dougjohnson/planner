package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGuardTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('proj-1', 'Test', '', 'active', 'foundations', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-gpt', 'openai', 'gpt-4o', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-opus', 'anthropic', 'claude-opus', datetime('now'), datetime('now'))`)
	return tdb, context.Background()
}

func TestRegisteredGuards_AllPresent(t *testing.T) {
	names := RegisteredGuards()
	assert.GreaterOrEqual(t, len(names), 11, "should have at least 11 guards")

	expected := []string{
		"foundationsApproved", "seedPrdSubmitted", "parallelQuorumSatisfied",
		"runCompleted", "hasDisagreements", "noDisagreements",
		"allDecisionsMade", "fragmentOperationsRecorded",
		"loopNotExhausted", "loopExhausted", "loopConverged",
	}
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, e := range expected {
		assert.True(t, nameSet[e], "missing guard: %s", e)
	}
}

func TestEvaluateGuard_UnknownGuard(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	_, err := EvaluateGuard(ctx, tdb.DB, "nonexistent", "proj-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown guard")
}

func TestGuard_FoundationsApproved_NoInputs(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	result, err := EvaluateGuard(ctx, tdb.DB, "foundationsApproved", "proj-1")
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestGuard_FoundationsApproved_WithInputs(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'proj-1', 'foundation', 'upload', '/path', datetime('now'), datetime('now'))`)

	result, err := EvaluateGuard(ctx, tdb.DB, "foundationsApproved", "proj-1")
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestGuard_SeedPrdSubmitted_Empty(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	result, err := EvaluateGuard(ctx, tdb.DB, "seedPrdSubmitted", "proj-1")
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestGuard_SeedPrdSubmitted_Present(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-2', 'proj-1', 'seed_prd', 'paste', '/prd.md', datetime('now'), datetime('now'))`)

	result, err := EvaluateGuard(ctx, tdb.DB, "seedPrdSubmitted", "proj-1")
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestGuard_ParallelQuorumSatisfied_NoRuns(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	result, err := EvaluateGuard(ctx, tdb.DB, "parallelQuorumSatisfied", "proj-1")
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestGuard_ParallelQuorumSatisfied_BothFamilies(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-1', 'proj-1', 'parallel_prd_generation', 'mc-gpt', 'completed', datetime('now'))`)
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-2', 'proj-1', 'parallel_prd_generation', 'mc-opus', 'completed', datetime('now'))`)

	result, err := EvaluateGuard(ctx, tdb.DB, "parallelQuorumSatisfied", "proj-1")
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestGuard_ParallelQuorumSatisfied_OnlyGPT(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-1', 'proj-1', 'parallel_prd_generation', 'mc-gpt', 'completed', datetime('now'))`)

	result, err := EvaluateGuard(ctx, tdb.DB, "parallelQuorumSatisfied", "proj-1")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "Opus")
}

func TestGuard_HasDisagreements_None(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	result, err := EvaluateGuard(ctx, tdb.DB, "hasDisagreements", "proj-1")
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestGuard_NoDisagreements_None(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	result, err := EvaluateGuard(ctx, tdb.DB, "noDisagreements", "proj-1")
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestGuard_LoopExhausted_Default4(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	// With 0 completed reviews, loop is not exhausted (default max=4).
	result, err := EvaluateGuard(ctx, tdb.DB, "loopExhausted", "proj-1")
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestGuard_LoopNotExhausted_WithRemaining(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	result, err := EvaluateGuard(ctx, tdb.DB, "loopNotExhausted", "proj-1")
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestGuard_LoopConverged_NotAccepted(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	result, err := EvaluateGuard(ctx, tdb.DB, "loopConverged", "proj-1")
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestGuard_LoopConverged_Accepted(t *testing.T) {
	tdb, ctx := setupGuardTest(t)
	tdb.Exec(`INSERT INTO workflow_events (id, project_id, event_type, created_at)
		VALUES ('e-1', 'proj-1', 'convergence_accepted', datetime('now'))`)

	result, err := EvaluateGuard(ctx, tdb.DB, "loopConverged", "proj-1")
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestAllTransitionGuardsAreRegistered(t *testing.T) {
	for _, tr := range AllTransitions() {
		_, ok := guardRegistry[tr.Guard]
		assert.True(t, ok,
			"transition %s -> %s references unregistered guard %q",
			tr.FromStageID, tr.ToStageID, tr.Guard)
	}
}
