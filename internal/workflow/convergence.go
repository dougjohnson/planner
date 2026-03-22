package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ConvergenceStatus describes whether a review loop has converged.
type ConvergenceStatus string

const (
	ConvergenceNone     ConvergenceStatus = "none"
	ConvergenceDetected ConvergenceStatus = "detected"
	ConvergenceAccepted ConvergenceStatus = "accepted"
	ConvergenceDeclined ConvergenceStatus = "declined"
)

// ConvergenceResult holds the outcome of a convergence check.
type ConvergenceResult struct {
	Status             ConvergenceStatus `json:"status"`
	IterationNumber    int               `json:"iteration_number"`
	RemainingLoops     int               `json:"remaining_loops"`
	OperationCount     int               `json:"operation_count"`
	Message            string            `json:"message"`
}

// CheckConvergence examines a CommitResult to determine if the review loop
// has converged (zero fragment operations proposed). This is called after
// each Stage 8 (PRD) or Stage 15 (plan) commit pass.
func CheckConvergence(commitResult *CommitResult, iterationNumber, maxIterations int) ConvergenceResult {
	opCount := commitResult.UpdateCount + commitResult.AddCount + commitResult.RemoveCount
	remaining := maxIterations - iterationNumber

	if commitResult.NoChanges || opCount == 0 {
		return ConvergenceResult{
			Status:          ConvergenceDetected,
			IterationNumber: iterationNumber,
			RemainingLoops:  remaining,
			OperationCount:  0,
			Message: fmt.Sprintf(
				"Loop %d of %d: model proposed no changes. Continue or finish?",
				iterationNumber, maxIterations),
		}
	}

	return ConvergenceResult{
		Status:          ConvergenceNone,
		IterationNumber: iterationNumber,
		RemainingLoops:  remaining,
		OperationCount:  opCount,
		Message: fmt.Sprintf(
			"Loop %d of %d: %d fragment operations applied.",
			iterationNumber, maxIterations, opCount),
	}
}

// AcceptConvergence records that the user accepted early exit from the review
// loop. Emits a convergence_accepted workflow event and returns metadata
// for the export manifest.
func AcceptConvergence(ctx context.Context, db *sql.DB, projectID string, result ConvergenceResult) error {
	eventID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	payload := fmt.Sprintf(
		`{"iteration":%d,"remaining":%d,"accepted_at":"%s"}`,
		result.IterationNumber, result.RemainingLoops, now)

	_, err := db.ExecContext(ctx,
		`INSERT INTO workflow_events (id, project_id, event_type, payload_json, created_at)
		 VALUES (?, ?, 'convergence_accepted', ?, ?)`,
		eventID, projectID, payload, now)
	if err != nil {
		return fmt.Errorf("recording convergence acceptance: %w", err)
	}

	return nil
}

// DeclineConvergence records that the user chose to continue the loop
// despite convergence detection, optionally with additional guidance.
func DeclineConvergence(ctx context.Context, db *sql.DB, projectID string, result ConvergenceResult, guidance string) error {
	eventID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	payload := fmt.Sprintf(
		`{"iteration":%d,"remaining":%d,"declined_at":"%s","has_guidance":%t}`,
		result.IterationNumber, result.RemainingLoops, now, guidance != "")

	_, err := db.ExecContext(ctx,
		`INSERT INTO workflow_events (id, project_id, event_type, payload_json, created_at)
		 VALUES (?, ?, 'convergence_declined', ?, ?)`,
		eventID, projectID, payload, now)
	if err != nil {
		return fmt.Errorf("recording convergence decline: %w", err)
	}

	// If guidance provided, store it for the next iteration.
	if guidance != "" {
		guidanceID := uuid.New().String()
		_, err = db.ExecContext(ctx,
			`INSERT INTO guidance_injections (id, project_id, stage, guidance_mode, content, created_at)
			 VALUES (?, ?, 'prd_review', 'advisory_only', ?, ?)`,
			guidanceID, projectID, guidance, now)
		if err != nil {
			return fmt.Errorf("storing guidance for next iteration: %w", err)
		}
	}

	return nil
}

// GetConvergenceHistory returns all convergence events for a project,
// useful for the export manifest's reproducibility metadata.
func GetConvergenceHistory(ctx context.Context, db *sql.DB, projectID string) ([]ConvergenceResult, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT event_type, payload_json FROM workflow_events
		 WHERE project_id = ? AND event_type IN ('convergence_accepted', 'convergence_declined')
		 ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("querying convergence history: %w", err)
	}
	defer rows.Close()

	var results []ConvergenceResult
	for rows.Next() {
		var eventType, payload string
		if err := rows.Scan(&eventType, &payload); err != nil {
			return nil, err
		}
		status := ConvergenceAccepted
		if eventType == "convergence_declined" {
			status = ConvergenceDeclined
		}
		results = append(results, ConvergenceResult{
			Status:  status,
			Message: eventType,
		})
	}
	return results, rows.Err()
}
