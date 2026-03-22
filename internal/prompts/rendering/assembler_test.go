package rendering

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
		t.Fatalf("opening db: %v", err)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")
	if err := migrations.Run(context.Background(), db, testLogger()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestAssemble_DeterministicOrder(t *testing.T) {
	a := NewAssembler(nil)
	input := AssemblyInput{
		Stage:               "prd_gpt_synthesis",
		SystemInstructions:  "You are a planning assistant.",
		ToolDefinitions:     "Tool: submit_document",
		FoundationalContext: "Project: Flywheel Planner\nStack: Go + React",
		PromptTemplateID:    "pt-1",
		PromptText:          "Synthesize the following PRD sections.",
		ArtifactContext:     "## Introduction\nContent here.",
		ArtifactIDs:         []string{"a-1", "a-2"},
		ChangeHistory:       "Iteration 2: Updated scope section.",
		UserGuidance:        "Focus on security requirements.",
	}

	rp := a.Assemble(context.Background(), input)

	// Verify all 6 segments in correct order.
	if len(rp.Segments) != 6 {
		t.Fatalf("expected 6 segments, got %d", len(rp.Segments))
	}

	expectedLabels := []string{
		"system_instructions",
		"foundational_context",
		"prompt_text",
		"artifact_context",
		"change_history",
		"user_guidance",
	}

	for i, label := range expectedLabels {
		if rp.Segments[i].Label != label {
			t.Errorf("segment %d: expected label %q, got %q", i, label, rp.Segments[i].Label)
		}
	}
}

func TestAssemble_SystemInstructionsIncludesTools(t *testing.T) {
	a := NewAssembler(nil)
	input := AssemblyInput{
		SystemInstructions: "You are an assistant.",
		ToolDefinitions:    "Tool: submit_document",
	}

	rp := a.Assemble(context.Background(), input)

	if len(rp.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(rp.Segments))
	}
	if !strings.Contains(rp.Segments[0].Content, "You are an assistant.") {
		t.Error("missing system instructions")
	}
	if !strings.Contains(rp.Segments[0].Content, "submit_document") {
		t.Error("missing tool definitions")
	}
}

func TestAssemble_EmptyOptionalSegments(t *testing.T) {
	a := NewAssembler(nil)
	input := AssemblyInput{
		PromptTemplateID: "pt-1",
		PromptText:       "Generate the plan.",
	}

	rp := a.Assemble(context.Background(), input)

	// Only prompt_text segment should exist.
	if len(rp.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(rp.Segments))
	}
	if rp.Segments[0].Label != "prompt_text" {
		t.Errorf("expected 'prompt_text', got %q", rp.Segments[0].Label)
	}
}

func TestAssemble_FirstIteration_NoChangeHistory(t *testing.T) {
	a := NewAssembler(nil)
	input := AssemblyInput{
		PromptText:    "Synthesize.",
		ChangeHistory: "", // First iteration — no history.
	}

	rp := a.Assemble(context.Background(), input)

	for _, seg := range rp.Segments {
		if seg.Label == "change_history" {
			t.Error("change_history segment should not exist for first iteration")
		}
	}
}

func TestAssemble_FullText(t *testing.T) {
	a := NewAssembler(nil)
	input := AssemblyInput{
		PromptText:    "Part A.",
		UserGuidance:  "Part B.",
	}

	rp := a.Assemble(context.Background(), input)
	text := rp.FullText()

	if !strings.Contains(text, "Part A.") || !strings.Contains(text, "Part B.") {
		t.Errorf("FullText missing segments: %q", text)
	}
}

func TestAssemble_ArtifactIDs(t *testing.T) {
	a := NewAssembler(nil)
	input := AssemblyInput{
		PromptText:  "Test.",
		ArtifactIDs: []string{"a-1", "a-2", "a-3"},
	}

	rp := a.Assemble(context.Background(), input)

	if len(rp.ArtifactIDs) != 3 {
		t.Errorf("expected 3 artifact IDs, got %d", len(rp.ArtifactIDs))
	}
}

func TestAssemble_IDAndTimestamp(t *testing.T) {
	a := NewAssembler(nil)
	input := AssemblyInput{PromptText: "Test."}

	rp := a.Assemble(context.Background(), input)

	if rp.ID == "" {
		t.Error("expected non-empty ID")
	}
	if rp.RenderedAt == "" {
		t.Error("expected non-empty RenderedAt")
	}
}

func TestSaveRender(t *testing.T) {
	db := setupTestDB(t)
	a := NewAssembler(db)
	ctx := context.Background()
	now := "2026-01-01T00:00:00Z"

	// Create prerequisites.
	db.ExecContext(ctx, "INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)
	db.ExecContext(ctx, "INSERT INTO prompt_templates (id, name, stage, version, created_at, updated_at) VALUES ('pt-1', 'test', 's1', 1, ?, ?)", now, now)
	db.ExecContext(ctx, "INSERT INTO workflow_runs (id, project_id, stage, created_at) VALUES ('wr-1', 'p-1', 's1', ?)", now)

	rp := &RenderedPrompt{
		ID:               "rp-1",
		PromptTemplateID: "pt-1",
		RenderedAt:       now,
	}

	err := a.SaveRender(ctx, "wr-1", rp, "/prompts/render-1.json")
	if err != nil {
		t.Fatalf("SaveRender: %v", err)
	}

	// Verify record exists.
	var count int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM prompt_renders WHERE id = 'rp-1'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 prompt render record, got %d", count)
	}
}
