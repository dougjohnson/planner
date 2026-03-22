package tools

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

func TestUpdateFragment_Valid(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewFragmentOpsHandler(store)

	// Create a fragment to update.
	frag, err := store.CreateFragment(context.Background(), "proj_1", "prd", "Introduction", 2)
	if err != nil {
		t.Fatalf("CreateFragment: %v", err)
	}
	_, err = store.CreateVersion(context.Background(), frag.ID, "Original content", "stage_3", "run_1", "")
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	call := models.ToolCall{
		Name: "update_fragment",
		Arguments: map[string]any{
			"fragment_id": frag.ID,
			"new_content": "Updated content",
			"rationale":   "Improved clarity",
		},
	}

	op, err := h.HandleUpdateFragment(context.Background(), call, "stage_7", "run_2")
	if err != nil {
		t.Fatalf("HandleUpdateFragment: %v", err)
	}
	if op.Type != "update" {
		t.Errorf("Type = %q, want %q", op.Type, "update")
	}
	if op.FragmentID != frag.ID {
		t.Errorf("FragmentID = %q, want %q", op.FragmentID, frag.ID)
	}

	// Verify new version was created.
	latest, err := store.LatestVersion(context.Background(), frag.ID)
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if latest.Content != "Updated content" {
		t.Errorf("latest content = %q, want %q", latest.Content, "Updated content")
	}
}

func TestUpdateFragment_NotFound(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewFragmentOpsHandler(store)

	call := models.ToolCall{
		Name: "update_fragment",
		Arguments: map[string]any{
			"fragment_id": "nonexistent_id",
			"new_content": "content",
			"rationale":   "reason",
		},
	}

	_, err := h.HandleUpdateFragment(context.Background(), call, "stage_7", "run_1")
	if err == nil {
		t.Fatal("expected error for nonexistent fragment")
	}
}

func TestUpdateFragment_MissingArgs(t *testing.T) {
	h := NewFragmentOpsHandler(nil)

	// Missing fragment_id
	_, err := h.HandleUpdateFragment(context.Background(), models.ToolCall{
		Name:      "update_fragment",
		Arguments: map[string]any{"new_content": "x", "rationale": "y"},
	}, "s", "r")
	if err == nil {
		t.Fatal("expected error for missing fragment_id")
	}

	// Missing new_content
	_, err = h.HandleUpdateFragment(context.Background(), models.ToolCall{
		Name:      "update_fragment",
		Arguments: map[string]any{"fragment_id": "x", "rationale": "y"},
	}, "s", "r")
	if err == nil {
		t.Fatal("expected error for missing new_content")
	}
}

func TestAddFragment_Valid(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewFragmentOpsHandler(store)

	// Create an existing fragment to insert after.
	existing, err := store.CreateFragment(context.Background(), "proj_1", "prd", "Existing", 2)
	if err != nil {
		t.Fatalf("CreateFragment: %v", err)
	}

	call := models.ToolCall{
		Name: "add_fragment",
		Arguments: map[string]any{
			"after_fragment_id": existing.ID,
			"heading":           "New Section",
			"content":           "New section content.",
			"rationale":         "Gap in coverage",
		},
	}

	op, err := h.HandleAddFragment(context.Background(), call, "proj_1", "prd", "stage_7", "run_2")
	if err != nil {
		t.Fatalf("HandleAddFragment: %v", err)
	}
	if op.Type != "add" {
		t.Errorf("Type = %q, want %q", op.Type, "add")
	}
	if op.Heading != "New Section" {
		t.Errorf("Heading = %q, want %q", op.Heading, "New Section")
	}
	if op.FragmentID == "" {
		t.Error("FragmentID should be set")
	}
}

func TestAddFragment_AfterNotFound(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewFragmentOpsHandler(store)

	call := models.ToolCall{
		Name: "add_fragment",
		Arguments: map[string]any{
			"after_fragment_id": "nonexistent",
			"heading":           "New",
			"content":           "Content",
			"rationale":         "Reason",
		},
	}

	_, err := h.HandleAddFragment(context.Background(), call, "proj_1", "prd", "s", "r")
	if err == nil {
		t.Fatal("expected error for nonexistent after_fragment_id")
	}
}

func TestRemoveFragment_Valid(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewFragmentOpsHandler(store)

	frag, err := store.CreateFragment(context.Background(), "proj_1", "prd", "ToRemove", 2)
	if err != nil {
		t.Fatalf("CreateFragment: %v", err)
	}

	call := models.ToolCall{
		Name: "remove_fragment",
		Arguments: map[string]any{
			"fragment_id": frag.ID,
			"rationale":   "No longer relevant",
		},
	}

	op, err := h.HandleRemoveFragment(context.Background(), call)
	if err != nil {
		t.Fatalf("HandleRemoveFragment: %v", err)
	}
	if op.Type != "remove" {
		t.Errorf("Type = %q, want %q", op.Type, "remove")
	}
	if op.FragmentID != frag.ID {
		t.Errorf("FragmentID = %q, want %q", op.FragmentID, frag.ID)
	}
}

func TestRemoveFragment_NotFound(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewFragmentOpsHandler(store)

	call := models.ToolCall{
		Name: "remove_fragment",
		Arguments: map[string]any{
			"fragment_id": "gone",
			"rationale":   "reason",
		},
	}

	_, err := h.HandleRemoveFragment(context.Background(), call)
	if err == nil {
		t.Fatal("expected error for nonexistent fragment")
	}
}
