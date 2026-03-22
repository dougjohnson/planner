package composer

import (
	"context"
	"database/sql"
	"testing"
)

func seedDiffArtifacts(t *testing.T, db *sql.DB) (string, string) {
	t.Helper()
	ctx := context.Background()
	now := "2026-01-01T00:00:00Z"

	db.ExecContext(ctx, "INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)

	// Shared fragment (unchanged between artifacts).
	db.ExecContext(ctx, "INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at) VALUES ('f-shared', 'p-1', 'prd', 'Shared', 2, ?)", now)
	db.ExecContext(ctx, "INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-shared', 'f-shared', 'Same content', 'ck1', ?)", now)

	// Fragment modified between artifacts.
	db.ExecContext(ctx, "INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at) VALUES ('f-mod', 'p-1', 'prd', 'Modified', 2, ?)", now)
	db.ExecContext(ctx, "INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-mod-v1', 'f-mod', 'Old text', 'ck2', ?)", now)
	db.ExecContext(ctx, "INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-mod-v2', 'f-mod', 'New text with changes', 'ck3', ?)", now)

	// Fragment only in A (removed).
	db.ExecContext(ctx, "INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at) VALUES ('f-removed', 'p-1', 'prd', 'Removed', 2, ?)", now)
	db.ExecContext(ctx, "INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-removed', 'f-removed', 'Gone', 'ck4', ?)", now)

	// Fragment only in B (added).
	db.ExecContext(ctx, "INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at) VALUES ('f-added', 'p-1', 'prd', 'Added', 2, ?)", now)
	db.ExecContext(ctx, "INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-added', 'f-added', 'Brand new', 'ck5', ?)", now)

	// Artifact A: shared + modified(v1) + removed.
	db.ExecContext(ctx, "INSERT INTO artifacts (id, project_id, artifact_type, created_at) VALUES ('a-1', 'p-1', 'prd', ?)", now)
	db.ExecContext(ctx, "INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('a-1', 'fv-shared', 0)")
	db.ExecContext(ctx, "INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('a-1', 'fv-mod-v1', 1)")
	db.ExecContext(ctx, "INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('a-1', 'fv-removed', 2)")

	// Artifact B: shared + modified(v2) + added.
	db.ExecContext(ctx, "INSERT INTO artifacts (id, project_id, artifact_type, created_at) VALUES ('a-2', 'p-1', 'prd', ?)", now)
	db.ExecContext(ctx, "INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('a-2', 'fv-shared', 0)")
	db.ExecContext(ctx, "INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('a-2', 'fv-mod-v2', 1)")
	db.ExecContext(ctx, "INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('a-2', 'fv-added', 2)")

	return "a-1", "a-2"
}

func TestFragmentDiff_MixedChanges(t *testing.T) {
	db := setupTestDB(t)
	a, b := seedDiffArtifacts(t, db)

	engine := NewDiffEngine(db)
	result, err := engine.FragmentDiff(context.Background(), a, b)
	if err != nil {
		t.Fatalf("FragmentDiff: %v", err)
	}

	if result.Summary.Unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", result.Summary.Unchanged)
	}
	if result.Summary.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", result.Summary.Modified)
	}
	if result.Summary.Removed != 1 {
		t.Errorf("expected 1 removed, got %d", result.Summary.Removed)
	}
	if result.Summary.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Summary.Added)
	}
}

func TestFragmentDiff_IdenticalArtifacts(t *testing.T) {
	db := setupTestDB(t)
	a, _ := seedDiffArtifacts(t, db)

	engine := NewDiffEngine(db)
	result, err := engine.FragmentDiff(context.Background(), a, a)
	if err != nil {
		t.Fatalf("FragmentDiff: %v", err)
	}

	if result.Summary.Added != 0 || result.Summary.Removed != 0 || result.Summary.Modified != 0 {
		t.Errorf("expected no changes, got %+v", result.Summary)
	}
	if result.Summary.Unchanged != 3 {
		t.Errorf("expected 3 unchanged, got %d", result.Summary.Unchanged)
	}
}

func TestFragmentDiff_ModifiedHasLineDiff(t *testing.T) {
	db := setupTestDB(t)
	a, b := seedDiffArtifacts(t, db)

	engine := NewDiffEngine(db)
	result, _ := engine.FragmentDiff(context.Background(), a, b)

	for _, entry := range result.Entries {
		if entry.Status == "modified" {
			if entry.LineDiff == "" {
				t.Error("modified entry should have line diff")
			}
			if entry.OldContent == "" || entry.NewContent == "" {
				t.Error("modified entry should have old and new content")
			}
		}
	}
}

func TestComposedDiff(t *testing.T) {
	db := setupTestDB(t)
	a, b := seedDiffArtifacts(t, db)

	engine := NewDiffEngine(db)
	result, err := engine.ComposedDiff(context.Background(), a, b)
	if err != nil {
		t.Fatalf("ComposedDiff: %v", err)
	}

	if result.UnifiedDiff == "" {
		t.Error("expected non-empty unified diff")
	}
}

func TestComposedDiff_Identical(t *testing.T) {
	db := setupTestDB(t)
	a, _ := seedDiffArtifacts(t, db)

	engine := NewDiffEngine(db)
	result, err := engine.ComposedDiff(context.Background(), a, a)
	if err != nil {
		t.Fatalf("ComposedDiff: %v", err)
	}

	if result.UnifiedDiff != "" {
		t.Errorf("expected empty diff for identical artifacts, got %q", result.UnifiedDiff)
	}
}
