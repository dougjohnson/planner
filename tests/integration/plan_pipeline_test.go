package integration

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/db/migrations"
	"github.com/dougflynn/flywheel-planner/internal/documents/composer"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/dougflynn/flywheel-planner/internal/workflow/stages"
	_ "modernc.org/sqlite"
)

func setupPlanPipelineDB(t *testing.T) (*sql.DB, *fragments.Store, *artifacts.SnapshotCreator, *composer.Composer) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if err := migrations.Run(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	// Seed project.
	now := "2026-01-01T00:00:00Z"
	db.ExecContext(context.Background(),
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('proj-plan', 'Plan Test', ?, ?)", now, now)

	fragStore := fragments.NewStore(db)
	snapCreator := artifacts.NewSnapshotCreator(db)
	comp := composer.New(db)

	t.Cleanup(func() { db.Close() })
	return db, fragStore, snapCreator, comp
}

func TestPlanPipeline_Stage10_DecomposesPlanFragments(t *testing.T) {
	_, fragStore, snapCreator, _ := setupPlanPipelineDB(t)
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	artStore := artifacts.NewStore(t.TempDir(), logger)
	handler := stages.NewStage3Handler(nil, fragStore, snapCreator, artStore, logger)

	// Simulate a plan submission (same decomposition flow as PRD).
	content := []byte("## Architecture\nLayered design.\n\n## Implementation\nStep by step.\n")
	result, err := handler.HandleSubmitDocument(ctx, "proj-plan", "gpt-4o", "run-10", content)
	if err != nil {
		t.Fatalf("HandleSubmitDocument: %v", err)
	}

	if result.ArtifactID == "" {
		t.Error("expected artifact ID")
	}
	if result.FragmentCount < 2 {
		t.Errorf("expected at least 2 fragments, got %d", result.FragmentCount)
	}
}

func TestPlanPipeline_CommitFragmentOperations(t *testing.T) {
	db, fragStore, _, _ := setupPlanPipelineDB(t)
	ctx := context.Background()

	// Create a fragment and version for the plan.
	frag, _ := fragStore.CreateFragment(ctx, "proj-plan", "plan", "Design", 2)
	ver, _ := fragStore.CreateVersion(ctx, frag.ID, "Original design content", "stage-10", "run-10", "initial")

	// Create an artifact with this fragment.
	snapCreator := artifacts.NewSnapshotCreator(db)
	snap, _ := snapCreator.CreateSnapshot(ctx, artifacts.SnapshotInput{
		ProjectID:          "proj-plan",
		ArtifactType:       "plan",
		SourceStage:        "stage-10",
		FragmentVersionIDs: []string{ver.ID},
		IsCanonical:        true,
	})

	// Commit an update operation.
	ops := []workflow.FragmentOperation{{
		Type:       "update",
		FragmentID: frag.ID,
		NewContent: "Updated design content with improvements",
		Rationale:  "Improved clarity",
	}}

	result, err := workflow.CommitFragmentOperations(ctx, db, "proj-plan", snap.ArtifactID, ops, "stage-14", "run-14", "plan")
	if err != nil {
		t.Fatalf("CommitFragmentOperations: %v", err)
	}

	if result.UpdateCount != 1 {
		t.Errorf("expected 1 update, got %d", result.UpdateCount)
	}
	if result.NoChanges {
		t.Error("expected changes, got no_changes")
	}
}

func TestPlanPipeline_ZeroOperations_NoChanges(t *testing.T) {
	db, fragStore, _, _ := setupPlanPipelineDB(t)
	ctx := context.Background()

	// Create a simple plan artifact.
	frag, _ := fragStore.CreateFragment(ctx, "proj-plan", "plan", "Section", 2)
	ver, _ := fragStore.CreateVersion(ctx, frag.ID, "Content", "s10", "r10", "")

	snapCreator := artifacts.NewSnapshotCreator(db)
	snap, _ := snapCreator.CreateSnapshot(ctx, artifacts.SnapshotInput{
		ProjectID:          "proj-plan",
		ArtifactType:       "plan",
		SourceStage:        "stage-14",
		FragmentVersionIDs: []string{ver.ID},
		IsCanonical:        true,
	})

	// Commit zero operations — should be no_changes.
	result, err := workflow.CommitFragmentOperations(ctx, db, "proj-plan", snap.ArtifactID, nil, "stage-15", "run-15", "plan")
	if err != nil {
		t.Fatalf("CommitFragmentOperations: %v", err)
	}

	if !result.NoChanges {
		t.Error("expected no_changes for zero operations")
	}
}
