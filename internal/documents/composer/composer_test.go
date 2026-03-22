package composer

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

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
		t.Fatalf("opening test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("setting WAL: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("setting FK: %v", err)
	}

	ctx := context.Background()
	if err := migrations.Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

// seedDocument creates a project, fragments, versions, artifact, and junction rows.
// Returns the artifact ID.
func seedDocument(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	now := "2026-01-01T00:00:00Z"

	// Project.
	_, err := db.ExecContext(ctx,
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)
	if err != nil {
		t.Fatalf("inserting project: %v", err)
	}

	// Fragments.
	frags := []struct {
		id, heading string
		depth       int
	}{
		{"f-1", "Introduction", 2},
		{"f-2", "Architecture", 2},
		{"f-3", "Conclusion", 2},
	}
	for _, f := range frags {
		_, err := db.ExecContext(ctx,
			"INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at) VALUES (?, 'p-1', 'prd', ?, ?, ?)",
			f.id, f.heading, f.depth, now)
		if err != nil {
			t.Fatalf("inserting fragment %s: %v", f.id, err)
		}
	}

	// Fragment versions.
	versions := []struct {
		id, fragmentID, content string
	}{
		{"fv-1", "f-1", "This is the introduction."},
		{"fv-2", "f-2", "The system uses a layered architecture.\n\nKey components include the API, service, and data layers."},
		{"fv-3", "f-3", "In summary, the design is solid."},
	}
	for _, v := range versions {
		_, err := db.ExecContext(ctx,
			"INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES (?, ?, ?, 'cksum', ?)",
			v.id, v.fragmentID, v.content, now)
		if err != nil {
			t.Fatalf("inserting version %s: %v", v.id, err)
		}
	}

	// Artifact.
	_, err = db.ExecContext(ctx,
		"INSERT INTO artifacts (id, project_id, artifact_type, created_at) VALUES ('a-1', 'p-1', 'prd', ?)", now)
	if err != nil {
		t.Fatalf("inserting artifact: %v", err)
	}

	// Junction: artifact_fragments.
	junctions := []struct {
		artifactID, versionID string
		position              int
	}{
		{"a-1", "fv-1", 0},
		{"a-1", "fv-2", 1},
		{"a-1", "fv-3", 2},
	}
	for _, j := range junctions {
		_, err := db.ExecContext(ctx,
			"INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES (?, ?, ?)",
			j.artifactID, j.versionID, j.position)
		if err != nil {
			t.Fatalf("inserting junction: %v", err)
		}
	}

	return "a-1"
}

func TestCompose_BasicDocument(t *testing.T) {
	db := setupTestDB(t)
	artifactID := seedDocument(t, db)

	c := New(db)
	result, err := c.Compose(context.Background(), artifactID)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	// Verify all sections are present.
	if !strings.Contains(result, "## Introduction") {
		t.Error("missing Introduction heading")
	}
	if !strings.Contains(result, "This is the introduction.") {
		t.Error("missing Introduction content")
	}
	if !strings.Contains(result, "## Architecture") {
		t.Error("missing Architecture heading")
	}
	if !strings.Contains(result, "layered architecture") {
		t.Error("missing Architecture content")
	}
	if !strings.Contains(result, "## Conclusion") {
		t.Error("missing Conclusion heading")
	}
}

func TestCompose_EmptyArtifact(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := "2026-01-01T00:00:00Z"

	// Create project and empty artifact.
	db.ExecContext(ctx, "INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-2', 'Empty', ?, ?)", now, now)
	db.ExecContext(ctx, "INSERT INTO artifacts (id, project_id, artifact_type, created_at) VALUES ('a-empty', 'p-2', 'plan', ?)", now)

	c := New(db)
	result, err := c.Compose(ctx, "a-empty")
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for empty artifact, got %q", result)
	}
}

func TestCompose_PositionOrdering(t *testing.T) {
	db := setupTestDB(t)
	artifactID := seedDocument(t, db)

	c := New(db)
	result, err := c.Compose(context.Background(), artifactID)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	// Verify sections appear in position order.
	introIdx := strings.Index(result, "Introduction")
	archIdx := strings.Index(result, "Architecture")
	concIdx := strings.Index(result, "Conclusion")

	if introIdx >= archIdx || archIdx >= concIdx {
		t.Errorf("sections not in position order: intro=%d, arch=%d, conc=%d", introIdx, archIdx, concIdx)
	}
}

func TestComposeWithAnnotations(t *testing.T) {
	db := setupTestDB(t)
	artifactID := seedDocument(t, db)

	c := New(db)
	result, err := c.ComposeWithAnnotations(context.Background(), artifactID)
	if err != nil {
		t.Fatalf("ComposeWithAnnotations: %v", err)
	}

	// Should contain fragment ID annotations.
	if !strings.Contains(result, "<!-- fragment:f-1 version:fv-1 -->") {
		t.Error("missing annotation for fragment f-1")
	}
	if !strings.Contains(result, "<!-- fragment:f-2 version:fv-2 -->") {
		t.Error("missing annotation for fragment f-2")
	}

	// Should also contain regular content.
	if !strings.Contains(result, "## Introduction") {
		t.Error("missing heading in annotated output")
	}
}

func TestCompose_NonexistentArtifact(t *testing.T) {
	db := setupTestDB(t)

	c := New(db)
	result, err := c.Compose(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error for nonexistent artifact, got: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}
