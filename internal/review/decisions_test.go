package review

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupReviewDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE review_items (
			id TEXT PRIMARY KEY, project_id TEXT NOT NULL, fragment_id TEXT NOT NULL,
			stage TEXT NOT NULL, run_id TEXT NOT NULL, severity TEXT NOT NULL,
			summary TEXT NOT NULL, rationale TEXT NOT NULL, suggested_change TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending', created_at TEXT NOT NULL
		)`,
		`CREATE TABLE review_decisions (
			id TEXT PRIMARY KEY, review_item_id TEXT NOT NULL REFERENCES review_items(id),
			action TEXT NOT NULL, user_note TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("creating table: %v", err)
		}
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateAndGetReviewItem(t *testing.T) {
	db := setupReviewDB(t)
	repo := NewReviewRepository(db)

	item, err := repo.CreateReviewItem(context.Background(),
		"proj_1", "frag_42", "stage_5", "run_1",
		"major", "Missing error handling", "No failure modes described", "Add error handling section")
	if err != nil {
		t.Fatalf("CreateReviewItem: %v", err)
	}
	if item.Status != StatusPending {
		t.Errorf("Status = %q, want %q", item.Status, StatusPending)
	}

	got, err := repo.GetReviewItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("GetReviewItem: %v", err)
	}
	if got.Summary != "Missing error handling" {
		t.Errorf("Summary = %q", got.Summary)
	}
	if got.Severity != "major" {
		t.Errorf("Severity = %q", got.Severity)
	}
}

func TestListReviewItemsByProject(t *testing.T) {
	db := setupReviewDB(t)
	repo := NewReviewRepository(db)

	repo.CreateReviewItem(context.Background(), "proj_1", "f1", "stage_5", "r1", "minor", "s1", "rat", "fix")
	repo.CreateReviewItem(context.Background(), "proj_1", "f2", "stage_5", "r1", "major", "s2", "rat", "fix")
	repo.CreateReviewItem(context.Background(), "proj_2", "f3", "stage_5", "r2", "minor", "s3", "rat", "fix")

	items, err := repo.ListByProject(context.Background(), "proj_1", "", "")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
}

func TestListReviewItems_FilterByStage(t *testing.T) {
	db := setupReviewDB(t)
	repo := NewReviewRepository(db)

	repo.CreateReviewItem(context.Background(), "proj_1", "f1", "stage_5", "r1", "minor", "s1", "rat", "fix")
	repo.CreateReviewItem(context.Background(), "proj_1", "f2", "stage_12", "r2", "major", "s2", "rat", "fix")

	items, err := repo.ListByProject(context.Background(), "proj_1", "stage_5", "")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("got %d items, want 1", len(items))
	}
}

func TestRecordDecision_Accept(t *testing.T) {
	db := setupReviewDB(t)
	repo := NewReviewRepository(db)

	item, _ := repo.CreateReviewItem(context.Background(), "proj_1", "f1", "s5", "r1", "minor", "s", "r", "fix")

	decision, err := repo.RecordDecision(context.Background(), item.ID, "accepted", "Looks good")
	if err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}
	if decision.Action != "accepted" {
		t.Errorf("Action = %q", decision.Action)
	}

	// Verify status updated.
	updated, _ := repo.GetReviewItem(context.Background(), item.ID)
	if updated.Status != StatusAccepted {
		t.Errorf("Status = %q, want %q", updated.Status, StatusAccepted)
	}
}

func TestRecordDecision_Reject(t *testing.T) {
	db := setupReviewDB(t)
	repo := NewReviewRepository(db)

	item, _ := repo.CreateReviewItem(context.Background(), "proj_1", "f1", "s5", "r1", "major", "s", "r", "fix")

	_, err := repo.RecordDecision(context.Background(), item.ID, "rejected", "Not applicable")
	if err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}

	updated, _ := repo.GetReviewItem(context.Background(), item.ID)
	if updated.Status != StatusRejected {
		t.Errorf("Status = %q, want %q", updated.Status, StatusRejected)
	}
}

func TestRecordDecision_AlreadyDecided(t *testing.T) {
	db := setupReviewDB(t)
	repo := NewReviewRepository(db)

	item, _ := repo.CreateReviewItem(context.Background(), "proj_1", "f1", "s5", "r1", "minor", "s", "r", "fix")
	repo.RecordDecision(context.Background(), item.ID, "accepted", "")

	_, err := repo.RecordDecision(context.Background(), item.ID, "rejected", "changed mind")
	if err == nil {
		t.Fatal("expected error for already-decided item")
	}
}

func TestRecordDecision_InvalidAction(t *testing.T) {
	db := setupReviewDB(t)
	repo := NewReviewRepository(db)

	item, _ := repo.CreateReviewItem(context.Background(), "proj_1", "f1", "s5", "r1", "minor", "s", "r", "fix")

	_, err := repo.RecordDecision(context.Background(), item.ID, "maybe", "")
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestBulkDecide(t *testing.T) {
	db := setupReviewDB(t)
	repo := NewReviewRepository(db)

	item1, _ := repo.CreateReviewItem(context.Background(), "proj_1", "f1", "s5", "r1", "minor", "s1", "r", "fix")
	item2, _ := repo.CreateReviewItem(context.Background(), "proj_1", "f2", "s5", "r1", "major", "s2", "r", "fix")
	item3, _ := repo.CreateReviewItem(context.Background(), "proj_1", "f3", "s5", "r1", "moderate", "s3", "r", "fix")

	accepted, rejected, err := repo.BulkDecide(context.Background(), []struct {
		ReviewItemID string
		Action       string
		UserNote     string
	}{
		{item1.ID, "accepted", ""},
		{item2.ID, "rejected", "Not relevant"},
		{item3.ID, "accepted", "Good point"},
	})
	if err != nil {
		t.Fatalf("BulkDecide: %v", err)
	}
	if accepted != 2 {
		t.Errorf("accepted = %d, want 2", accepted)
	}
	if rejected != 1 {
		t.Errorf("rejected = %d, want 1", rejected)
	}

	// Verify all statuses updated.
	i1, _ := repo.GetReviewItem(context.Background(), item1.ID)
	if i1.Status != StatusAccepted {
		t.Errorf("item1 status = %q", i1.Status)
	}
	i2, _ := repo.GetReviewItem(context.Background(), item2.ID)
	if i2.Status != StatusRejected {
		t.Errorf("item2 status = %q", i2.Status)
	}
}
