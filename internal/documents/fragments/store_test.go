package fragments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

func seedProject(t testing.TB, tdb *testutil.TestDB) string {
	t.Helper()
	projectID := "proj-frag-test"
	now := time.Now().UTC().Format(time.RFC3339)
	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES (?, 'Test', 'active', ?, ?)",
		projectID, now, now)
	return projectID
}

func TestCreateFragment(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)

	f, err := store.CreateFragment(context.Background(), projectID, "prd", "Overview", 2)
	if err != nil {
		t.Fatalf("CreateFragment: %v", err)
	}

	if f.ID == "" {
		t.Error("expected non-empty ID")
	}
	if f.ProjectID != projectID {
		t.Errorf("expected project_id %s, got %s", projectID, f.ProjectID)
	}
	if f.DocumentType != "prd" {
		t.Errorf("expected document_type 'prd', got %s", f.DocumentType)
	}
	if f.Heading != "Overview" {
		t.Errorf("expected heading 'Overview', got %s", f.Heading)
	}
	if f.Depth != 2 {
		t.Errorf("expected depth 2, got %d", f.Depth)
	}
}

func TestCreateFragment_InvalidDocumentType(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)

	_, err := store.CreateFragment(context.Background(), projectID, "invalid", "Heading", 2)
	if err == nil {
		t.Fatal("expected error for invalid document_type")
	}
}

func TestCreateFragment_EmptyInputs(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)

	_, err := store.CreateFragment(context.Background(), "", "prd", "Heading", 2)
	if err == nil {
		t.Fatal("expected error for empty project_id")
	}

	projectID := seedProject(t, tdb)
	_, err = store.CreateFragment(context.Background(), projectID, "prd", "", 2)
	if err == nil {
		t.Fatal("expected error for empty heading")
	}
}

func TestGetFragment(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)

	created, _ := store.CreateFragment(context.Background(), projectID, "prd", "Overview", 2)

	got, err := store.GetFragment(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetFragment: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, got.ID)
	}
	if got.Heading != "Overview" {
		t.Errorf("expected heading 'Overview', got %s", got.Heading)
	}
}

func TestGetFragment_NotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)

	_, err := store.GetFragment(context.Background(), "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFindByHeading(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)

	store.CreateFragment(context.Background(), projectID, "prd", "Technical Requirements", 2)

	found, err := store.FindByHeading(context.Background(), projectID, "prd", "Technical Requirements")
	if err != nil {
		t.Fatalf("FindByHeading: %v", err)
	}
	if found.Heading != "Technical Requirements" {
		t.Errorf("expected 'Technical Requirements', got %s", found.Heading)
	}
}

func TestFindByHeading_NotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)

	_, err := store.FindByHeading(context.Background(), projectID, "prd", "Nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListByProject(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	store.CreateFragment(ctx, projectID, "prd", "Overview", 2)
	store.CreateFragment(ctx, projectID, "prd", "Requirements", 2)
	store.CreateFragment(ctx, projectID, "plan", "Architecture", 2) // different doc type

	prdFragments, err := store.ListByProject(ctx, projectID, "prd")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(prdFragments) != 2 {
		t.Errorf("expected 2 PRD fragments, got %d", len(prdFragments))
	}

	planFragments, err := store.ListByProject(ctx, projectID, "plan")
	if err != nil {
		t.Fatalf("ListByProject plan: %v", err)
	}
	if len(planFragments) != 1 {
		t.Errorf("expected 1 plan fragment, got %d", len(planFragments))
	}
}

func TestCreateVersion(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	frag, _ := store.CreateFragment(ctx, projectID, "prd", "Overview", 2)

	v, err := store.CreateVersion(ctx, frag.ID, "This is the overview content.", "stage-3", "run-001", "Initial generation")
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	if v.ID == "" {
		t.Error("expected non-empty version ID")
	}
	if v.FragmentID != frag.ID {
		t.Errorf("expected fragment_id %s, got %s", frag.ID, v.FragmentID)
	}
	if v.Content != "This is the overview content." {
		t.Errorf("content mismatch")
	}
	if v.Checksum == "" {
		t.Error("expected non-empty checksum")
	}
	if v.SourceStage != "stage-3" {
		t.Errorf("expected source_stage 'stage-3', got %s", v.SourceStage)
	}
}

func TestCreateVersion_EmptyInputs(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	ctx := context.Background()

	_, err := store.CreateVersion(ctx, "", "content", "", "", "")
	if err == nil {
		t.Fatal("expected error for empty fragment_id")
	}

	_, err = store.CreateVersion(ctx, "frag-001", "", "", "", "")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestGetVersion(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	frag, _ := store.CreateFragment(ctx, projectID, "prd", "Overview", 2)
	created, _ := store.CreateVersion(ctx, frag.ID, "Content here", "stage-3", "run-001", "")

	got, err := store.GetVersion(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got.Content != "Content here" {
		t.Errorf("content mismatch")
	}
}

func TestGetVersion_NotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)

	_, err := store.GetVersion(context.Background(), "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListVersions(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	frag, _ := store.CreateFragment(ctx, projectID, "prd", "Overview", 2)

	store.CreateVersion(ctx, frag.ID, "Version 1", "stage-3", "run-001", "Initial")
	store.CreateVersion(ctx, frag.ID, "Version 2", "stage-7", "run-002", "Review pass")
	store.CreateVersion(ctx, frag.ID, "Version 3", "stage-7", "run-003", "Second review")

	versions, err := store.ListVersions(ctx, frag.ID)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}

	// Should be ordered by created_at ASC.
	if versions[0].Content != "Version 1" {
		t.Errorf("expected first version content 'Version 1', got %s", versions[0].Content)
	}
	if versions[2].Content != "Version 3" {
		t.Errorf("expected last version content 'Version 3', got %s", versions[2].Content)
	}
}

func TestLatestVersion(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	frag, _ := store.CreateFragment(ctx, projectID, "prd", "Overview", 2)

	store.CreateVersion(ctx, frag.ID, "Version 1", "stage-3", "run-001", "")
	store.CreateVersion(ctx, frag.ID, "Version 2 — latest", "stage-7", "run-002", "")

	latest, err := store.LatestVersion(ctx, frag.ID)
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if latest.Content != "Version 2 — latest" {
		t.Errorf("expected latest content, got %s", latest.Content)
	}
}

func TestLatestVersion_NoVersions(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	frag, _ := store.CreateFragment(ctx, projectID, "prd", "Overview", 2)

	_, err := store.LatestVersion(ctx, frag.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFindVersionByChecksum(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	frag, _ := store.CreateFragment(ctx, projectID, "prd", "Overview", 2)

	content := "This is dedup-testable content."
	v, _ := store.CreateVersion(ctx, frag.ID, content, "stage-3", "run-001", "")

	checksum := ComputeChecksum(content)
	found, err := store.FindVersionByChecksum(ctx, frag.ID, checksum)
	if err != nil {
		t.Fatalf("FindVersionByChecksum: %v", err)
	}
	if found.ID != v.ID {
		t.Errorf("expected version %s, got %s", v.ID, found.ID)
	}
}

func TestFindVersionByChecksum_NotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := NewStore(tdb.DB)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	frag, _ := store.CreateFragment(ctx, projectID, "prd", "Overview", 2)

	_, err := store.FindVersionByChecksum(ctx, frag.ID, "nonexistent-checksum")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestComputeChecksum_Deterministic(t *testing.T) {
	content := "Hello, world!"
	c1 := ComputeChecksum(content)
	c2 := ComputeChecksum(content)
	if c1 != c2 {
		t.Errorf("checksums should be deterministic: %s != %s", c1, c2)
	}
	if len(c1) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("expected 64-char hex checksum, got %d chars", len(c1))
	}
}

func TestComputeChecksum_DifferentContent(t *testing.T) {
	c1 := ComputeChecksum("content A")
	c2 := ComputeChecksum("content B")
	if c1 == c2 {
		t.Error("different content should produce different checksums")
	}
}
