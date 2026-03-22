package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrDuplicateCommand indicates the idempotency key was already used.
var ErrDuplicateCommand = errors.New("duplicate command: idempotency key already exists")

// IdempotencyStatus tracks the lifecycle of an idempotent command.
type IdempotencyStatus string

const (
	IdempotencyReceived  IdempotencyStatus = "received"
	IdempotencyCompleted IdempotencyStatus = "completed"
	IdempotencyFailed    IdempotencyStatus = "failed"
)

// IdempotencyRecord represents a stored idempotency key and its outcome.
type IdempotencyRecord struct {
	Key         string            `json:"key"`
	ProjectID   string            `json:"project_id"`
	Command     string            `json:"command"`
	Status      IdempotencyStatus `json:"status"`
	ResultJSON  json.RawMessage   `json:"result_json"`
	CreatedAt   time.Time         `json:"created_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

// IdempotencyStore manages idempotency keys for mutating commands.
type IdempotencyStore struct {
	db *sql.DB
}

// NewIdempotencyStore creates a store backed by the given database.
func NewIdempotencyStore(db *sql.DB) *IdempotencyStore {
	return &IdempotencyStore{db: db}
}

// Acquire attempts to claim an idempotency key. Returns nil if the key is
// new (caller should proceed with the command). Returns the existing record
// if the key was already used (caller should return the stored result).
func (s *IdempotencyStore) Acquire(ctx context.Context, key, projectID, command string) (*IdempotencyRecord, error) {
	now := time.Now().UTC()

	// Try to insert the key. If it already exists, the INSERT will fail
	// due to PRIMARY KEY constraint.
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO idempotency_keys (key, project_id, command, status, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		key, projectID, command, string(IdempotencyReceived),
		now.Format(time.RFC3339Nano))

	if err != nil {
		// Key already exists — return the existing record.
		existing, lookupErr := s.Get(ctx, key)
		if lookupErr != nil {
			return nil, fmt.Errorf("idempotency key conflict but lookup failed: %w", err)
		}
		return existing, ErrDuplicateCommand
	}

	return nil, nil
}

// Complete marks an idempotency key as completed with its result.
func (s *IdempotencyStore) Complete(ctx context.Context, key string, result any) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling idempotency result: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err = s.db.ExecContext(ctx,
		`UPDATE idempotency_keys SET status = ?, result_json = ?, completed_at = ? WHERE key = ?`,
		string(IdempotencyCompleted), string(resultJSON), now, key)
	if err != nil {
		return fmt.Errorf("completing idempotency key: %w", err)
	}
	return nil
}

// Fail marks an idempotency key as failed with an error message.
func (s *IdempotencyStore) Fail(ctx context.Context, key string, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	resultJSON, _ := json.Marshal(map[string]string{"error": errMsg})

	_, err := s.db.ExecContext(ctx,
		`UPDATE idempotency_keys SET status = ?, result_json = ?, completed_at = ? WHERE key = ?`,
		string(IdempotencyFailed), string(resultJSON), now, key)
	if err != nil {
		return fmt.Errorf("failing idempotency key: %w", err)
	}
	return nil
}

// Get retrieves an idempotency record by key.
func (s *IdempotencyStore) Get(ctx context.Context, key string) (*IdempotencyRecord, error) {
	var rec IdempotencyRecord
	var status, resultJSON, createdAt string
	var completedAt sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT key, project_id, command, status, result_json, created_at, completed_at
		 FROM idempotency_keys WHERE key = ?`, key).
		Scan(&rec.Key, &rec.ProjectID, &rec.Command, &status, &resultJSON, &createdAt, &completedAt)
	if err != nil {
		return nil, fmt.Errorf("looking up idempotency key: %w", err)
	}

	rec.Status = IdempotencyStatus(status)
	rec.ResultJSON = json.RawMessage(resultJSON)
	rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, completedAt.String)
		rec.CompletedAt = &t
	}
	return &rec, nil
}
