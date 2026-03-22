package artifacts

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/db/migrations"
	_ "modernc.org/sqlite"
)

func setupSnapshotDB(t *testing.T) *sql.DB {
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

	ctx := context.Background()
	now := "2026-01-01T00:00:00Z"
	db.ExecContext(ctx, "INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)
	db.ExecContext(ctx, "INSERT INTO fragments (id, project_id, document_type, heading, created_at) VALUES ('f-1', 'p-1', 'prd', 'Intro', ?)", now)
	db.ExecContext(ctx, "INSERT INTO fragments (id, project_id, document_type, heading, created_at) VALUES ('f-2', 'p-1', 'prd', 'Body', ?)", now)
	db.ExecContext(ctx, "INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-1', 'f-1', 'intro text', 'ck1', ?)", now)
	db.ExecContext(ctx, "INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-2', 'f-2', 'body text', 'ck2', ?)", now)

	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateSnapshot_Basic(t *testing.T) {
	db := setupSnapshotDB(t)
	sc := NewSnapshotCreator(db)
	ctx := context.Background()

	result, err := sc.CreateSnapshot(ctx, SnapshotInput{
		ProjectID:          "p-1",
		ArtifactType:       "prd",
		SourceStage:        "prd_intake",
		SourceModel:        "gpt-4o",
		VersionSuffix:      ".seed",
		FragmentVersionIDs: []string{"fv-1", "fv-2"},
	})
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	if result.ArtifactID == "" {
		t.Error("expected non-empty artifact ID")
	}
	if result.VersionLabel != "prd.v01.seed" {
		t.Errorf("expected 'prd.v01.seed', got %q", result.VersionLabel)
	}

	// Verify junction table.
	var count int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM artifact_fragments WHERE artifact_id = ?", result.ArtifactID).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 junction rows, got %d", count)
	}
}

func TestCreateSnapshot_AutoIncrementVersion(t *testing.T) {
	db := setupSnapshotDB(t)
	sc := NewSnapshotCreator(db)
	ctx := context.Background()

	r1, _ := sc.CreateSnapshot(ctx, SnapshotInput{
		ProjectID:          "p-1",
		ArtifactType:       "prd",
		FragmentVersionIDs: []string{"fv-1"},
	})
	r2, _ := sc.CreateSnapshot(ctx, SnapshotInput{
		ProjectID:          "p-1",
		ArtifactType:       "prd",
		FragmentVersionIDs: []string{"fv-2"},
	})

	if r1.VersionLabel != "prd.v01" {
		t.Errorf("first: expected 'prd.v01', got %q", r1.VersionLabel)
	}
	if r2.VersionLabel != "prd.v02" {
		t.Errorf("second: expected 'prd.v02', got %q", r2.VersionLabel)
	}
}

func TestCreateSnapshot_WithLineage(t *testing.T) {
	db := setupSnapshotDB(t)
	sc := NewSnapshotCreator(db)
	ctx := context.Background()

	// Create source artifact.
	source, _ := sc.CreateSnapshot(ctx, SnapshotInput{
		ProjectID:          "p-1",
		ArtifactType:       "prd",
		VersionSuffix:      ".seed",
		FragmentVersionIDs: []string{"fv-1"},
	})

	// Create derived artifact with lineage.
	derived, err := sc.CreateSnapshot(ctx, SnapshotInput{
		ProjectID:          "p-1",
		ArtifactType:       "prd",
		VersionSuffix:      ".synthesized",
		FragmentVersionIDs: []string{"fv-1", "fv-2"},
		SourceArtifactIDs:  []string{source.ArtifactID},
		RelationType:       "synthesized_from",
	})
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	// Verify lineage relation.
	var relCount int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM artifact_relations WHERE artifact_id = ?", derived.ArtifactID).Scan(&relCount)
	if relCount != 1 {
		t.Errorf("expected 1 lineage relation, got %d", relCount)
	}

	var relatedID, relType string
	db.QueryRowContext(ctx,
		"SELECT related_artifact_id, relation_type FROM artifact_relations WHERE artifact_id = ?",
		derived.ArtifactID).Scan(&relatedID, &relType)
	if relatedID != source.ArtifactID {
		t.Errorf("expected related to %s, got %s", source.ArtifactID, relatedID)
	}
	if relType != "synthesized_from" {
		t.Errorf("expected relation 'synthesized_from', got %q", relType)
	}
}

func TestCreateSnapshot_NoFragments(t *testing.T) {
	db := setupSnapshotDB(t)
	sc := NewSnapshotCreator(db)

	_, err := sc.CreateSnapshot(context.Background(), SnapshotInput{
		ProjectID:    "p-1",
		ArtifactType: "prd",
	})
	if err == nil {
		t.Error("expected error for empty fragment version IDs")
	}
}

func TestCreateSnapshot_PositionOrdering(t *testing.T) {
	db := setupSnapshotDB(t)
	sc := NewSnapshotCreator(db)
	ctx := context.Background()

	result, _ := sc.CreateSnapshot(ctx, SnapshotInput{
		ProjectID:          "p-1",
		ArtifactType:       "prd",
		FragmentVersionIDs: []string{"fv-2", "fv-1"}, // reversed order
	})

	// Verify positions are correct.
	rows, _ := db.QueryContext(ctx,
		"SELECT fragment_version_id, position FROM artifact_fragments WHERE artifact_id = ? ORDER BY position",
		result.ArtifactID)
	defer rows.Close()

	var positions []struct {
		fvID string
		pos  int
	}
	for rows.Next() {
		var p struct {
			fvID string
			pos  int
		}
		rows.Scan(&p.fvID, &p.pos)
		positions = append(positions, p)
	}

	if len(positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(positions))
	}
	if positions[0].fvID != "fv-2" || positions[0].pos != 0 {
		t.Errorf("position 0: expected fv-2, got %s at pos %d", positions[0].fvID, positions[0].pos)
	}
	if positions[1].fvID != "fv-1" || positions[1].pos != 1 {
		t.Errorf("position 1: expected fv-1, got %s at pos %d", positions[1].fvID, positions[1].pos)
	}
}
