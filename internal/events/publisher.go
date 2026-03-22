// Package events provides workflow event publishing and persistence for
// flywheel-planner. Events are persisted to the workflow_events table and
// simultaneously pushed to the SSE hub for real-time dashboard updates.
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/google/uuid"
)

// Event types from §14.3.
const (
	StageStarted    = "workflow:stage_started"
	StageCompleted  = "workflow:stage_completed"
	StageFailed     = "workflow:stage_failed"
	StageBlocked    = "workflow:stage_blocked"
	RunStarted      = "workflow:run_started"
	RunRetrying     = "workflow:run_retrying"
	RunFailed       = "workflow:run_failed"
	RunCompleted    = "workflow:run_completed"
	RunProgress     = "workflow:run_progress"
	ReviewReady     = "workflow:review_ready"
	LoopTick        = "workflow:loop_tick"
	StateChanged    = "workflow:state_changed"
	ArtifactCreated = "workflow:artifact_created"
	ExportCompleted = "workflow:export_completed"
)

// Payload is the structured data attached to a workflow event.
// Fields are optional depending on event type.
type Payload struct {
	Stage       string   `json:"stage,omitempty"`
	RunID       string   `json:"run_id,omitempty"`
	ArtifactIDs []string `json:"artifact_ids,omitempty"`
	Provider    string   `json:"provider,omitempty"`
	Error       string   `json:"error,omitempty"`
	Iteration   int      `json:"iteration,omitempty"`
	Message     string   `json:"message,omitempty"`
}

// Publisher persists events and pushes them to the SSE hub.
type Publisher struct {
	db     *sql.DB
	hub    *sse.Hub
	logger *slog.Logger
}

// NewPublisher creates a new event Publisher.
func NewPublisher(db *sql.DB, hub *sse.Hub, logger *slog.Logger) *Publisher {
	return &Publisher{
		db:     db,
		hub:    hub,
		logger: logger,
	}
}

// Publish creates a workflow_events record and pushes the event to the SSE hub.
func (p *Publisher) Publish(ctx context.Context, projectID, eventType string, workflowRunID string, payload Payload) error {
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling event payload: %w", err)
	}

	// Persist to workflow_events table.
	var runID any
	if workflowRunID != "" {
		runID = workflowRunID
	}

	_, err = p.db.ExecContext(ctx, `
		INSERT INTO workflow_events (id, project_id, workflow_run_id, event_type, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, projectID, runID, eventType, string(payloadJSON), now)
	if err != nil {
		return fmt.Errorf("persisting event: %w", err)
	}

	// Push to SSE hub for real-time delivery.
	if p.hub != nil {
		p.hub.Publish(projectID, eventType, map[string]any{
			"event_id":   id,
			"project_id": projectID,
			"event_type": eventType,
			"payload":    payload,
			"created_at": now,
		})
	}

	p.logger.Debug("event published",
		"event_id", id,
		"project_id", projectID,
		"event_type", eventType,
	)

	return nil
}

// ListByProject returns recent events for a project, newest first.
func (p *Publisher) ListByProject(ctx context.Context, projectID string, limit int) ([]StoredEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := p.db.QueryContext(ctx, `
		SELECT id, project_id, workflow_run_id, event_type, payload_json, created_at
		FROM workflow_events
		WHERE project_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing events: %w", err)
	}
	defer rows.Close()

	var events []StoredEvent
	for rows.Next() {
		var e StoredEvent
		var runID sql.NullString
		if err := rows.Scan(&e.ID, &e.ProjectID, &runID, &e.EventType, &e.PayloadJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		if runID.Valid {
			e.WorkflowRunID = runID.String
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// StoredEvent is a persisted workflow event.
type StoredEvent struct {
	ID             string `json:"id"`
	ProjectID      string `json:"project_id"`
	WorkflowRunID  string `json:"workflow_run_id,omitempty"`
	EventType      string `json:"event_type"`
	PayloadJSON    string `json:"payload_json"`
	CreatedAt      string `json:"created_at"`
}
