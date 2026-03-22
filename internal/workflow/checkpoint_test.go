package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

func seedCheckpointProject(t testing.TB, tdb *testutil.TestDB) string {
	t.Helper()
	projectID := "proj-cp-test"
	now := time.Now().UTC().Format(time.RFC3339)
	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES (?, 'Test', 'active', ?, ?)",
		projectID, now, now)
	return projectID
}

func TestSaveAndLoad(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewCheckpointStore(tdb.DB)
	projectID := seedCheckpointProject(t, tdb)
	ctx := context.Background()

	cp := &Checkpoint{
		ProjectID:    projectID,
		CurrentStage: "stage-5",
		CompletedStages: map[string]StageOutcome{
			"stage-1": {Stage: "stage-1", Status: "completed"},
			"stage-2": {Stage: "stage-2", Status: "completed"},
			"stage-3": {Stage: "stage-3", Status: "completed", ArtifactID: "art-1"},
			"stage-4": {Stage: "stage-4", Status: "completed", ArtifactID: "art-2"},
		},
		CanonicalArtifacts: map[string]string{
			"prd-stream": "art-2",
		},
		LoopIterations: map[string]int{
			"prd-review": 0,
		},
	}

	err := store.Save(ctx, cp)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ctx, projectID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.CurrentStage != "stage-5" {
		t.Errorf("expected stage-5, got %s", loaded.CurrentStage)
	}
	if len(loaded.CompletedStages) != 4 {
		t.Errorf("expected 4 completed stages, got %d", len(loaded.CompletedStages))
	}
	if loaded.CanonicalArtifacts["prd-stream"] != "art-2" {
		t.Errorf("expected canonical artifact art-2")
	}
	if loaded.Checksum == "" {
		t.Error("expected non-empty checksum")
	}
}

func TestLoad_NoCheckpoint(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewCheckpointStore(tdb.DB)
	ctx := context.Background()

	_, err := store.Load(ctx, "nonexistent")
	if !errors.Is(err, ErrNoCheckpoint) {
		t.Errorf("expected ErrNoCheckpoint, got %v", err)
	}
}

func TestDeterministic(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewCheckpointStore(tdb.DB)
	projectID := seedCheckpointProject(t, tdb)
	ctx := context.Background()

	makeCheckpoint := func() *Checkpoint {
		return &Checkpoint{
			ProjectID:    projectID,
			CurrentStage: "stage-3",
			CompletedStages: map[string]StageOutcome{
				"stage-1": {Stage: "stage-1", Status: "completed"},
				"stage-2": {Stage: "stage-2", Status: "completed"},
			},
			CanonicalArtifacts: map[string]string{},
			LoopIterations:     map[string]int{},
			CreatedAt:          "2026-01-01T00:00:00Z", // Fixed timestamp for determinism.
		}
	}

	cp1 := makeCheckpoint()
	cp2 := makeCheckpoint()

	// Compute checksums without saving.
	cp1.Checksum = ""
	cp2.Checksum = ""
	data1, _ := marshalDeterministic(cp1)
	data2, _ := marshalDeterministic(cp2)

	if string(data1) != string(data2) {
		t.Errorf("identical state should produce identical bytes:\n  %s\n  %s", data1, data2)
	}

	// Save and verify checksum matches.
	store.Save(ctx, makeCheckpoint())
	loaded, _ := store.Load(ctx, projectID)
	if loaded.Checksum == "" {
		t.Error("loaded checkpoint should have checksum")
	}
}

func TestLatestCheckpointWins(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewCheckpointStore(tdb.DB)
	projectID := seedCheckpointProject(t, tdb)
	ctx := context.Background()

	// Save two checkpoints.
	store.Save(ctx, &Checkpoint{
		ProjectID:    projectID,
		CurrentStage: "stage-3",
	})
	store.Save(ctx, &Checkpoint{
		ProjectID:    projectID,
		CurrentStage: "stage-7",
	})

	loaded, err := store.Load(ctx, projectID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.CurrentStage != "stage-7" {
		t.Errorf("expected latest checkpoint stage-7, got %s", loaded.CurrentStage)
	}
}

func TestChecksumIntegrity(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewCheckpointStore(tdb.DB)
	projectID := seedCheckpointProject(t, tdb)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)

	// Insert a corrupt checkpoint directly.
	tdb.Exec(`INSERT INTO workflow_events (id, project_id, event_type, payload_json, created_at)
		VALUES ('corrupt-1', ?, ?, '{"version":1,"project_id":"proj-cp-test","current_stage":"stage-3","checksum":"wrong"}', ?)`,
		projectID, checkpointEventType, now)

	_, err := store.Load(ctx, projectID)
	if !errors.Is(err, ErrCorruptCheckpoint) {
		t.Errorf("expected ErrCorruptCheckpoint for bad checksum, got %v", err)
	}
}

func TestSave_EmptyProjectID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewCheckpointStore(tdb.DB)

	err := store.Save(context.Background(), &Checkpoint{})
	if err == nil {
		t.Fatal("expected error for empty project_id")
	}
}

func TestCheckpoint_WithPendingReview(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewCheckpointStore(tdb.DB)
	projectID := seedCheckpointProject(t, tdb)
	ctx := context.Background()

	store.Save(ctx, &Checkpoint{
		ProjectID:     projectID,
		CurrentStage:  "stage-6",
		PendingReview: true,
	})

	loaded, _ := store.Load(ctx, projectID)
	if !loaded.PendingReview {
		t.Error("expected PendingReview=true")
	}
}

func TestCheckpoint_NilMaps(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewCheckpointStore(tdb.DB)
	projectID := seedCheckpointProject(t, tdb)
	ctx := context.Background()

	// Save with nil maps — should not panic.
	err := store.Save(ctx, &Checkpoint{
		ProjectID:    projectID,
		CurrentStage: "stage-1",
	})
	if err != nil {
		t.Fatalf("Save with nil maps: %v", err)
	}

	loaded, err := store.Load(ctx, projectID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.CurrentStage != "stage-1" {
		t.Errorf("expected stage-1, got %s", loaded.CurrentStage)
	}
}
