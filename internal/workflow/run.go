package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Run represents a single workflow run (model invocation or automated step).
type Run struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	Stage             string    `json:"stage"`
	ModelConfigID     string    `json:"model_config_id,omitempty"`
	Status            RunStatus `json:"status"`
	Attempt           int       `json:"attempt"`
	SessionHandle     string    `json:"session_handle,omitempty"`
	ContinuityMode    string    `json:"continuity_mode,omitempty"`
	TimeoutMs         int       `json:"timeout_ms,omitempty"`
	ProviderRequestID string    `json:"provider_request_id,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// RunRepository manages workflow run persistence. Status must be persisted
// before AND after each external model call for crash safety.
type RunRepository struct {
	db *sql.DB
}

// NewRunRepository creates a new repository backed by the given database.
func NewRunRepository(db *sql.DB) *RunRepository {
	return &RunRepository{db: db}
}

// Create inserts a new workflow run in "pending" status.
func (r *RunRepository) Create(ctx context.Context, projectID, stage, modelConfigID string) (*Run, error) {
	run := &Run{
		ID:            uuid.New().String(),
		ProjectID:     projectID,
		Stage:         stage,
		ModelConfigID: modelConfigID,
		Status:        RunPending,
		Attempt:       1,
		CreatedAt:     time.Now().UTC(),
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, attempt, session_handle, continuity_mode, timeout_ms, provider_request_id, started_at, completed_at, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.ProjectID, run.Stage, nullString(run.ModelConfigID),
		string(run.Status), run.Attempt, run.SessionHandle, run.ContinuityMode,
		run.TimeoutMs, run.ProviderRequestID, nil, nil, run.ErrorMessage,
		run.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("creating workflow run: %w", err)
	}
	return run, nil
}

// UpdateStatus transitions a run to a new status with validation.
// This must be called before and after every external model call.
func (r *RunRepository) UpdateStatus(ctx context.Context, runID string, newStatus RunStatus) error {
	run, err := r.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("fetching run for status update: %w", err)
	}

	if err := ValidateRunTransition(run.Status, newStatus); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	var startedAt, completedAt any
	if newStatus == RunRunning && run.StartedAt == nil {
		startedAt = now
	}
	if newStatus.IsTerminal() {
		completedAt = now
	}

	query := `UPDATE workflow_runs SET status = ?, started_at = COALESCE(?, started_at), completed_at = COALESCE(?, completed_at) WHERE id = ?`
	_, err = r.db.ExecContext(ctx, query, string(newStatus), startedAt, completedAt, runID)
	if err != nil {
		return fmt.Errorf("updating run status to %s: %w", newStatus, err)
	}
	return nil
}

// RecordAttempt increments the attempt counter for a retry.
func (r *RunRepository) RecordAttempt(ctx context.Context, runID string) (int, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE workflow_runs SET attempt = attempt + 1 WHERE id = ?`, runID)
	if err != nil {
		return 0, fmt.Errorf("incrementing attempt: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return 0, fmt.Errorf("workflow run %s not found", runID)
	}

	// Return the new attempt number.
	var attempt int
	err = r.db.QueryRowContext(ctx,
		`SELECT attempt FROM workflow_runs WHERE id = ?`, runID).Scan(&attempt)
	if err != nil {
		return 0, fmt.Errorf("reading updated attempt: %w", err)
	}
	return attempt, nil
}

// SetSessionHandle records the provider session handle for continuity.
func (r *RunRepository) SetSessionHandle(ctx context.Context, runID, handle, continuityMode string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE workflow_runs SET session_handle = ?, continuity_mode = ? WHERE id = ?`,
		handle, continuityMode, runID)
	if err != nil {
		return fmt.Errorf("setting session handle: %w", err)
	}
	return nil
}

// SetProviderRequestID records the provider's request ID for tracing.
func (r *RunRepository) SetProviderRequestID(ctx context.Context, runID, requestID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE workflow_runs SET provider_request_id = ? WHERE id = ?`,
		requestID, runID)
	if err != nil {
		return fmt.Errorf("setting provider request ID: %w", err)
	}
	return nil
}

// SetError records an error message on a run.
func (r *RunRepository) SetError(ctx context.Context, runID, errMsg string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE workflow_runs SET error_message = ? WHERE id = ?`,
		errMsg, runID)
	if err != nil {
		return fmt.Errorf("setting error message: %w", err)
	}
	return nil
}

// GetByID retrieves a single run by ID.
func (r *RunRepository) GetByID(ctx context.Context, runID string) (*Run, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, stage, model_config_id, status, attempt, session_handle, continuity_mode, timeout_ms, provider_request_id, started_at, completed_at, error_message, created_at
		 FROM workflow_runs WHERE id = ?`, runID)
	return scanRun(row)
}

// ListByProjectStage returns all runs for a project and stage, ordered by creation time.
func (r *RunRepository) ListByProjectStage(ctx context.Context, projectID, stage string) ([]*Run, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, stage, model_config_id, status, attempt, session_handle, continuity_mode, timeout_ms, provider_request_id, started_at, completed_at, error_message, created_at
		 FROM workflow_runs WHERE project_id = ? AND stage = ? ORDER BY created_at ASC`,
		projectID, stage)
	if err != nil {
		return nil, fmt.Errorf("listing runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		run, err := scanRunFromRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// FindInterruptedRuns returns all runs in "running" status, used at startup
// to mark them as "interrupted" for crash recovery.
func (r *RunRepository) FindInterruptedRuns(ctx context.Context) ([]*Run, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, stage, model_config_id, status, attempt, session_handle, continuity_mode, timeout_ms, provider_request_id, started_at, completed_at, error_message, created_at
		 FROM workflow_runs WHERE status = ?`, string(RunRunning))
	if err != nil {
		return nil, fmt.Errorf("finding interrupted runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		run, err := scanRunFromRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// MarkInterrupted transitions all runs in "running" status to "interrupted".
// Called at startup for crash recovery.
func (r *RunRepository) MarkInterrupted(ctx context.Context) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx,
		`UPDATE workflow_runs SET status = ?, completed_at = ? WHERE status = ?`,
		string(RunInterrupted), now, string(RunRunning))
	if err != nil {
		return 0, fmt.Errorf("marking interrupted runs: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// --- Helpers ---

func scanRun(row *sql.Row) (*Run, error) {
	var run Run
	var modelConfigID sql.NullString
	var status string
	var startedAt, completedAt sql.NullString
	var createdAt string

	err := row.Scan(
		&run.ID, &run.ProjectID, &run.Stage, &modelConfigID,
		&status, &run.Attempt, &run.SessionHandle, &run.ContinuityMode,
		&run.TimeoutMs, &run.ProviderRequestID, &startedAt, &completedAt,
		&run.ErrorMessage, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning workflow run: %w", err)
	}

	run.Status = RunStatus(status)
	if modelConfigID.Valid {
		run.ModelConfigID = modelConfigID.String
	}
	run.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, startedAt.String)
		run.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, completedAt.String)
		run.CompletedAt = &t
	}
	return &run, nil
}

func scanRunFromRows(rows *sql.Rows) (*Run, error) {
	var run Run
	var modelConfigID sql.NullString
	var status string
	var startedAt, completedAt sql.NullString
	var createdAt string

	err := rows.Scan(
		&run.ID, &run.ProjectID, &run.Stage, &modelConfigID,
		&status, &run.Attempt, &run.SessionHandle, &run.ContinuityMode,
		&run.TimeoutMs, &run.ProviderRequestID, &startedAt, &completedAt,
		&run.ErrorMessage, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning workflow run: %w", err)
	}

	run.Status = RunStatus(status)
	if modelConfigID.Valid {
		run.ModelConfigID = modelConfigID.String
	}
	run.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, startedAt.String)
		run.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, completedAt.String)
		run.CompletedAt = &t
	}
	return &run, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
