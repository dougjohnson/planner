package tools

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

func TestReportAgreement_Valid(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewDispositionHandler(store)

	frag, _ := store.CreateFragment(context.Background(), "proj_1", "prd", "Section", 2)

	call := models.ToolCall{
		Name: "report_agreement",
		Arguments: map[string]any{
			"fragment_id": frag.ID,
			"category":    "wholeheartedly_agrees",
			"rationale":   "Well written and comprehensive.",
		},
	}

	report, err := h.HandleReportAgreement(context.Background(), call, "stage_5", "run_1")
	if err != nil {
		t.Fatalf("HandleReportAgreement: %v", err)
	}
	if report.FragmentID != frag.ID {
		t.Errorf("FragmentID = %q, want %q", report.FragmentID, frag.ID)
	}
	if report.Category != "wholeheartedly_agrees" {
		t.Errorf("Category = %q", report.Category)
	}
}

func TestReportAgreement_InvalidCategory(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewDispositionHandler(store)

	frag, _ := store.CreateFragment(context.Background(), "proj_1", "prd", "Section", 2)

	call := models.ToolCall{
		Name: "report_agreement",
		Arguments: map[string]any{
			"fragment_id": frag.ID,
			"category":    "strongly_agrees",
			"rationale":   "reason",
		},
	}

	_, err := h.HandleReportAgreement(context.Background(), call, "s", "r")
	if err == nil {
		t.Fatal("expected error for invalid category")
	}
}

func TestReportAgreement_FragmentNotFound(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewDispositionHandler(store)

	call := models.ToolCall{
		Name: "report_agreement",
		Arguments: map[string]any{
			"fragment_id": "nonexistent",
			"category":    "somewhat_agrees",
			"rationale":   "reason",
		},
	}

	_, err := h.HandleReportAgreement(context.Background(), call, "s", "r")
	if err == nil {
		t.Fatal("expected error for nonexistent fragment")
	}
}

func TestReportDisagreement_Valid(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewDispositionHandler(store)

	frag, _ := store.CreateFragment(context.Background(), "proj_1", "prd", "Architecture", 2)

	call := models.ToolCall{
		Name: "report_disagreement",
		Arguments: map[string]any{
			"fragment_id":      frag.ID,
			"severity":         "major",
			"summary":          "Missing error handling section",
			"rationale":        "The architecture section does not address failure modes.",
			"suggested_change": "Add a subsection on error handling and recovery.",
		},
	}

	report, err := h.HandleReportDisagreement(context.Background(), call, "stage_5", "run_1")
	if err != nil {
		t.Fatalf("HandleReportDisagreement: %v", err)
	}
	if report.Severity != "major" {
		t.Errorf("Severity = %q", report.Severity)
	}
	if report.Summary != "Missing error handling section" {
		t.Errorf("Summary = %q", report.Summary)
	}
	if report.SuggestedChange == "" {
		t.Error("SuggestedChange should not be empty")
	}
}

func TestReportDisagreement_InvalidSeverity(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewDispositionHandler(store)

	frag, _ := store.CreateFragment(context.Background(), "proj_1", "prd", "Section", 2)

	call := models.ToolCall{
		Name: "report_disagreement",
		Arguments: map[string]any{
			"fragment_id":      frag.ID,
			"severity":         "critical",
			"summary":          "issue",
			"rationale":        "reason",
			"suggested_change": "fix",
		},
	}

	_, err := h.HandleReportDisagreement(context.Background(), call, "s", "r")
	if err == nil {
		t.Fatal("expected error for invalid severity")
	}
}

func TestReportDisagreement_MissingArgs(t *testing.T) {
	h := NewDispositionHandler(nil)

	call := models.ToolCall{
		Name: "report_disagreement",
		Arguments: map[string]any{
			"fragment_id": "x",
			"severity":    "minor",
			// missing summary, rationale, suggested_change
		},
	}

	_, err := h.HandleReportDisagreement(context.Background(), call, "s", "r")
	if err == nil {
		t.Fatal("expected error for missing required args")
	}
}

func TestReportDisagreement_FragmentNotFound(t *testing.T) {
	database := setupTestDB(t)
	store := fragments.NewStore(database)
	h := NewDispositionHandler(store)

	call := models.ToolCall{
		Name: "report_disagreement",
		Arguments: map[string]any{
			"fragment_id":      "gone",
			"severity":         "minor",
			"summary":          "issue",
			"rationale":        "reason",
			"suggested_change": "fix",
		},
	}

	_, err := h.HandleReportDisagreement(context.Background(), call, "s", "r")
	if err == nil {
		t.Fatal("expected error for nonexistent fragment")
	}
}
