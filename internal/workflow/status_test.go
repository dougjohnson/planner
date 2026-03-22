package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllStageStatuses(t *testing.T) {
	statuses := AllStageStatuses()
	assert.Len(t, statuses, 8)
}

func TestAllRunStatuses(t *testing.T) {
	statuses := AllRunStatuses()
	assert.Len(t, statuses, 8)
}

func TestStageStatus_IsTerminal(t *testing.T) {
	assert.True(t, StageCompleted.IsTerminal())
	assert.True(t, StageArchived.IsTerminal())
	assert.False(t, StageRunning.IsTerminal())
	assert.False(t, StageNotStarted.IsTerminal())
}

func TestStageStatus_IsActive(t *testing.T) {
	assert.True(t, StageRunning.IsActive())
	assert.True(t, StageAwaitingUser.IsActive())
	assert.False(t, StageCompleted.IsActive())
	assert.False(t, StageNotStarted.IsActive())
}

func TestRunStatus_IsTerminal(t *testing.T) {
	assert.True(t, RunCompleted.IsTerminal())
	assert.True(t, RunFailed.IsTerminal())
	assert.True(t, RunCancelled.IsTerminal())
	assert.True(t, RunInterrupted.IsTerminal())
	assert.False(t, RunRunning.IsTerminal())
	assert.False(t, RunPending.IsTerminal())
}

func TestRunStatus_IsActive(t *testing.T) {
	assert.True(t, RunRunning.IsActive())
	assert.True(t, RunCancellationRequested.IsActive())
	assert.False(t, RunCompleted.IsActive())
	assert.False(t, RunPending.IsActive())
}

func TestStageTransitions_HappyPath(t *testing.T) {
	// Normal workflow progression.
	assert.True(t, ValidStageTransition(StageNotStarted, StageReady))
	assert.True(t, ValidStageTransition(StageReady, StageRunning))
	assert.True(t, ValidStageTransition(StageRunning, StageCompleted))
	assert.True(t, ValidStageTransition(StageRunning, StageAwaitingUser))
	assert.True(t, ValidStageTransition(StageAwaitingUser, StageCompleted))
}

func TestStageTransitions_FailureAndRecovery(t *testing.T) {
	assert.True(t, ValidStageTransition(StageRunning, StageRetryableFailure))
	assert.True(t, ValidStageTransition(StageRetryableFailure, StageRunning)) // retry
	assert.True(t, ValidStageTransition(StageRunning, StageBlocked))
	assert.True(t, ValidStageTransition(StageBlocked, StageReady)) // unblocked
}

func TestStageTransitions_Illegal(t *testing.T) {
	assert.False(t, ValidStageTransition(StageNotStarted, StageCompleted)) // can't skip
	assert.False(t, ValidStageTransition(StageCompleted, StageRunning))    // can't go back
	assert.False(t, ValidStageTransition(StageArchived, StageRunning))     // terminal
}

func TestRunTransitions_HappyPath(t *testing.T) {
	assert.True(t, ValidRunTransition(RunPending, RunRunning))
	assert.True(t, ValidRunTransition(RunRunning, RunCompleted))
	assert.True(t, ValidRunTransition(RunRunning, RunFailed))
	assert.True(t, ValidRunTransition(RunRunning, RunNeedsReview))
}

func TestRunTransitions_Cancellation(t *testing.T) {
	assert.True(t, ValidRunTransition(RunRunning, RunCancellationRequested))
	assert.True(t, ValidRunTransition(RunCancellationRequested, RunCancelled))
	// Cancellation may not take effect if run completes first.
	assert.True(t, ValidRunTransition(RunCancellationRequested, RunCompleted))
}

func TestRunTransitions_RetryAndResume(t *testing.T) {
	assert.True(t, ValidRunTransition(RunFailed, RunRunning))      // retry
	assert.True(t, ValidRunTransition(RunInterrupted, RunRunning)) // resume
	assert.True(t, ValidRunTransition(RunNeedsReview, RunRunning)) // re-run after review
}

func TestRunTransitions_Illegal(t *testing.T) {
	assert.False(t, ValidRunTransition(RunCompleted, RunRunning))   // terminal
	assert.False(t, ValidRunTransition(RunCancelled, RunRunning))   // terminal
	assert.False(t, ValidRunTransition(RunPending, RunCompleted))   // must run first
}

func TestValidateStageTransition_Error(t *testing.T) {
	err := ValidateStageTransition(StageNotStarted, StageCompleted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "illegal stage transition")
}

func TestValidateStageTransition_OK(t *testing.T) {
	err := ValidateStageTransition(StageNotStarted, StageReady)
	assert.NoError(t, err)
}

func TestValidateRunTransition_Error(t *testing.T) {
	err := ValidateRunTransition(RunCompleted, RunRunning)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "illegal run transition")
}

func TestValidateRunTransition_OK(t *testing.T) {
	err := ValidateRunTransition(RunPending, RunRunning)
	assert.NoError(t, err)
}

func TestEveryStageStatusHasTransitions(t *testing.T) {
	for _, s := range AllStageStatuses() {
		_, ok := stageTransitions[s]
		assert.True(t, ok, "stage status %q has no transition entry", s)
	}
}

func TestEveryRunStatusHasTransitions(t *testing.T) {
	for _, r := range AllRunStatuses() {
		_, ok := runTransitions[r]
		assert.True(t, ok, "run status %q has no transition entry", r)
	}
}
