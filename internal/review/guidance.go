// Package review provides the review decision engine and user guidance
// management for flywheel-planner.
package review

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrGuidanceNotFound is returned when a guidance entry does not exist.
	ErrGuidanceNotFound = errors.New("guidance not found")
)

// GuidanceMode determines how guidance is treated in prompt assembly.
type GuidanceMode string

const (
	// ModeAdvisoryOnly means guidance is injected as advisory text the model should consider.
	ModeAdvisoryOnly GuidanceMode = "advisory_only"

	// ModeDecisionRecord means guidance is a user decision that must be respected.
	ModeDecisionRecord GuidanceMode = "decision_record"
)

// Guidance represents a user-submitted guidance injection.
type Guidance struct {
	ID           string       `json:"id"`
	ProjectID    string       `json:"project_id"`
	Stage        string       `json:"stage"`
	GuidanceMode GuidanceMode `json:"guidance_mode"`
	Content      string       `json:"content"`
	CreatedAt    string       `json:"created_at"`
}

// GuidanceSubmission is the input for creating a new guidance entry.
type GuidanceSubmission struct {
	Content       string       `json:"content"`
	GuidanceMode  GuidanceMode `json:"guidance_mode"`
	TargetStage   string       `json:"target_stage,omitempty"`
	IdempotencyKey string      `json:"idempotency_key,omitempty"`
}

// GuidanceService provides operations for guidance injections.
type GuidanceService struct {
	db *sql.DB
}

// NewGuidanceService creates a new guidance service.
func NewGuidanceService(db *sql.DB) *GuidanceService {
	return &GuidanceService{db: db}
}

// Submit creates a new guidance entry. If an idempotency key is provided
// and a matching entry exists, the existing entry is returned without
// creating a duplicate.
func (s *GuidanceService) Submit(ctx context.Context, projectID string, sub GuidanceSubmission) (*Guidance, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if sub.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if sub.GuidanceMode != ModeAdvisoryOnly && sub.GuidanceMode != ModeDecisionRecord {
		return nil, fmt.Errorf("guidance_mode must be 'advisory_only' or 'decision_record', got %q", sub.GuidanceMode)
	}
	if sub.TargetStage == "" {
		return nil, fmt.Errorf("target_stage is required")
	}

	// Idempotency check: if a key is provided, look for existing entry.
	if sub.IdempotencyKey != "" {
		existing, err := s.findByIdempotencyKey(ctx, projectID, sub.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
	}

	id := uuid.NewString()
	// Use idempotency key as ID if provided for easy lookup.
	if sub.IdempotencyKey != "" {
		id = sub.IdempotencyKey
	}

	g := &Guidance{
		ID:           id,
		ProjectID:    projectID,
		Stage:        sub.TargetStage,
		GuidanceMode: sub.GuidanceMode,
		Content:      sub.Content,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO guidance_injections (id, project_id, stage, guidance_mode, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		g.ID, g.ProjectID, g.Stage, g.GuidanceMode, g.Content, g.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting guidance: %w", err)
	}

	return g, nil
}

// GetByID returns a guidance entry by its ID.
func (s *GuidanceService) GetByID(ctx context.Context, id string) (*Guidance, error) {
	g := &Guidance{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, stage, guidance_mode, content, created_at
		 FROM guidance_injections WHERE id = ?`, id,
	).Scan(&g.ID, &g.ProjectID, &g.Stage, &g.GuidanceMode, &g.Content, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrGuidanceNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying guidance: %w", err)
	}
	return g, nil
}

// ListByProject returns all guidance entries for a project, optionally
// filtered by stage. Results are sorted by created_at ascending.
func (s *GuidanceService) ListByProject(ctx context.Context, projectID string, stage string) ([]*Guidance, error) {
	var rows *sql.Rows
	var err error

	if stage != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, project_id, stage, guidance_mode, content, created_at
			 FROM guidance_injections
			 WHERE project_id = ? AND stage = ?
			 ORDER BY created_at ASC`, projectID, stage)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, project_id, stage, guidance_mode, content, created_at
			 FROM guidance_injections
			 WHERE project_id = ?
			 ORDER BY created_at ASC`, projectID)
	}
	if err != nil {
		return nil, fmt.Errorf("listing guidance: %w", err)
	}
	defer rows.Close()

	var results []*Guidance
	for rows.Next() {
		g := &Guidance{}
		if err := rows.Scan(&g.ID, &g.ProjectID, &g.Stage, &g.GuidanceMode, &g.Content, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning guidance: %w", err)
		}
		results = append(results, g)
	}
	return results, rows.Err()
}

// ListByStage returns all guidance entries for a specific project and stage,
// ordered chronologically. Used by the prompt assembly pipeline.
func (s *GuidanceService) ListByStage(ctx context.Context, projectID, stage string) ([]*Guidance, error) {
	return s.ListByProject(ctx, projectID, stage)
}

// findByIdempotencyKey looks up a guidance entry by its ID (used as idempotency key).
func (s *GuidanceService) findByIdempotencyKey(ctx context.Context, projectID, key string) (*Guidance, error) {
	return s.GetByID(ctx, key)
}
