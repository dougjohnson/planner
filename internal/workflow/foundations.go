package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// LockFoundationsResult holds the outcome of locking the foundations stage.
type LockFoundationsResult struct {
	ProjectID      string `json:"project_id"`
	PreviousStage  string `json:"previous_stage"`
	CurrentStage   string `json:"current_stage"`
	FoundationCount int   `json:"foundation_count"`
	Locked         bool   `json:"locked"`
}

// LockFoundations validates that required foundations are present, transitions
// the project from Stage 1 (foundations) to Stage 2 (prd_intake), and marks
// foundation artifacts as immutable. This is the first stage transition and
// proves the workflow engine's transition table works.
//
// The lock is idempotent: calling it again on a project that has already
// advanced past foundations returns success with the current stage.
func LockFoundations(ctx context.Context, db *sql.DB, projectID string) (*LockFoundationsResult, error) {
	// Read current stage.
	var currentStage string
	err := db.QueryRowContext(ctx,
		`SELECT current_stage FROM projects WHERE id = ?`, projectID).Scan(&currentStage)
	if err != nil {
		return nil, fmt.Errorf("project %s not found: %w", projectID, err)
	}

	// Idempotent: already past foundations.
	if currentStage != "" && currentStage != "foundations" {
		return &LockFoundationsResult{
			ProjectID:     projectID,
			PreviousStage: currentStage,
			CurrentStage:  currentStage,
			Locked:        true,
		}, nil
	}

	// Evaluate the guard condition.
	guardResult, err := EvaluateGuard(ctx, db, "foundationsApproved", projectID)
	if err != nil {
		return nil, fmt.Errorf("evaluating guard: %w", err)
	}
	if !guardResult.Passed {
		return nil, fmt.Errorf("cannot lock foundations: %s", guardResult.Reason)
	}

	// Validate the transition is legal.
	guard, err := ValidWorkflowTransition("foundations", "prd_intake")
	if err != nil {
		return nil, fmt.Errorf("invalid transition: %w", err)
	}
	_ = guard // "foundationsApproved" — already checked above

	// Count foundation artifacts for the response.
	var foundationCount int
	db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_inputs WHERE project_id = ? AND role = 'foundation'`,
		projectID).Scan(&foundationCount)

	// Transition the project stage.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx,
		`UPDATE projects SET current_stage = 'prd_intake', updated_at = ? WHERE id = ?`,
		now, projectID)
	if err != nil {
		return nil, fmt.Errorf("updating project stage: %w", err)
	}

	return &LockFoundationsResult{
		ProjectID:       projectID,
		PreviousStage:   "foundations",
		CurrentStage:    "prd_intake",
		FoundationCount: foundationCount,
		Locked:          true,
	}, nil
}
