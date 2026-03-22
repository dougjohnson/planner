package workflow

import "fmt"

// StageStatus represents the current state of a workflow stage.
type StageStatus string

const (
	StageNotStarted      StageStatus = "not_started"
	StageReady           StageStatus = "ready"
	StageRunning         StageStatus = "running"
	StageAwaitingUser    StageStatus = "awaiting_user"
	StageRetryableFailure StageStatus = "retryable_failure"
	StageBlocked         StageStatus = "blocked"
	StageCompleted       StageStatus = "completed"
	StageArchived        StageStatus = "archived"
)

// AllStageStatuses returns all valid stage statuses.
func AllStageStatuses() []StageStatus {
	return []StageStatus{
		StageNotStarted, StageReady, StageRunning, StageAwaitingUser,
		StageRetryableFailure, StageBlocked, StageCompleted, StageArchived,
	}
}

// IsTerminal returns true if the stage is in a final state.
func (s StageStatus) IsTerminal() bool {
	return s == StageCompleted || s == StageArchived
}

// IsActive returns true if the stage is currently executing or awaiting input.
func (s StageStatus) IsActive() bool {
	return s == StageRunning || s == StageAwaitingUser
}

// stageTransitions defines legal stage status transitions.
// A transition from status A to status B is legal if stageTransitions[A]
// contains B.
var stageTransitions = map[StageStatus][]StageStatus{
	StageNotStarted:       {StageReady, StageBlocked},
	StageReady:            {StageRunning, StageBlocked},
	StageRunning:          {StageCompleted, StageAwaitingUser, StageRetryableFailure, StageBlocked},
	StageAwaitingUser:     {StageRunning, StageCompleted, StageBlocked},
	StageRetryableFailure: {StageRunning, StageBlocked, StageArchived},
	StageBlocked:          {StageReady, StageNotStarted},
	StageCompleted:        {StageArchived},
	StageArchived:         {},
}

// ValidStageTransition checks whether transitioning from one stage status
// to another is legal.
func ValidStageTransition(from, to StageStatus) bool {
	targets, ok := stageTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// ValidateStageTransition returns an error if the transition is not legal.
func ValidateStageTransition(from, to StageStatus) error {
	if !ValidStageTransition(from, to) {
		return fmt.Errorf("illegal stage transition: %s -> %s", from, to)
	}
	return nil
}

// RunStatus represents the current state of a workflow run (a single model invocation).
type RunStatus string

const (
	RunPending                RunStatus = "pending"
	RunRunning                RunStatus = "running"
	RunCompleted              RunStatus = "completed"
	RunFailed                 RunStatus = "failed"
	RunNeedsReview            RunStatus = "needs_review"
	RunInterrupted            RunStatus = "interrupted"
	RunCancelled              RunStatus = "cancelled"
	RunCancellationRequested  RunStatus = "cancellation_requested"
)

// AllRunStatuses returns all valid run statuses.
func AllRunStatuses() []RunStatus {
	return []RunStatus{
		RunPending, RunRunning, RunCompleted, RunFailed,
		RunNeedsReview, RunInterrupted, RunCancelled, RunCancellationRequested,
	}
}

// IsTerminal returns true if the run is in a final state.
func (r RunStatus) IsTerminal() bool {
	return r == RunCompleted || r == RunFailed || r == RunCancelled || r == RunInterrupted
}

// IsActive returns true if the run is currently executing.
func (r RunStatus) IsActive() bool {
	return r == RunRunning || r == RunCancellationRequested
}

// runTransitions defines legal run status transitions.
var runTransitions = map[RunStatus][]RunStatus{
	RunPending:               {RunRunning, RunCancelled},
	RunRunning:               {RunCompleted, RunFailed, RunNeedsReview, RunInterrupted, RunCancellationRequested},
	RunCancellationRequested: {RunCancelled, RunCompleted, RunFailed},
	RunNeedsReview:           {RunCompleted, RunFailed, RunRunning},
	RunFailed:                {RunRunning}, // retry
	RunCompleted:             {},
	RunCancelled:             {},
	RunInterrupted:           {RunRunning}, // resume after process restart
}

// ValidRunTransition checks whether transitioning from one run status
// to another is legal.
func ValidRunTransition(from, to RunStatus) bool {
	targets, ok := runTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// ValidateRunTransition returns an error if the transition is not legal.
func ValidateRunTransition(from, to RunStatus) error {
	if !ValidRunTransition(from, to) {
		return fmt.Errorf("illegal run transition: %s -> %s", from, to)
	}
	return nil
}
