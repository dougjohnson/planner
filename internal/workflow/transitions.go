package workflow

import "fmt"

// WorkflowTransition defines a legal stage-to-stage transition with its guard.
type WorkflowTransition struct {
	FromStageID string `json:"from_stage_id"`
	ToStageID   string `json:"to_stage_id"`
	Guard       string `json:"guard"`
}

// transitionTable is the complete, immutable set of legal stage transitions.
// No route, UI action, or backend job may bypass this table.
var transitionTable []WorkflowTransition

func init() {
	transitionTable = buildTransitionTable()
}

// AllTransitions returns the full legal transition table.
func AllTransitions() []WorkflowTransition {
	result := make([]WorkflowTransition, len(transitionTable))
	copy(result, transitionTable)
	return result
}

// ValidWorkflowTransition checks if a stage-to-stage transition is legal.
// Returns the guard name if valid, or an error if the transition is not in the table.
func ValidWorkflowTransition(fromStageID, toStageID string) (string, error) {
	for _, t := range transitionTable {
		if t.FromStageID == fromStageID && t.ToStageID == toStageID {
			return t.Guard, nil
		}
	}
	return "", fmt.Errorf("illegal workflow transition: %s -> %s", fromStageID, toStageID)
}

// TransitionsFrom returns all legal transitions from a given stage.
func TransitionsFrom(stageID string) []WorkflowTransition {
	var result []WorkflowTransition
	for _, t := range transitionTable {
		if t.FromStageID == stageID {
			result = append(result, t)
		}
	}
	return result
}

// TransitionsTo returns all legal transitions targeting a given stage.
func TransitionsTo(stageID string) []WorkflowTransition {
	var result []WorkflowTransition
	for _, t := range transitionTable {
		if t.ToStageID == stageID {
			result = append(result, t)
		}
	}
	return result
}

func buildTransitionTable() []WorkflowTransition {
	return []WorkflowTransition{
		// --- Stage 1: Foundations ---
		{FromStageID: "foundations", ToStageID: "prd_intake", Guard: "foundationsApproved"},

		// --- Stage 2: PRD Intake ---
		{FromStageID: "prd_intake", ToStageID: "parallel_prd_generation", Guard: "seedPrdSubmitted"},

		// --- Stage 3: Parallel PRD Generation ---
		{FromStageID: "parallel_prd_generation", ToStageID: "prd_synthesis", Guard: "parallelQuorumSatisfied"},

		// --- Stage 4: PRD Synthesis ---
		{FromStageID: "prd_synthesis", ToStageID: "prd_integration", Guard: "runCompleted"},

		// --- Stage 5: PRD Integration ---
		// Normal path: disagreements exist → user review.
		{FromStageID: "prd_integration", ToStageID: "prd_disagreement_review", Guard: "hasDisagreements"},
		// Skip path: no disagreements → skip review, go to PRD review loop.
		{FromStageID: "prd_integration", ToStageID: "prd_review", Guard: "noDisagreements"},

		// --- Stage 6: User Review of PRD Disagreements ---
		{FromStageID: "prd_disagreement_review", ToStageID: "prd_review", Guard: "allDecisionsMade"},

		// --- Stage 7: PRD Review Pass ---
		{FromStageID: "prd_review", ToStageID: "prd_commit", Guard: "fragmentOperationsRecorded"},

		// --- Stage 8: Commit PRD Fragment Revisions ---
		{FromStageID: "prd_commit", ToStageID: "prd_loop_control", Guard: "runCompleted"},

		// --- Stage 9: PRD Loop Control ---
		// Loop continues: iterations remaining and no convergence accepted.
		{FromStageID: "prd_loop_control", ToStageID: "prd_review", Guard: "loopNotExhausted"},
		// Loop exits: iterations exhausted.
		{FromStageID: "prd_loop_control", ToStageID: "parallel_plan_generation", Guard: "loopExhausted"},
		// Loop exits: convergence accepted by user.
		{FromStageID: "prd_loop_control", ToStageID: "parallel_plan_generation", Guard: "loopConverged"},

		// --- Stage 10: Parallel Plan Generation ---
		{FromStageID: "parallel_plan_generation", ToStageID: "plan_synthesis", Guard: "parallelQuorumSatisfied"},

		// --- Stage 11: Plan Synthesis ---
		{FromStageID: "plan_synthesis", ToStageID: "plan_integration", Guard: "runCompleted"},

		// --- Stage 12: Plan Integration ---
		// Normal path: disagreements exist → user review.
		{FromStageID: "plan_integration", ToStageID: "plan_disagreement_review", Guard: "hasDisagreements"},
		// Skip path: no disagreements → skip review, go to plan review loop.
		{FromStageID: "plan_integration", ToStageID: "plan_review", Guard: "noDisagreements"},

		// --- Stage 13: User Review of Plan Disagreements ---
		{FromStageID: "plan_disagreement_review", ToStageID: "plan_review", Guard: "allDecisionsMade"},

		// --- Stage 14: Plan Review Pass ---
		{FromStageID: "plan_review", ToStageID: "plan_commit", Guard: "fragmentOperationsRecorded"},

		// --- Stage 15: Commit Plan Fragment Revisions ---
		{FromStageID: "plan_commit", ToStageID: "plan_loop_control", Guard: "runCompleted"},

		// --- Stage 16: Plan Loop Control ---
		// Loop continues.
		{FromStageID: "plan_loop_control", ToStageID: "plan_review", Guard: "loopNotExhausted"},
		// Loop exits: iterations exhausted.
		{FromStageID: "plan_loop_control", ToStageID: "final_export", Guard: "loopExhausted"},
		// Loop exits: convergence accepted.
		{FromStageID: "plan_loop_control", ToStageID: "final_export", Guard: "loopConverged"},

		// --- Stage 17: Final Export ---
		// Terminal stage — no outgoing transitions.
	}
}
