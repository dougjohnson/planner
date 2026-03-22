package tools

import (
	"context"
	"database/sql"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/db"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), ":memory:", nil)
	if err != nil {
		// Fallback: open raw sqlite
		database, err = sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("opening test db: %v", err)
		}
	}

	// Create the tables needed for fragments.
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS fragments (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			document_type TEXT NOT NULL CHECK(document_type IN ('prd', 'plan')),
			heading TEXT NOT NULL,
			depth INTEGER NOT NULL DEFAULT 2,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS fragment_versions (
			id TEXT PRIMARY KEY,
			fragment_id TEXT NOT NULL REFERENCES fragments(id),
			content TEXT NOT NULL,
			source_stage TEXT NOT NULL DEFAULT '',
			source_run_id TEXT NOT NULL DEFAULT '',
			change_rationale TEXT NOT NULL DEFAULT '',
			checksum TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	} {
		if _, err := database.Exec(stmt); err != nil {
			t.Fatalf("creating table: %v", err)
		}
	}

	t.Cleanup(func() { database.Close() })
	return database
}

func TestValidate_MissingContent(t *testing.T) {
	h := NewSubmitDocumentHandler(nil)
	err := h.Validate(models.ToolCall{
		Name:      "submit_document",
		Arguments: map[string]any{"change_summary": "hello"},
	})
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestValidate_EmptyContent(t *testing.T) {
	h := NewSubmitDocumentHandler(nil)
	err := h.Validate(models.ToolCall{
		Name:      "submit_document",
		Arguments: map[string]any{"content": ""},
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestValidate_ValidCall(t *testing.T) {
	h := NewSubmitDocumentHandler(nil)
	err := h.Validate(models.ToolCall{
		Name:      "submit_document",
		Arguments: map[string]any{"content": "# Doc\n\nHello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_WrongTool(t *testing.T) {
	h := NewSubmitDocumentHandler(nil)
	err := h.Validate(models.ToolCall{
		Name:      "update_fragment",
		Arguments: map[string]any{"content": "x"},
	})
	if err == nil {
		t.Fatal("expected error for wrong tool name")
	}
}

func TestExecute_NewDocument(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewSubmitDocumentHandler(store)

	call := models.ToolCall{
		Name: "submit_document",
		Arguments: map[string]any{
			"content":        "## Introduction\n\nThis is the intro.\n\n## Architecture\n\nThis describes architecture.\n",
			"change_summary": "Initial PRD submission",
		},
	}

	result, err := h.Execute(context.Background(), call, "proj_1", "prd", "stage_3", "run_1")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result.FragmentCount != 2 {
		t.Errorf("FragmentCount = %d, want 2", result.FragmentCount)
	}
	if result.NewFragments != 2 {
		t.Errorf("NewFragments = %d, want 2", result.NewFragments)
	}
	if result.UpdatedFragments != 0 {
		t.Errorf("UpdatedFragments = %d, want 0", result.UpdatedFragments)
	}
	if result.ChangeSummary != "Initial PRD submission" {
		t.Errorf("ChangeSummary = %q", result.ChangeSummary)
	}
}

func TestExecute_UpdateExistingFragments(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewSubmitDocumentHandler(store)

	// First submission.
	call1 := models.ToolCall{
		Name: "submit_document",
		Arguments: map[string]any{
			"content":        "## Intro\n\nOriginal content.\n\n## Details\n\nOriginal details.\n",
			"change_summary": "seed",
		},
	}
	_, err := h.Execute(context.Background(), call1, "proj_1", "prd", "stage_3", "run_1")
	if err != nil {
		t.Fatalf("first Execute error: %v", err)
	}

	// Second submission with one changed section.
	call2 := models.ToolCall{
		Name: "submit_document",
		Arguments: map[string]any{
			"content":        "## Intro\n\nOriginal content.\n\n## Details\n\nUpdated details.\n",
			"change_summary": "revised details",
		},
	}
	result, err := h.Execute(context.Background(), call2, "proj_1", "prd", "stage_4", "run_2")
	if err != nil {
		t.Fatalf("second Execute error: %v", err)
	}

	if result.FragmentCount != 2 {
		t.Errorf("FragmentCount = %d, want 2", result.FragmentCount)
	}
	if result.UnchangedFragments != 1 {
		t.Errorf("UnchangedFragments = %d, want 1", result.UnchangedFragments)
	}
	if result.UpdatedFragments != 1 {
		t.Errorf("UpdatedFragments = %d, want 1", result.UpdatedFragments)
	}
	if result.NewFragments != 0 {
		t.Errorf("NewFragments = %d, want 0", result.NewFragments)
	}
}

func TestExecute_WithPreamble(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewSubmitDocumentHandler(store)

	call := models.ToolCall{
		Name: "submit_document",
		Arguments: map[string]any{
			"content": "# Top Title\n\nPreamble text.\n\n## Section One\n\nContent.\n",
		},
	}

	result, err := h.Execute(context.Background(), call, "proj_1", "prd", "stage_3", "run_1")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	// Preamble + one section = 2 fragments.
	if result.FragmentCount != 2 {
		t.Errorf("FragmentCount = %d, want 2", result.FragmentCount)
	}
}

func TestValidateToolCall_RequiredArgMissing(t *testing.T) {
	tools := models.GenerationTools()
	call := models.ToolCall{
		Name:      "submit_document",
		Arguments: map[string]any{},
	}

	err := ValidateToolCall(call, tools)
	if err == nil {
		t.Fatal("expected validation error for missing required args")
	}
}

func TestValidateToolCall_UnknownTool(t *testing.T) {
	tools := models.GenerationTools()
	call := models.ToolCall{
		Name:      "nonexistent_tool",
		Arguments: map[string]any{},
	}

	err := ValidateToolCall(call, tools)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestValidateToolCall_ValidCall(t *testing.T) {
	tools := models.GenerationTools()
	call := models.ToolCall{
		Name: "submit_document",
		Arguments: map[string]any{
			"content":        "hello",
			"change_summary": "initial",
		},
	}

	err := ValidateToolCall(call, tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateToolCall_InvalidEnum(t *testing.T) {
	tools := models.SynthesisTools()
	call := models.ToolCall{
		Name: "submit_change_rationale",
		Arguments: map[string]any{
			"section_id":  "s1",
			"change_type": "invalid_type",
			"rationale":   "because",
		},
	}

	err := ValidateToolCall(call, tools)
	if err == nil {
		t.Fatal("expected validation error for invalid enum value")
	}
}
