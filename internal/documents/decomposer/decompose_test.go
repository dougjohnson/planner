package decomposer

import (
	"context"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

func seedProject(t testing.TB, tdb *testutil.TestDB) string {
	t.Helper()
	projectID := "proj-decompose"
	now := time.Now().UTC().Format(time.RFC3339)
	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES (?, 'Test', 'active', ?, ?)",
		projectID, now, now)
	return projectID
}

func TestDecompose_FirstDocument(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := fragments.NewStore(tdb.DB)
	dec := New(store)
	projectID := seedProject(t, tdb)

	doc := []byte("# My PRD\n\nIntro text.\n\n## Overview\n\nOverview content.\n\n## Requirements\n\nReq content.\n\n## Non-Functional\n\nNF content.\n")

	result, err := dec.Decompose(context.Background(), projectID, "prd", doc, "stage-3", "run-001")
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	// Should have preamble + 3 sections = 4 fragments.
	if len(result.Fragments) != 4 {
		t.Errorf("expected 4 fragments, got %d", len(result.Fragments))
	}
	if result.NewFragments != 4 {
		t.Errorf("expected 4 new fragments, got %d", result.NewFragments)
	}
	if result.NewVersions != 4 {
		t.Errorf("expected 4 new versions, got %d", result.NewVersions)
	}
	if result.ReusedVersions != 0 {
		t.Errorf("expected 0 reused versions, got %d", result.ReusedVersions)
	}

	// First fragment should be preamble.
	if result.Fragments[0].Heading != "" {
		t.Errorf("first fragment should be preamble (empty heading), got %q", result.Fragments[0].Heading)
	}
	if result.Fragments[1].Heading != "Overview" {
		t.Errorf("second fragment heading: expected 'Overview', got %q", result.Fragments[1].Heading)
	}
}

func TestDecompose_SecondSubmission_IdenticalContent(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := fragments.NewStore(tdb.DB)
	dec := New(store)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	doc := []byte("## Overview\n\nSame content.\n\n## Details\n\nSame details.\n")

	// First decomposition.
	r1, _ := dec.Decompose(ctx, projectID, "prd", doc, "stage-3", "run-001")

	// Second decomposition with identical content.
	r2, err := dec.Decompose(ctx, projectID, "prd", doc, "stage-7", "run-002")
	if err != nil {
		t.Fatalf("second Decompose: %v", err)
	}

	// Should match existing fragments and reuse versions.
	if r2.NewFragments != 0 {
		t.Errorf("expected 0 new fragments on identical resubmission, got %d", r2.NewFragments)
	}
	if r2.ReusedVersions != 2 {
		t.Errorf("expected 2 reused versions, got %d", r2.ReusedVersions)
	}

	// Fragment IDs should be stable.
	if r1.Fragments[0].ID != r2.Fragments[0].ID {
		t.Error("fragment IDs should be stable across decompositions")
	}
}

func TestDecompose_ContentChanged(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := fragments.NewStore(tdb.DB)
	dec := New(store)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	doc1 := []byte("## Overview\n\nOriginal content.\n")
	dec.Decompose(ctx, projectID, "prd", doc1, "stage-3", "run-001")

	doc2 := []byte("## Overview\n\nUpdated content.\n")
	r2, err := dec.Decompose(ctx, projectID, "prd", doc2, "stage-7", "run-002")
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if r2.NewFragments != 0 {
		t.Errorf("expected 0 new fragments (heading matched), got %d", r2.NewFragments)
	}
	if r2.NewVersions != 1 {
		t.Errorf("expected 1 new version (content changed), got %d", r2.NewVersions)
	}
}

func TestDecompose_NewSection(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := fragments.NewStore(tdb.DB)
	dec := New(store)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	doc1 := []byte("## Overview\n\nContent.\n")
	dec.Decompose(ctx, projectID, "prd", doc1, "stage-3", "run-001")

	doc2 := []byte("## Overview\n\nContent.\n\n## New Section\n\nNew content.\n")
	r2, err := dec.Decompose(ctx, projectID, "prd", doc2, "stage-7", "run-002")
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if r2.NewFragments != 1 {
		t.Errorf("expected 1 new fragment, got %d", r2.NewFragments)
	}
	if len(r2.Fragments) != 2 {
		t.Errorf("expected 2 total fragments, got %d", len(r2.Fragments))
	}
}

func TestDecompose_ExcludedFragment(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := fragments.NewStore(tdb.DB)
	dec := New(store)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	doc1 := []byte("## Overview\n\nContent.\n\n## To Remove\n\nWill be removed.\n")
	dec.Decompose(ctx, projectID, "prd", doc1, "stage-3", "run-001")

	doc2 := []byte("## Overview\n\nContent.\n")
	r2, err := dec.Decompose(ctx, projectID, "prd", doc2, "stage-7", "run-002")
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if len(r2.ExcludedFragmentIDs) != 1 {
		t.Errorf("expected 1 excluded fragment, got %d", len(r2.ExcludedFragmentIDs))
	}
}

func TestDecompose_DuplicateHeadings(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := fragments.NewStore(tdb.DB)
	dec := New(store)
	projectID := seedProject(t, tdb)
	ctx := context.Background()

	doc := []byte("## Section\n\nFirst section.\n\n## Section\n\nSecond section.\n")
	result, err := dec.Decompose(ctx, projectID, "prd", doc, "stage-3", "run-001")
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if len(result.Fragments) != 2 {
		t.Fatalf("expected 2 fragments for duplicate headings, got %d", len(result.Fragments))
	}
	// Both should have heading "Section" but different IDs.
	if result.Fragments[0].ID == result.Fragments[1].ID {
		t.Error("duplicate heading fragments should have different IDs")
	}
}

func TestDecompose_NoPreamble(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	store := fragments.NewStore(tdb.DB)
	dec := New(store)
	projectID := seedProject(t, tdb)

	doc := []byte("## Overview\n\nContent starts at heading.\n")
	result, _ := dec.Decompose(context.Background(), projectID, "prd", doc, "stage-3", "run-001")

	if len(result.Fragments) != 1 {
		t.Errorf("expected 1 fragment (no preamble), got %d", len(result.Fragments))
	}
	if result.Fragments[0].Heading != "Overview" {
		t.Errorf("expected heading 'Overview', got %q", result.Fragments[0].Heading)
	}
}
