package review

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

func seedGuidanceProject(t testing.TB, tdb *testutil.TestDB) string {
	t.Helper()
	projectID := "proj-guidance-test"
	now := time.Now().UTC().Format(time.RFC3339)
	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES (?, 'Test', 'active', ?, ?)",
		projectID, now, now)
	return projectID
}

func TestSubmit_AdvisoryGuidance(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)
	projectID := seedGuidanceProject(t, tdb)

	g, err := svc.Submit(context.Background(), projectID, GuidanceSubmission{
		Content:      "Focus on security requirements in the auth section.",
		GuidanceMode: ModeAdvisoryOnly,
		TargetStage:  "stage-7",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if g.ID == "" {
		t.Error("expected non-empty ID")
	}
	if g.GuidanceMode != ModeAdvisoryOnly {
		t.Errorf("expected advisory_only, got %q", g.GuidanceMode)
	}
	if g.Stage != "stage-7" {
		t.Errorf("expected stage-7, got %q", g.Stage)
	}
	if g.Content != "Focus on security requirements in the auth section." {
		t.Errorf("content mismatch")
	}
}

func TestSubmit_DecisionRecord(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)
	projectID := seedGuidanceProject(t, tdb)

	g, err := svc.Submit(context.Background(), projectID, GuidanceSubmission{
		Content:      "Use PostgreSQL instead of SQLite for production.",
		GuidanceMode: ModeDecisionRecord,
		TargetStage:  "stage-14",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if g.GuidanceMode != ModeDecisionRecord {
		t.Errorf("expected decision_record, got %q", g.GuidanceMode)
	}
}

func TestSubmit_InvalidMode(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)
	projectID := seedGuidanceProject(t, tdb)

	_, err := svc.Submit(context.Background(), projectID, GuidanceSubmission{
		Content:      "test",
		GuidanceMode: "invalid_mode",
		TargetStage:  "stage-7",
	})
	if err == nil {
		t.Fatal("expected error for invalid guidance_mode")
	}
}

func TestSubmit_EmptyContent(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)
	projectID := seedGuidanceProject(t, tdb)

	_, err := svc.Submit(context.Background(), projectID, GuidanceSubmission{
		GuidanceMode: ModeAdvisoryOnly,
		TargetStage:  "stage-7",
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestSubmit_EmptyProjectID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)

	_, err := svc.Submit(context.Background(), "", GuidanceSubmission{
		Content:      "test",
		GuidanceMode: ModeAdvisoryOnly,
		TargetStage:  "stage-7",
	})
	if err == nil {
		t.Fatal("expected error for empty project_id")
	}
}

func TestSubmit_Idempotency(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)
	projectID := seedGuidanceProject(t, tdb)
	ctx := context.Background()

	key := "idempotency-key-001"

	g1, err := svc.Submit(ctx, projectID, GuidanceSubmission{
		Content:        "First submission",
		GuidanceMode:   ModeAdvisoryOnly,
		TargetStage:    "stage-7",
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	g2, err := svc.Submit(ctx, projectID, GuidanceSubmission{
		Content:        "Duplicate submission",
		GuidanceMode:   ModeAdvisoryOnly,
		TargetStage:    "stage-7",
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}

	if g1.ID != g2.ID {
		t.Errorf("idempotent submission should return same ID: %s != %s", g1.ID, g2.ID)
	}
	if g2.Content != "First submission" {
		t.Errorf("idempotent submission should return original content, got %q", g2.Content)
	}
}

func TestGetByID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)
	projectID := seedGuidanceProject(t, tdb)
	ctx := context.Background()

	created, _ := svc.Submit(ctx, projectID, GuidanceSubmission{
		Content: "Test guidance", GuidanceMode: ModeAdvisoryOnly, TargetStage: "stage-7",
	})

	got, err := svc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Content != "Test guidance" {
		t.Errorf("content mismatch")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)

	_, err := svc.GetByID(context.Background(), "nonexistent")
	if !errors.Is(err, ErrGuidanceNotFound) {
		t.Errorf("expected ErrGuidanceNotFound, got %v", err)
	}
}

func TestListByProject(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)
	projectID := seedGuidanceProject(t, tdb)
	ctx := context.Background()

	svc.Submit(ctx, projectID, GuidanceSubmission{
		Content: "G1", GuidanceMode: ModeAdvisoryOnly, TargetStage: "stage-7",
	})
	svc.Submit(ctx, projectID, GuidanceSubmission{
		Content: "G2", GuidanceMode: ModeDecisionRecord, TargetStage: "stage-14",
	})
	svc.Submit(ctx, projectID, GuidanceSubmission{
		Content: "G3", GuidanceMode: ModeAdvisoryOnly, TargetStage: "stage-7",
	})

	// All guidance for project.
	all, err := svc.ListByProject(ctx, projectID, "")
	if err != nil {
		t.Fatalf("ListByProject all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 entries, got %d", len(all))
	}

	// Filtered by stage.
	stage7, err := svc.ListByProject(ctx, projectID, "stage-7")
	if err != nil {
		t.Fatalf("ListByProject stage-7: %v", err)
	}
	if len(stage7) != 2 {
		t.Errorf("expected 2 stage-7 entries, got %d", len(stage7))
	}

	stage14, _ := svc.ListByProject(ctx, projectID, "stage-14")
	if len(stage14) != 1 {
		t.Errorf("expected 1 stage-14 entry, got %d", len(stage14))
	}
}

func TestListByStage(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)
	projectID := seedGuidanceProject(t, tdb)
	ctx := context.Background()

	svc.Submit(ctx, projectID, GuidanceSubmission{
		Content: "For stage 7", GuidanceMode: ModeAdvisoryOnly, TargetStage: "stage-7",
	})

	entries, err := svc.ListByStage(ctx, projectID, "stage-7")
	if err != nil {
		t.Fatalf("ListByStage: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1, got %d", len(entries))
	}
}

func TestListByProject_Empty(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewGuidanceService(tdb.DB)
	ctx := context.Background()

	entries, err := svc.ListByProject(ctx, "nonexistent", "")
	if err != nil {
		t.Fatalf("ListByProject empty: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
