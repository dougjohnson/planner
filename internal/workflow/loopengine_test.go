package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPRDLoopConfig(t *testing.T) {
	cfg := PRDLoopConfig()
	assert.Equal(t, "prd", cfg.DocumentType)
	assert.Equal(t, "prd_review", cfg.ReviewStageID)
	assert.Equal(t, "prd_commit", cfg.CommitStageID)
	assert.Equal(t, 4, cfg.MaxIterations)
	assert.Len(t, cfg.ReviewPromptTemplates, 2)
}

func TestPlanLoopConfig(t *testing.T) {
	cfg := PlanLoopConfig()
	assert.Equal(t, "plan", cfg.DocumentType)
	assert.Equal(t, "plan_review", cfg.ReviewStageID)
	assert.Equal(t, "plan_commit", cfg.CommitStageID)
}

func TestLoopState_NextAction_Continue(t *testing.T) {
	ls := NewLoopState(PRDLoopConfig())
	action := ls.NextAction(&CommitResult{UpdateCount: 2})
	assert.Equal(t, LoopActionContinue, action)
	assert.Equal(t, 1, ls.CurrentIteration)
}

func TestLoopState_NextAction_Converged(t *testing.T) {
	ls := NewLoopState(PRDLoopConfig())
	ls.CurrentIteration = 1 // simulate having done 1 iteration

	action := ls.NextAction(&CommitResult{NoChanges: true})
	assert.Equal(t, LoopActionConverged, action)
	assert.Equal(t, ConvergenceDetected, ls.Convergence)
}

func TestLoopState_NextAction_Exhausted(t *testing.T) {
	cfg := PRDLoopConfig()
	cfg.MaxIterations = 3
	ls := NewLoopState(cfg)
	ls.CurrentIteration = 2 // already done 2

	action := ls.NextAction(&CommitResult{UpdateCount: 1})
	assert.Equal(t, LoopActionExhausted, action)
}

func TestLoopState_NextAction_PauseForUser(t *testing.T) {
	cfg := PRDLoopConfig()
	cfg.PauseBetweenLoops = true
	ls := NewLoopState(cfg)

	action := ls.NextAction(&CommitResult{UpdateCount: 1})
	assert.Equal(t, LoopActionPauseForUser, action)
}

func TestLoopState_ModelFamilyForIteration_Default(t *testing.T) {
	ls := NewLoopState(PRDLoopConfig()) // max 4

	assert.Equal(t, "gpt", ls.ModelFamilyForIteration(1))
	assert.Equal(t, "opus", ls.ModelFamilyForIteration(2)) // midpoint
	assert.Equal(t, "gpt", ls.ModelFamilyForIteration(3))
	assert.Equal(t, "gpt", ls.ModelFamilyForIteration(4))
}

func TestLoopState_ModelFamilyForIteration_SmallLoop(t *testing.T) {
	cfg := PRDLoopConfig()
	cfg.MaxIterations = 2
	ls := NewLoopState(cfg)

	assert.Equal(t, "opus", ls.ModelFamilyForIteration(1)) // midpoint = 1
	assert.Equal(t, "gpt", ls.ModelFamilyForIteration(2))
}

func TestLoopState_PromptTemplateForFamily(t *testing.T) {
	ls := NewLoopState(PRDLoopConfig())
	assert.Equal(t, "GPT_PRD_REVIEW_V1", ls.PromptTemplateForFamily("gpt"))
	assert.Equal(t, "OPUS_PRD_REVIEW_V1", ls.PromptTemplateForFamily("opus"))
}

func TestLoopState_RecordIteration(t *testing.T) {
	ls := NewLoopState(PRDLoopConfig())
	assert.Empty(t, ls.Iterations)

	ls.RecordIteration(LoopIterationRecord{Number: 1, ModelFamily: "gpt", OperationCount: 3})
	assert.Len(t, ls.Iterations, 1)
	assert.Equal(t, 3, ls.Iterations[0].OperationCount)
}

func TestLoadLoopConfig_Defaults(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)

	cfg := LoadLoopConfig(ctx, tdb.DB, "p-1", "prd")
	assert.Equal(t, 4, cfg.MaxIterations)
	assert.False(t, cfg.PauseBetweenLoops)
}

func TestLoadLoopConfig_FromDB(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO loop_configs (id, project_id, loop_type, iteration_count, pause_between_loops, created_at, updated_at)
		VALUES ('lc-1', 'p-1', 'prd', 6, 1, datetime('now'), datetime('now'))`)

	cfg := LoadLoopConfig(ctx, tdb.DB, "p-1", "prd")
	assert.Equal(t, 6, cfg.MaxIterations)
	assert.True(t, cfg.PauseBetweenLoops)
}

func TestLoopEngine_FullCycleWithConvergence(t *testing.T) {
	cfg := PRDLoopConfig()
	cfg.MaxIterations = 4
	ls := NewLoopState(cfg)

	// Iteration 1: changes made.
	action := ls.NextAction(&CommitResult{UpdateCount: 3})
	assert.Equal(t, LoopActionContinue, action)

	// Iteration 2: changes made.
	action = ls.NextAction(&CommitResult{UpdateCount: 1, AddCount: 1})
	assert.Equal(t, LoopActionContinue, action)

	// Iteration 3: no changes — convergence.
	action = ls.NextAction(&CommitResult{NoChanges: true})
	assert.Equal(t, LoopActionConverged, action)
	assert.Equal(t, 3, ls.CurrentIteration)
}

func TestLoopEngine_FullCycleNoConvergence(t *testing.T) {
	cfg := PRDLoopConfig()
	cfg.MaxIterations = 3
	ls := NewLoopState(cfg)

	ls.NextAction(&CommitResult{UpdateCount: 2})
	ls.NextAction(&CommitResult{UpdateCount: 1})
	action := ls.NextAction(&CommitResult{UpdateCount: 1})
	assert.Equal(t, LoopActionExhausted, action)
}

func TestLoopEngine_PlanReuse(t *testing.T) {
	// Verify the same engine works for plan loops.
	ls := NewLoopState(PlanLoopConfig())
	assert.Equal(t, "plan_review", ls.Config.ReviewStageID)
	assert.Equal(t, "plan_commit", ls.Config.CommitStageID)

	action := ls.NextAction(&CommitResult{NoChanges: true})
	assert.Equal(t, LoopActionConverged, action)
}
