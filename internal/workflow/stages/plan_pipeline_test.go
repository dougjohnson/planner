package stages

import (
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlanStageDefinitions verifies all plan pipeline stages (10-16) have
// correct configuration in the stage definition catalog.
func TestPlanStageDefinitions(t *testing.T) {
	allStages := workflow.AllStages()
	planStages := make(map[int]workflow.StageDefinition)
	for _, s := range allStages {
		if s.PRDNumber >= 10 && s.PRDNumber <= 16 {
			planStages[s.PRDNumber] = s
		}
	}

	// Stage 10: Parallel Plan Generation.
	s10, ok := planStages[10]
	require.True(t, ok, "Stage 10 must exist")
	assert.True(t, s10.IsParallel, "Stage 10 should be parallel")
	assert.True(t, s10.RequiresModels, "Stage 10 requires model calls")

	// Stage 11: Plan Synthesis.
	s11, ok := planStages[11]
	require.True(t, ok, "Stage 11 must exist")
	assert.False(t, s11.IsParallel, "Stage 11 is not parallel")
	assert.True(t, s11.RequiresModels, "Stage 11 requires model calls")

	// Stage 14: Plan Review.
	s14, ok := planStages[14]
	require.True(t, ok, "Stage 14 must exist")
	assert.True(t, s14.RequiresModels, "Stage 14 requires model calls")

	// Stage 16: Plan Loop Control.
	s16, ok := planStages[16]
	require.True(t, ok, "Stage 16 must exist")
	assert.True(t, s16.IsLoopControl, "Stage 16 is a loop controller")
	assert.False(t, s16.RequiresModels, "Stage 16 doesn't call models")
}

// TestPlanPipelineToolAssignment verifies correct tools are assigned per stage.
func TestPlanPipelineToolAssignment(t *testing.T) {
	tests := []struct {
		stageID    string
		toolCount  int
		mustHave   []string
	}{
		{"parallel_plan_generation", 1, []string{"submit_document"}},
		{"plan_synthesis", 2, []string{"submit_document", "submit_change_rationale"}},
		{"plan_integration", 3, []string{"submit_document", "report_agreement", "report_disagreement"}},
		{"plan_review", 4, []string{"update_fragment", "add_fragment", "remove_fragment", "submit_review_summary"}},
	}

	for _, tt := range tests {
		t.Run(tt.stageID, func(t *testing.T) {
			tools := models.ToolsForStage(tt.stageID)
			assert.Len(t, tools, tt.toolCount, "wrong tool count for %s", tt.stageID)

			names := make(map[string]bool)
			for _, tool := range tools {
				names[tool.Name] = true
			}
			for _, expected := range tt.mustHave {
				assert.True(t, names[expected], "%s missing tool %q", tt.stageID, expected)
			}
		})
	}
}

// TestPlanPipelineTransitionChain verifies the plan pipeline transitions
// form the expected chain: 10→11→12→(13|14)→14→15→16→(14|17).
func TestPlanPipelineTransitionChain(t *testing.T) {
	chain := []struct {
		from, to, guard string
	}{
		{"parallel_plan_generation", "plan_synthesis", "parallelQuorumSatisfied"},
		{"plan_synthesis", "plan_integration", "runCompleted"},
		{"plan_integration", "plan_disagreement_review", "hasDisagreements"},
		{"plan_integration", "plan_review", "noDisagreements"},
		{"plan_disagreement_review", "plan_review", "allDecisionsMade"},
		{"plan_review", "plan_commit", "fragmentOperationsRecorded"},
		{"plan_commit", "plan_loop_control", "runCompleted"},
		{"plan_loop_control", "plan_review", "loopNotExhausted"},
		// Two transitions to final_export with different guards — just check one is valid.
		{"plan_loop_control", "final_export", "loopExhausted"},
	}

	for _, tt := range chain {
		guard, err := workflow.ValidWorkflowTransition(tt.from, tt.to)
		assert.NoError(t, err, "%s → %s should be legal", tt.from, tt.to)
		assert.Equal(t, tt.guard, guard, "%s → %s guard mismatch", tt.from, tt.to)
	}
}

// TestPlanAndPRDPipelineSymmetry verifies the plan pipeline mirrors the PRD pipeline.
func TestPlanAndPRDPipelineSymmetry(t *testing.T) {
	prdToPlain := map[string]string{
		"parallel_prd_generation":  "parallel_plan_generation",
		"prd_synthesis":            "plan_synthesis",
		"prd_integration":          "plan_integration",
		"prd_disagreement_review":  "plan_disagreement_review",
		"prd_review":               "plan_review",
		"prd_commit":               "plan_commit",
		"prd_loop_control":         "plan_loop_control",
	}

	for prdStage, planStage := range prdToPlain {
		prdTransitions := workflow.TransitionsFrom(prdStage)
		planTransitions := workflow.TransitionsFrom(planStage)
		assert.Equal(t, len(prdTransitions), len(planTransitions),
			"PRD stage %s (%d transitions) vs Plan stage %s (%d transitions)",
			prdStage, len(prdTransitions), planStage, len(planTransitions))
	}
}

// TestPlanReviewUsesFragmentOps verifies plan review stages use fragment
// operation tools, not submit_document.
func TestPlanReviewUsesFragmentOps(t *testing.T) {
	tools := models.ToolsForStage("plan_review")
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	assert.True(t, names["update_fragment"], "plan_review must have update_fragment")
	assert.True(t, names["add_fragment"], "plan_review must have add_fragment")
	assert.True(t, names["remove_fragment"], "plan_review must have remove_fragment")
	assert.False(t, names["submit_document"], "plan_review should NOT have submit_document")
}

// TestPlanGenerationUsesSubmitOnly verifies plan generation stages only have
// submit_document (not fragment ops).
func TestPlanGenerationUsesSubmitOnly(t *testing.T) {
	tools := models.ToolsForStage("parallel_plan_generation")
	require.Len(t, tools, 1)
	assert.Equal(t, "submit_document", tools[0].Name)
	assert.True(t, tools[0].Required, "submit_document must be required")
}
