package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllTransitions_Count(t *testing.T) {
	transitions := AllTransitions()
	// 22 transitions: foundations→intake(1) + PRD pipeline(11) + Plan pipeline(10).
	assert.Equal(t, 22, len(transitions),
		"expected 22 legal transitions in the workflow")
}

func TestValidWorkflowTransition_LegalTransitions(t *testing.T) {
	tests := []struct {
		from, to, expectedGuard string
	}{
		{"foundations", "prd_intake", "foundationsApproved"},
		{"prd_intake", "parallel_prd_generation", "seedPrdSubmitted"},
		{"parallel_prd_generation", "prd_synthesis", "parallelQuorumSatisfied"},
		{"prd_synthesis", "prd_integration", "runCompleted"},
		{"prd_integration", "prd_disagreement_review", "hasDisagreements"},
		{"prd_integration", "prd_review", "noDisagreements"},
		{"prd_disagreement_review", "prd_review", "allDecisionsMade"},
		{"prd_review", "prd_commit", "fragmentOperationsRecorded"},
		{"prd_commit", "prd_loop_control", "runCompleted"},
		{"prd_loop_control", "prd_review", "loopNotExhausted"},
		{"prd_loop_control", "parallel_plan_generation", "loopExhausted"},
	}
	for _, tt := range tests {
		guard, err := ValidWorkflowTransition(tt.from, tt.to)
		require.NoError(t, err, "transition %s -> %s should be legal", tt.from, tt.to)
		assert.Equal(t, tt.expectedGuard, guard)
	}
}

func TestValidWorkflowTransition_IllegalTransitions(t *testing.T) {
	tests := []struct {
		from, to string
	}{
		{"foundations", "parallel_prd_generation"}, // skip Stage 2
		{"prd_intake", "prd_synthesis"},            // skip Stage 3
		{"prd_review", "final_export"},             // jump to end
		{"final_export", "foundations"},             // go backward
		{"prd_commit", "prd_review"},               // skip loop control
	}
	for _, tt := range tests {
		_, err := ValidWorkflowTransition(tt.from, tt.to)
		assert.Error(t, err, "transition %s -> %s should be illegal", tt.from, tt.to)
		assert.Contains(t, err.Error(), "illegal workflow transition")
	}
}

func TestSkipPath_Stage5To7(t *testing.T) {
	// Stage 5 (prd_integration) → Stage 7 (prd_review) when no disagreements.
	guard, err := ValidWorkflowTransition("prd_integration", "prd_review")
	require.NoError(t, err)
	assert.Equal(t, "noDisagreements", guard)
}

func TestSkipPath_Stage12To14(t *testing.T) {
	// Stage 12 (plan_integration) → Stage 14 (plan_review) when no disagreements.
	guard, err := ValidWorkflowTransition("plan_integration", "plan_review")
	require.NoError(t, err)
	assert.Equal(t, "noDisagreements", guard)
}

func TestLoopTransitions_PRD(t *testing.T) {
	// Stage 9 → Stage 7: loop continues.
	guard, err := ValidWorkflowTransition("prd_loop_control", "prd_review")
	require.NoError(t, err)
	assert.Equal(t, "loopNotExhausted", guard)

	// Stage 9 → Stage 10: loop exhausted.
	guard, err = ValidWorkflowTransition("prd_loop_control", "parallel_plan_generation")
	require.NoError(t, err)
	assert.Contains(t, []string{"loopExhausted", "loopConverged"}, guard)
}

func TestLoopTransitions_Plan(t *testing.T) {
	// Stage 16 → Stage 14: loop continues.
	guard, err := ValidWorkflowTransition("plan_loop_control", "plan_review")
	require.NoError(t, err)
	assert.Equal(t, "loopNotExhausted", guard)

	// Stage 16 → Stage 17: loop exits.
	guard, err = ValidWorkflowTransition("plan_loop_control", "final_export")
	require.NoError(t, err)
	assert.Contains(t, []string{"loopExhausted", "loopConverged"}, guard)
}

func TestTransitionsFrom(t *testing.T) {
	// Loop control has 3 outgoing transitions.
	transitions := TransitionsFrom("prd_loop_control")
	assert.Len(t, transitions, 3)

	// Final export has 0 outgoing transitions.
	transitions = TransitionsFrom("final_export")
	assert.Len(t, transitions, 0)

	// Integration has 2 outgoing (disagreements or no disagreements).
	transitions = TransitionsFrom("prd_integration")
	assert.Len(t, transitions, 2)
}

func TestTransitionsTo(t *testing.T) {
	// prd_review can be reached from 3 sources:
	// prd_integration (skip), prd_disagreement_review, prd_loop_control
	transitions := TransitionsTo("prd_review")
	assert.Len(t, transitions, 3)

	// foundations can only be reached as the start (no incoming).
	transitions = TransitionsTo("foundations")
	assert.Len(t, transitions, 0)
}

func TestPRDAndPlanSymmetry(t *testing.T) {
	// PRD path stages (5, 6, 7, 8, 9) should have symmetric plan stages (12, 13, 14, 15, 16).
	prdMappings := map[string]string{
		"prd_integration":          "plan_integration",
		"prd_disagreement_review":  "plan_disagreement_review",
		"prd_review":               "plan_review",
		"prd_commit":               "plan_commit",
		"prd_loop_control":         "plan_loop_control",
	}

	for prdStage, planStage := range prdMappings {
		prdTransitions := TransitionsFrom(prdStage)
		planTransitions := TransitionsFrom(planStage)
		assert.Equal(t, len(prdTransitions), len(planTransitions),
			"stage %s has %d transitions but %s has %d",
			prdStage, len(prdTransitions), planStage, len(planTransitions))
	}
}

func TestAllTransitionsReferenceValidStages(t *testing.T) {
	stages := AllStages()
	stageIDs := make(map[string]bool)
	for _, s := range stages {
		stageIDs[s.ID] = true
	}

	for _, tr := range AllTransitions() {
		assert.True(t, stageIDs[tr.FromStageID],
			"transition references unknown from-stage: %s", tr.FromStageID)
		assert.True(t, stageIDs[tr.ToStageID],
			"transition references unknown to-stage: %s", tr.ToStageID)
		assert.NotEmpty(t, tr.Guard,
			"transition %s -> %s has empty guard", tr.FromStageID, tr.ToStageID)
	}
}

func TestEveryNonTerminalStageHasOutgoingTransition(t *testing.T) {
	for _, s := range AllStages() {
		transitions := TransitionsFrom(s.ID)
		if s.ID == "final_export" {
			assert.Len(t, transitions, 0, "final_export should have no outgoing transitions")
		} else {
			assert.NotEmpty(t, transitions,
				"stage %s (PRD #%d) has no outgoing transitions", s.ID, s.PRDNumber)
		}
	}
}
