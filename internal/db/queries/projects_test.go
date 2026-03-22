package queries

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
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

func TestProjectRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	project, err := repo.Create(ctx, "Test Project", "A test description")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if project.ID == "" {
		t.Error("expected non-empty ID")
	}
	if project.Name != "Test Project" {
		t.Errorf("expected name 'Test Project', got %q", project.Name)
	}
	if project.Description != "A test description" {
		t.Errorf("expected description 'A test description', got %q", project.Description)
	}
	if project.Status != "active" {
		t.Errorf("expected status 'active', got %q", project.Status)
	}
	if project.ArchivedAt.Valid {
		t.Error("expected nil archived_at for new project")
	}
}

func TestProjectRepo_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	created, _ := repo.Create(ctx, "Find Me", "")
	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.Name != "Find Me" {
		t.Errorf("expected 'Find Me', got %q", found.Name)
	}
}

func TestProjectRepo_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestProjectRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	repo.Create(ctx, "Project A", "")
	repo.Create(ctx, "Project B", "")

	projects, err := repo.List(ctx, ProjectFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestProjectRepo_List_ExcludesArchived(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	p1, _ := repo.Create(ctx, "Active", "")
	p2, _ := repo.Create(ctx, "Archived", "")
	repo.Archive(ctx, p2.ID)

	// Default: exclude archived.
	projects, _ := repo.List(ctx, ProjectFilter{})
	if len(projects) != 1 {
		t.Errorf("expected 1 active project, got %d", len(projects))
	}
	if projects[0].ID != p1.ID {
		t.Errorf("expected active project %s, got %s", p1.ID, projects[0].ID)
	}

	// Include archived.
	all, _ := repo.List(ctx, ProjectFilter{IncludeArchived: true})
	if len(all) != 2 {
		t.Errorf("expected 2 total projects, got %d", len(all))
	}
}

func TestProjectRepo_List_Pagination(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	for i := range 5 {
		repo.Create(ctx, fmt.Sprintf("Project %d", i), "")
	}

	page, _ := repo.List(ctx, ProjectFilter{Limit: 2, Offset: 0})
	if len(page) != 2 {
		t.Errorf("expected 2 projects in page, got %d", len(page))
	}
}

func TestProjectRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	created, _ := repo.Create(ctx, "Original", "desc")
	updated, err := repo.Update(ctx, created.ID, map[string]string{
		"name":        "Updated",
		"description": "new desc",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", updated.Name)
	}
	if updated.Description != "new desc" {
		t.Errorf("expected description 'new desc', got %q", updated.Description)
	}
}

func TestProjectRepo_Update_RejectsInvalidField(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	created, _ := repo.Create(ctx, "Test", "")
	_, err := repo.Update(ctx, created.ID, map[string]string{
		"id": "evil-new-id",
	})
	if err == nil {
		t.Error("expected error for updating non-updatable field 'id'")
	}
}

func TestProjectRepo_Archive_And_Resume(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	p, _ := repo.Create(ctx, "Archive Me", "")

	// Archive.
	if err := repo.Archive(ctx, p.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	archived, _ := repo.GetByID(ctx, p.ID)
	if !archived.ArchivedAt.Valid {
		t.Error("expected archived_at to be set")
	}

	// Archive again should fail (already archived).
	if err := repo.Archive(ctx, p.ID); err == nil {
		t.Error("expected error when archiving already-archived project")
	}

	// Resume.
	if err := repo.Resume(ctx, p.ID); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	resumed, _ := repo.GetByID(ctx, p.ID)
	if resumed.ArchivedAt.Valid {
		t.Error("expected archived_at to be cleared after resume")
	}

	// Resume again should fail (not archived).
	if err := repo.Resume(ctx, p.ID); err == nil {
		t.Error("expected error when resuming non-archived project")
	}
}
