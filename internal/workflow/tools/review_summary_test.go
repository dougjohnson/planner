package tools

import (
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

func TestSubmitReviewSummary_Valid(t *testing.T) {
	call := models.ToolCall{
		Name: "submit_review_summary",
		Arguments: map[string]any{
			"summary":      "Document is well-structured with minor issues.",
			"key_findings": []any{"Improved clarity in architecture", "Missing error handling section"},
		},
	}

	result, err := HandleSubmitReviewSummary(call, "stage_7", "run_1")
	if err != nil {
		t.Fatalf("HandleSubmitReviewSummary: %v", err)
	}
	if result.Summary != "Document is well-structured with minor issues." {
		t.Errorf("Summary = %q", result.Summary)
	}
	if len(result.KeyFindings) != 2 {
		t.Fatalf("KeyFindings count = %d, want 2", len(result.KeyFindings))
	}
	if result.KeyFindings[0] != "Improved clarity in architecture" {
		t.Errorf("KeyFindings[0] = %q", result.KeyFindings[0])
	}
}

func TestSubmitReviewSummary_NoFindings(t *testing.T) {
	call := models.ToolCall{
		Name: "submit_review_summary",
		Arguments: map[string]any{
			"summary": "No changes warranted.",
		},
	}

	result, err := HandleSubmitReviewSummary(call, "stage_7", "run_1")
	if err != nil {
		t.Fatalf("HandleSubmitReviewSummary: %v", err)
	}
	if len(result.KeyFindings) != 0 {
		t.Errorf("KeyFindings should be empty, got %d", len(result.KeyFindings))
	}
}

func TestSubmitReviewSummary_MissingSummary(t *testing.T) {
	call := models.ToolCall{
		Name:      "submit_review_summary",
		Arguments: map[string]any{},
	}

	_, err := HandleSubmitReviewSummary(call, "s", "r")
	if err == nil {
		t.Fatal("expected error for missing summary")
	}
}

func TestSubmitReviewSummary_InvalidFindings(t *testing.T) {
	call := models.ToolCall{
		Name: "submit_review_summary",
		Arguments: map[string]any{
			"summary":      "ok",
			"key_findings": "not an array",
		},
	}

	_, err := HandleSubmitReviewSummary(call, "s", "r")
	if err == nil {
		t.Fatal("expected error for non-array key_findings")
	}
}
