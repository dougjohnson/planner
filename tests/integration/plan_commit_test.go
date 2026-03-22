package integration

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPlanCommitTest(t *testing.T) (*testutil.TestDB, context.Context, string) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	now := "2026-01-01T00:00:00Z"

	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Plan Commit Test', '', 'active', 'plan_commit', '` + now + `', '` + now + `')`)

	// Plan fragments.
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('pf-1', 'p-1', 'plan', 'Epic 1: Core Infrastructure', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('pf-2', 'p-1', 'plan', 'Epic 2: Workflow Engine', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('pf-3', 'p-1', 'plan', 'Epic 3: Model Integration', 2, '` + now + `')`)

	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('pfv-1', 'pf-1', 'Core infra tasks...', 'ck1', '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('pfv-2', 'pf-2', 'Workflow engine tasks...', 'ck2', '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('pfv-3', 'pf-3', 'Model integration tasks...', 'ck3', '` + now + `')`)

	// Canonical plan artifact.
	tdb.Exec(`INSERT INTO artifacts (id, project_id, artifact_type, version_label, is_canonical, created_at)
		VALUES ('plan-canon', 'p-1', 'plan', 'plan.v01', 1, '` + now + `')`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('plan-canon', 'pfv-1', 0)`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('plan-canon', 'pfv-2', 1)`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('plan-canon', 'pfv-3', 2)`)

	return tdb, ctx, "plan-canon"
}

func TestStage15_PlanCommit_UpdateFragment(t *testing.T) {
	tdb, ctx, canonicalID := setupPlanCommitTest(t)

	ops := []workflow.FragmentOperation{
		{Type: "update", FragmentID: "pf-2", NewContent: "Revised workflow engine with state machines and guards.", Rationale: "More detailed task breakdown"},
	}

	result, err := workflow.CommitFragmentOperations(ctx, tdb.DB, "p-1", canonicalID, ops, "plan_commit", "run-1", "plan")
	require.NoError(t, err)

	assert.NotEmpty(t, result.ArtifactID)
	assert.Contains(t, result.VersionLabel, "plan.v02")
	assert.Equal(t, 1, result.UpdateCount)
	assert.Equal(t, 2, result.UnchangedCount)
	assert.False(t, result.NoChanges)

	// New artifact should have 3 fragments.
	var count int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifact_fragments WHERE artifact_id = ?`, result.ArtifactID).Scan(&count)
	assert.Equal(t, 3, count)

	// Lineage recorded.
	var relCount int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifact_relations WHERE artifact_id = ? AND relation_type = 'revised_from'`,
		result.ArtifactID).Scan(&relCount)
	assert.Equal(t, 1, relCount)
}

func TestStage15_PlanCommit_AddAndRemove(t *testing.T) {
	tdb, ctx, canonicalID := setupPlanCommitTest(t)

	ops := []workflow.FragmentOperation{
		{Type: "add", AfterFragmentID: "pf-1", Heading: "Epic 1.5: Testing", NewContent: "Test infrastructure tasks.", Rationale: "Missing testing epic"},
		{Type: "remove", FragmentID: "pf-3", Rationale: "Merged into Epic 2"},
	}

	result, err := workflow.CommitFragmentOperations(ctx, tdb.DB, "p-1", canonicalID, ops, "plan_commit", "run-2", "plan")
	require.NoError(t, err)

	assert.Equal(t, 1, result.AddCount)
	assert.Equal(t, 1, result.RemoveCount)

	// 3 original - 1 removed + 1 added = 3
	var count int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifact_fragments WHERE artifact_id = ?`, result.ArtifactID).Scan(&count)
	assert.Equal(t, 3, count)
}

func TestStage15_PlanCommit_ZeroOps_Convergence(t *testing.T) {
	tdb, ctx, canonicalID := setupPlanCommitTest(t)

	result, err := workflow.CommitFragmentOperations(ctx, tdb.DB, "p-1", canonicalID, nil, "plan_commit", "run-3", "plan")
	require.NoError(t, err)
	assert.True(t, result.NoChanges)

	conv := workflow.CheckConvergence(result, 3, 4)
	assert.Equal(t, workflow.ConvergenceDetected, conv.Status)
}

func TestStage15_PlanLoopEngine_ReusesConfig(t *testing.T) {
	cfg := workflow.PlanLoopConfig()
	assert.Equal(t, "plan", cfg.DocumentType)
	assert.Equal(t, "plan_review", cfg.ReviewStageID)
	assert.Equal(t, "plan_commit", cfg.CommitStageID)
	assert.Equal(t, "plan_loop_control", cfg.LoopControlStageID)

	ls := workflow.NewLoopState(cfg)
	action := ls.NextAction(&workflow.CommitResult{UpdateCount: 1})
	assert.Equal(t, workflow.LoopActionContinue, action)
}
