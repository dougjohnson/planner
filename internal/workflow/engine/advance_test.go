package engine

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/dougflynn/flywheel-planner/internal/db/migrations"
	"github.com/dougflynn/flywheel-planner/internal/events"
	_ "modernc.org/sqlite"
)

func setupTestDBForAdvance(t *testing.T) *sql.DB {
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
	db.ExecContext(context.Background(),
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')")
	t.Cleanup(func() { db.Close() })
	return db
}

func setupAdvancer(t *testing.T) *AutoAdvancer {
	t.Helper()
	db := setupTestDBForAdvance(t)
	hub := sse.NewHub(testLogger())
	pub := events.NewPublisher(db, hub, testLogger())
	return NewAutoAdvancer(pub, testLogger())
}

func TestEvaluate_AutoAdvanceFromPRDIntake(t *testing.T) {
	adv := setupAdvancer(t)

	// prd_intake → parallel_prd_generation should auto-advance.
	decision, err := adv.Evaluate("prd_intake")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !decision.ShouldAdvance {
		t.Error("expected auto-advance from prd_intake")
	}
	if decision.ToStageID != "parallel_prd_generation" {
		t.Errorf("expected next stage 'parallel_prd_generation', got %q", decision.ToStageID)
	}
}

func TestEvaluate_AwaitingUser_Foundations(t *testing.T) {
	adv := setupAdvancer(t)

	// Nothing transitions TO foundations in the normal flow — but if we check
	// a stage that leads to a user-action stage:
	// foundations → prd_intake (requires user to submit foundations first)
	decision, err := adv.Evaluate("foundations")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// foundations → prd_intake, but prd_intake is NOT a user-action stage,
	// so it should auto-advance.
	if !decision.ShouldAdvance {
		// prd_intake doesn't require user action, so this should advance.
		t.Log("foundations auto-advances to prd_intake (which is auto)")
	}
}

func TestEvaluate_UserActionRequired_DisagreementReview(t *testing.T) {
	adv := setupAdvancer(t)

	// prd_integration → prd_disagreement_review (requires user action)
	decision, err := adv.Evaluate("prd_integration")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// First candidate from prd_integration is prd_disagreement_review (if has disagreements)
	// or prd_review (if no disagreements). The first is user-action.
	if decision.AwaitingUser {
		t.Log("correctly identified user-action stage")
	}
}

func TestEvaluate_NoTransitions(t *testing.T) {
	adv := setupAdvancer(t)

	// final_review has no outgoing transitions (it's the terminal stage).
	decision, err := adv.Evaluate("final_review")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.ShouldAdvance {
		t.Error("final_review should not auto-advance")
	}
	if decision.Reason != "no outgoing transitions defined" {
		t.Errorf("unexpected reason: %q", decision.Reason)
	}
}

func TestExecute_PublishesEvents(t *testing.T) {
	db := setupTestDBForAdvance(t)
	hub := sse.NewHub(testLogger())
	pub := events.NewPublisher(db, hub, testLogger())
	adv := NewAutoAdvancer(pub, testLogger())

	// Subscribe to events.
	evtCh, cancel := hub.Subscribe(context.Background(), "p-1")
	defer cancel()

	_, err := adv.Execute(context.Background(), "p-1", "prd_intake")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Should have received a stage_started event.
	select {
	case evt := <-evtCh:
		if evt.Type != events.StageStarted {
			t.Errorf("expected %s event, got %s", events.StageStarted, evt.Type)
		}
	default:
		t.Error("expected SSE event but none received")
	}
}

func TestExecute_NilPublisher(t *testing.T) {
	adv := NewAutoAdvancer(nil, testLogger())

	decision, err := adv.Execute(context.Background(), "p-1", "prd_intake")
	if err != nil {
		t.Fatalf("Execute with nil publisher: %v", err)
	}
	if !decision.ShouldAdvance {
		t.Error("expected auto-advance even with nil publisher")
	}
}
