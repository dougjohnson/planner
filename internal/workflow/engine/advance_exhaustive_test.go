package engine

import (
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExhaustiveAutoAdvance_EveryStage tests the auto-advance decision for
// every stage in the workflow, verifying correct routing decisions.
func TestExhaustiveAutoAdvance_EveryStage(t *testing.T) {
	adv := setupAdvancer(t)

	// Expected auto-advance behavior per stage.
	tests := []struct {
		stageID       string
		shouldAdvance bool
		awaitingUser  bool
		hasTransition bool
	}{
		// Stage 1: foundations → prd_intake (auto if guard passes)
		{"foundations", true, false, true},
		// Stage 2: prd_intake → parallel_prd_generation (auto)
		{"prd_intake", true, false, true},
		// Stage 3: parallel_prd_generation → prd_synthesis (auto)
		{"parallel_prd_generation", true, false, true},
		// Stage 4: prd_synthesis → prd_integration (auto)
		{"prd_synthesis", true, false, true},
		// Stage 5: prd_integration → prd_disagreement_review (user-action) or prd_review
		{"prd_integration", false, true, true},
		// Stage 6: prd_disagreement_review → prd_review (auto after decisions)
		{"prd_disagreement_review", true, false, true},
		// Stage 7: prd_review → prd_commit (auto)
		{"prd_review", true, false, true},
		// Stage 8: prd_commit → prd_loop_control (user decides loop)
		{"prd_commit", false, true, true},
		// Stage 9: prd_loop_control → prd_review or parallel_plan_generation (user decision)
		{"prd_loop_control", true, false, true},
		// Stage 10: parallel_plan_generation → plan_synthesis (auto)
		{"parallel_plan_generation", true, false, true},
		// Stage 11: plan_synthesis → plan_integration (auto)
		{"plan_synthesis", true, false, true},
		// Stage 12: plan_integration → plan_disagreement_review (user) or plan_review
		{"plan_integration", false, true, true},
		// Stage 13: plan_disagreement_review → plan_review (auto)
		{"plan_disagreement_review", true, false, true},
		// Stage 14: plan_review → plan_commit (auto)
		{"plan_review", true, false, true},
		// Stage 15: plan_commit → plan_loop_control (user decides loop)
		{"plan_commit", false, true, true},
		// Stage 16: plan_loop_control → plan_review or final_export (user decision)
		{"plan_loop_control", true, false, true},
		// Stage 17: final_export — terminal, no transitions
		{"final_export", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.stageID, func(t *testing.T) {
			decision, err := adv.Evaluate(tt.stageID)
			require.NoError(t, err, "Evaluate(%s) should not error", tt.stageID)

			if !tt.hasTransition {
				assert.False(t, decision.ShouldAdvance, "%s: terminal stage should not advance", tt.stageID)
				return
			}

			if tt.awaitingUser {
				assert.True(t, decision.AwaitingUser, "%s: should be awaiting user", tt.stageID)
			}

			if tt.shouldAdvance {
				assert.True(t, decision.ShouldAdvance, "%s: should auto-advance", tt.stageID)
				assert.NotEmpty(t, decision.ToStageID, "%s: should have target stage", tt.stageID)
				assert.NotEmpty(t, decision.Guard, "%s: should have guard", tt.stageID)
			}
		})
	}
}

// TestIdempotentEvaluation ensures calling Evaluate multiple times for the
// same stage produces the same result.
func TestIdempotentEvaluation(t *testing.T) {
	adv := setupAdvancer(t)

	for _, stageID := range []string{"prd_intake", "prd_synthesis", "prd_loop_control", "final_export"} {
		d1, err1 := adv.Evaluate(stageID)
		d2, err2 := adv.Evaluate(stageID)

		require.NoError(t, err1)
		require.NoError(t, err2)

		assert.Equal(t, d1.ShouldAdvance, d2.ShouldAdvance, "%s: idempotency failed", stageID)
		assert.Equal(t, d1.ToStageID, d2.ToStageID, "%s: target mismatch", stageID)
		assert.Equal(t, d1.Guard, d2.Guard, "%s: guard mismatch", stageID)
	}
}

// TestAllStagesHaveAdvanceDecision ensures every stage in AllStages() can
// be evaluated without error.
func TestAllStagesHaveAdvanceDecision(t *testing.T) {
	adv := setupAdvancer(t)

	for _, stage := range workflow.AllStages() {
		decision, err := adv.Evaluate(stage.ID)
		require.NoError(t, err, "Evaluate(%s) errored", stage.ID)
		require.NotNil(t, decision, "Evaluate(%s) returned nil", stage.ID)

		// Every decision must have a reason.
		assert.NotEmpty(t, decision.Reason, "%s: missing reason", stage.ID)
		assert.Equal(t, stage.ID, decision.FromStageID, "%s: FromStageID mismatch", stage.ID)
	}
}

// TestUnknownStageReturnsNoTransitions verifies behavior for a non-existent stage.
func TestUnknownStageReturnsNoTransitions(t *testing.T) {
	adv := setupAdvancer(t)

	decision, err := adv.Evaluate("nonexistent_stage")
	require.NoError(t, err)
	assert.False(t, decision.ShouldAdvance)
	assert.Contains(t, decision.Reason, "no outgoing transitions")
}
