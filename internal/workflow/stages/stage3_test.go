package stages

import (
	"context"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

func seedStage3Project(t testing.TB, tdb *testutil.TestDB) string {
	t.Helper()
	projectID := "proj-stage3"
	now := time.Now().UTC().Format(time.RFC3339)
	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES (?, 'Stage3 Test', 'active', ?, ?)",
		projectID, now, now)
	return projectID
}

func TestHandleSubmitDocument(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	fragStore := fragments.NewStore(tdb.DB)
	snapCreator := artifacts.NewSnapshotCreator(tdb.DB)
	handler := NewStage3Handler(tdb.DB, fragStore, snapCreator, nil, tdb.Logger)
	projectID := seedStage3Project(t, tdb)

	content := []byte("# My PRD\n\nIntro.\n\n## Overview\n\nOverview content.\n\n## Requirements\n\nReq content.\n")

	result, err := handler.HandleSubmitDocument(context.Background(), projectID, "gpt-4o", "run-001", content)
	if err != nil {
		t.Fatalf("HandleSubmitDocument: %v", err)
	}

	if result.ArtifactID == "" {
		t.Error("expected non-empty artifact ID")
	}
	if result.FragmentCount != 3 { // preamble + 2 headings
		t.Errorf("expected 3 fragments, got %d", result.FragmentCount)
	}
	if result.NewFragments != 3 {
		t.Errorf("expected 3 new fragments, got %d", result.NewFragments)
	}

	// Verify artifact is NOT canonical.
	var isCanonical int
	tdb.QueryRow("SELECT is_canonical FROM artifacts WHERE id = ?", result.ArtifactID).Scan(&isCanonical)
	if isCanonical != 0 {
		t.Error("Stage 3 artifact should NOT be canonical")
	}
}

func TestHandleSubmitDocument_TwoModels(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	fragStore := fragments.NewStore(tdb.DB)
	snapCreator := artifacts.NewSnapshotCreator(tdb.DB)
	handler := NewStage3Handler(tdb.DB, fragStore, snapCreator, nil, tdb.Logger)
	projectID := seedStage3Project(t, tdb)
	ctx := context.Background()

	gptContent := []byte("## Overview\n\nGPT overview.\n\n## Design\n\nGPT design.\n")
	opusContent := []byte("## Overview\n\nOpus overview.\n\n## Architecture\n\nOpus architecture.\n")

	r1, err := handler.HandleSubmitDocument(ctx, projectID, "gpt-4o", "run-gpt", gptContent)
	if err != nil {
		t.Fatalf("GPT submit: %v", err)
	}

	r2, err := handler.HandleSubmitDocument(ctx, projectID, "claude-opus", "run-opus", opusContent)
	if err != nil {
		t.Fatalf("Opus submit: %v", err)
	}

	// Both should produce artifacts.
	if r1.ArtifactID == r2.ArtifactID {
		t.Error("two models should produce different artifact IDs")
	}

	// Neither should be canonical.
	var count int
	tdb.QueryRow("SELECT COUNT(*) FROM artifacts WHERE project_id = ? AND is_canonical = 1", projectID).Scan(&count)
	if count != 0 {
		t.Errorf("no Stage 3 artifacts should be canonical, got %d", count)
	}

	// Should have 2 artifacts total.
	var totalArtifacts int
	tdb.QueryRow("SELECT COUNT(*) FROM artifacts WHERE project_id = ?", projectID).Scan(&totalArtifacts)
	if totalArtifacts != 2 {
		t.Errorf("expected 2 artifacts, got %d", totalArtifacts)
	}
}

func TestHandleSubmitDocument_EmptyContent(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	fragStore := fragments.NewStore(tdb.DB)
	snapCreator := artifacts.NewSnapshotCreator(tdb.DB)
	handler := NewStage3Handler(tdb.DB, fragStore, snapCreator, nil, tdb.Logger)

	_, err := handler.HandleSubmitDocument(context.Background(), "proj-1", "gpt-4o", "run-001", nil)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestHandleSubmitDocument_VersionLabel(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	fragStore := fragments.NewStore(tdb.DB)
	snapCreator := artifacts.NewSnapshotCreator(tdb.DB)
	handler := NewStage3Handler(tdb.DB, fragStore, snapCreator, nil, tdb.Logger)
	projectID := seedStage3Project(t, tdb)

	content := []byte("## Section\n\nContent.\n")
	result, _ := handler.HandleSubmitDocument(context.Background(), projectID, "gpt-4o", "run-001", content)

	if result.VersionLabel == "" {
		t.Error("expected non-empty version label")
	}
	// Should contain "prd" and "generated".
	if result.VersionLabel != "prd.v01.generated" {
		t.Errorf("expected 'prd.v01.generated', got %q", result.VersionLabel)
	}
}
