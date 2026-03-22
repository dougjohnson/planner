package lineage

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

func setupTestDB(t *testing.T) *sql.DB {
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
	// Create artifacts for FK constraints.
	for _, id := range []string{"a-seed", "a-gpt", "a-opus", "a-synth", "a-integrated"} {
		db.ExecContext(ctx, "INSERT INTO artifacts (id, project_id, artifact_type, created_at) VALUES (?, 'p-1', 'prd', ?)", id, now)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestRecord(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	ctx := context.Background()

	err := svc.Record(ctx, "a-gpt", "a-seed", DecomposedFrom)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	rels, err := svc.GetRelations(ctx, "a-gpt")
	if err != nil {
		t.Fatalf("GetRelations: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rels))
	}
	if rels[0].RelationType != DecomposedFrom {
		t.Errorf("expected %q, got %q", DecomposedFrom, rels[0].RelationType)
	}
}

func TestRecordMultiple(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	ctx := context.Background()

	err := svc.RecordMultiple(ctx, "a-synth", []string{"a-gpt", "a-opus"}, SynthesizedFrom)
	if err != nil {
		t.Fatalf("RecordMultiple: %v", err)
	}

	sources, err := svc.GetSources(ctx, "a-synth")
	if err != nil {
		t.Fatalf("GetSources: %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}
}

func TestGetDerived(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	ctx := context.Background()

	svc.Record(ctx, "a-gpt", "a-seed", DecomposedFrom)
	svc.Record(ctx, "a-opus", "a-seed", DecomposedFrom)

	derived, err := svc.GetDerived(ctx, "a-seed")
	if err != nil {
		t.Fatalf("GetDerived: %v", err)
	}
	if len(derived) != 2 {
		t.Errorf("expected 2 derived, got %d", len(derived))
	}
}

func TestTraceLineage(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	ctx := context.Background()

	// Build a lineage chain: seed → gpt → synth → integrated
	svc.Record(ctx, "a-gpt", "a-seed", DecomposedFrom)
	svc.Record(ctx, "a-synth", "a-gpt", SynthesizedFrom)
	svc.Record(ctx, "a-integrated", "a-synth", IntegratedFrom)

	// Trace from integrated back to root.
	lineage, err := svc.TraceLineage(ctx, "a-integrated")
	if err != nil {
		t.Fatalf("TraceLineage: %v", err)
	}
	if len(lineage) != 3 {
		t.Errorf("expected 3 lineage entries, got %d", len(lineage))
	}

	// Verify we reach the seed.
	reachedSeed := false
	for _, r := range lineage {
		if r.RelatedArtifactID == "a-seed" {
			reachedSeed = true
		}
	}
	if !reachedSeed {
		t.Error("lineage trace did not reach seed artifact")
	}
}

func TestTraceLineage_NoAncestors(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	ctx := context.Background()

	lineage, err := svc.TraceLineage(ctx, "a-seed")
	if err != nil {
		t.Fatalf("TraceLineage: %v", err)
	}
	if len(lineage) != 0 {
		t.Errorf("expected 0 lineage entries for root, got %d", len(lineage))
	}
}

func TestRelationTypeConstants(t *testing.T) {
	types := []RelationType{
		SynthesizedFrom, IntegratedFrom, ResolvedFrom, RevisedFrom,
		RollbackOf, DiffTarget, ExportIncludes, DecomposedFrom,
	}
	if len(types) != 8 {
		t.Errorf("expected 8 relation types, got %d", len(types))
	}
	for _, rt := range types {
		if rt == "" {
			t.Error("empty relation type constant")
		}
	}
}
