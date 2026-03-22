package events

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/dougflynn/flywheel-planner/internal/db/migrations"
	_ "modernc.org/sqlite"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")
	if err := migrations.Run(context.Background(), db, testLogger()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	// Seed a test project.
	db.ExecContext(context.Background(),
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')")
	t.Cleanup(func() { db.Close() })
	return db
}

func TestPublish_PersistsEvent(t *testing.T) {
	db := setupTestDB(t)
	hub := sse.NewHub(testLogger())
	pub := NewPublisher(db, hub, testLogger())
	ctx := context.Background()

	err := pub.Publish(ctx, "p-1", StageStarted, "", Payload{
		Stage:   "prd_intake",
		Message: "Starting PRD intake",
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	var count int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workflow_events WHERE project_id = 'p-1'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

func TestPublish_WithWorkflowRunID(t *testing.T) {
	db := setupTestDB(t)
	pub := NewPublisher(db, nil, testLogger())
	ctx := context.Background()

	// Create a workflow run.
	db.ExecContext(ctx,
		"INSERT INTO workflow_runs (id, project_id, stage, created_at) VALUES ('wr-1', 'p-1', 's1', '2026-01-01T00:00:00Z')")

	err := pub.Publish(ctx, "p-1", RunStarted, "wr-1", Payload{
		RunID:    "wr-1",
		Provider: "openai",
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	var runID sql.NullString
	db.QueryRowContext(ctx, "SELECT workflow_run_id FROM workflow_events WHERE project_id = 'p-1'").Scan(&runID)
	if !runID.Valid || runID.String != "wr-1" {
		t.Errorf("expected workflow_run_id 'wr-1', got %v", runID)
	}
}

func TestPublish_PushesToSSEHub(t *testing.T) {
	db := setupTestDB(t)
	hub := sse.NewHub(testLogger())
	pub := NewPublisher(db, hub, testLogger())
	ctx := context.Background()

	// Subscribe to SSE events.
	events, cancel := hub.Subscribe(ctx, "p-1")
	defer cancel()

	err := pub.Publish(ctx, "p-1", ArtifactCreated, "", Payload{
		ArtifactIDs: []string{"a-1"},
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case evt := <-events:
		if evt.Type != ArtifactCreated {
			t.Errorf("expected event type %q, got %q", ArtifactCreated, evt.Type)
		}
	default:
		t.Error("expected SSE event but none received")
	}
}

func TestPublish_NilHub(t *testing.T) {
	db := setupTestDB(t)
	pub := NewPublisher(db, nil, testLogger())
	ctx := context.Background()

	// Should not panic with nil hub.
	err := pub.Publish(ctx, "p-1", StageCompleted, "", Payload{Stage: "s1"})
	if err != nil {
		t.Fatalf("Publish with nil hub: %v", err)
	}
}

func TestListByProject(t *testing.T) {
	db := setupTestDB(t)
	pub := NewPublisher(db, nil, testLogger())
	ctx := context.Background()

	pub.Publish(ctx, "p-1", StageStarted, "", Payload{Stage: "s1"})
	pub.Publish(ctx, "p-1", StageCompleted, "", Payload{Stage: "s1"})
	pub.Publish(ctx, "p-1", RunStarted, "", Payload{RunID: "r1"})

	events, err := pub.ListByProject(ctx, "p-1", 10)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
	// Newest first.
	if events[0].EventType != RunStarted {
		t.Errorf("expected newest event first, got %q", events[0].EventType)
	}
}

func TestListByProject_Limit(t *testing.T) {
	db := setupTestDB(t)
	pub := NewPublisher(db, nil, testLogger())
	ctx := context.Background()

	for range 5 {
		pub.Publish(ctx, "p-1", LoopTick, "", Payload{Iteration: 1})
	}

	events, _ := pub.ListByProject(ctx, "p-1", 2)
	if len(events) != 2 {
		t.Errorf("expected 2 events with limit, got %d", len(events))
	}
}

func TestAllEventTypes(t *testing.T) {
	// Verify all 14 event type constants are defined.
	types := []string{
		StageStarted, StageCompleted, StageFailed, StageBlocked,
		RunStarted, RunRetrying, RunFailed, RunCompleted, RunProgress,
		ReviewReady, LoopTick, StateChanged, ArtifactCreated, ExportCompleted,
	}
	if len(types) != 14 {
		t.Errorf("expected 14 event types, got %d", len(types))
	}
	for _, typ := range types {
		if typ == "" {
			t.Error("empty event type constant")
		}
	}
}
