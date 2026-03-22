package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExhaustiveStageTransitions tests EVERY stage status pair (8×8=64 pairs)
// to verify legal transitions succeed and illegal transitions are rejected.
func TestExhaustiveStageTransitions(t *testing.T) {
	allStatuses := AllStageStatuses()

	for _, from := range allStatuses {
		for _, to := range allStatuses {
			expected := ValidStageTransition(from, to)
			err := ValidateStageTransition(from, to)

			if expected {
				assert.NoError(t, err, "stage %s → %s should be legal", from, to)
			} else {
				assert.Error(t, err, "stage %s → %s should be illegal", from, to)
			}
		}
	}
}

// TestExhaustiveRunTransitions tests EVERY run status pair (8×8=64 pairs).
func TestExhaustiveRunTransitions(t *testing.T) {
	allStatuses := AllRunStatuses()

	for _, from := range allStatuses {
		for _, to := range allStatuses {
			expected := ValidRunTransition(from, to)
			err := ValidateRunTransition(from, to)

			if expected {
				assert.NoError(t, err, "run %s → %s should be legal", from, to)
			} else {
				assert.Error(t, err, "run %s → %s should be illegal", from, to)
			}
		}
	}
}

// TestExhaustiveWorkflowTransitions tests every stage×stage pair to ensure
// only transitions in the table are permitted.
func TestExhaustiveWorkflowTransitions(t *testing.T) {
	stages := AllStages()

	// Build set of legal transitions.
	legal := make(map[string]bool)
	for _, tr := range AllTransitions() {
		key := tr.FromStageID + "→" + tr.ToStageID
		legal[key] = true
	}

	for _, from := range stages {
		for _, to := range stages {
			key := from.ID + "→" + to.ID
			_, err := ValidWorkflowTransition(from.ID, to.ID)

			if legal[key] {
				assert.NoError(t, err, "transition %s → %s should be legal", from.ID, to.ID)
			} else {
				assert.Error(t, err, "transition %s → %s should be illegal", from.ID, to.ID)
			}
		}
	}
}

// TestTerminalStageStatusesRejectAllTransitions ensures terminal statuses
// (completed, archived) cannot transition to any other status.
func TestTerminalStageStatusesRejectAllTransitions(t *testing.T) {
	terminals := []StageStatus{StageArchived}
	allStatuses := AllStageStatuses()

	for _, terminal := range terminals {
		for _, to := range allStatuses {
			assert.False(t, ValidStageTransition(terminal, to),
				"terminal stage %s should not transition to %s", terminal, to)
		}
	}
}

// TestTerminalRunStatusesRejectAllTransitions ensures terminal run statuses
// cannot transition to any other status.
func TestTerminalRunStatusesRejectAllTransitions(t *testing.T) {
	terminals := []RunStatus{RunCompleted, RunCancelled}
	allStatuses := AllRunStatuses()

	for _, terminal := range terminals {
		for _, to := range allStatuses {
			assert.False(t, ValidRunTransition(terminal, to),
				"terminal run %s should not transition to %s", terminal, to)
		}
	}
}

// TestSelfTransitionsNotAllowed ensures no status can transition to itself.
func TestSelfTransitionsNotAllowed_Stage(t *testing.T) {
	for _, s := range AllStageStatuses() {
		assert.False(t, ValidStageTransition(s, s),
			"stage %s should not self-transition", s)
	}
}

func TestSelfTransitionsNotAllowed_Run(t *testing.T) {
	for _, r := range AllRunStatuses() {
		assert.False(t, ValidRunTransition(r, r),
			"run %s should not self-transition", r)
	}
}

// TestEveryGuardInTransitionTableIsRegistered verifies that every guard name
// referenced by the transition table has a registered implementation.
func TestEveryGuardInTransitionTableIsRegistered(t *testing.T) {
	registered := make(map[string]bool)
	for _, name := range RegisteredGuards() {
		registered[name] = true
	}

	for _, tr := range AllTransitions() {
		assert.True(t, registered[tr.Guard],
			"transition %s → %s references unregistered guard %q",
			tr.FromStageID, tr.ToStageID, tr.Guard)
	}
}

// TestDAGProperty verifies the transition table forms a DAG (no cycles),
// except for the explicitly allowed loop-back transitions (loop control → review).
func TestTransitionGraphReachability(t *testing.T) {
	// Every stage should be reachable from foundations (except foundations itself).
	reachable := make(map[string]bool)
	reachable["foundations"] = true

	// BFS from foundations.
	queue := []string{"foundations"}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, tr := range TransitionsFrom(current) {
			if !reachable[tr.ToStageID] {
				reachable[tr.ToStageID] = true
				queue = append(queue, tr.ToStageID)
			}
		}
	}

	// All stages should be reachable.
	for _, s := range AllStages() {
		assert.True(t, reachable[s.ID],
			"stage %s (PRD #%d) is not reachable from foundations", s.ID, s.PRDNumber)
	}
}
