package tools

import (
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

func TestChangeRationale_Valid(t *testing.T) {
	call := models.ToolCall{
		Name: "submit_change_rationale",
		Arguments: map[string]any{
			"section_id":   "intro",
			"change_type":  "modified",
			"rationale":    "Improved clarity based on GPT output.",
			"source_model": "gpt-4",
		},
	}

	r, err := HandleSubmitChangeRationale(call, "stage_4", "run_1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if r.SectionID != "intro" {
		t.Errorf("SectionID = %q", r.SectionID)
	}
	if r.ChangeType != "modified" {
		t.Errorf("ChangeType = %q", r.ChangeType)
	}
	if r.SourceModel != "gpt-4" {
		t.Errorf("SourceModel = %q", r.SourceModel)
	}
}

func TestChangeRationale_NoSourceModel(t *testing.T) {
	call := models.ToolCall{
		Name: "submit_change_rationale",
		Arguments: map[string]any{
			"section_id":  "arch",
			"change_type": "added",
			"rationale":   "New section needed.",
		},
	}

	r, err := HandleSubmitChangeRationale(call, "stage_4", "run_1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if r.SourceModel != "" {
		t.Errorf("SourceModel should be empty, got %q", r.SourceModel)
	}
}

func TestChangeRationale_InvalidType(t *testing.T) {
	call := models.ToolCall{
		Name: "submit_change_rationale",
		Arguments: map[string]any{
			"section_id":  "s1",
			"change_type": "destroyed",
			"rationale":   "reason",
		},
	}

	_, err := HandleSubmitChangeRationale(call, "s", "r")
	if err == nil {
		t.Fatal("expected error for invalid change_type")
	}
}

func TestChangeRationale_MissingRequired(t *testing.T) {
	call := models.ToolCall{
		Name: "submit_change_rationale",
		Arguments: map[string]any{
			"section_id": "s1",
		},
	}

	_, err := HandleSubmitChangeRationale(call, "s", "r")
	if err == nil {
		t.Fatal("expected error for missing required args")
	}
}

func TestChangeRationale_AllTypes(t *testing.T) {
	for _, ct := range []string{"added", "modified", "removed", "reorganized"} {
		call := models.ToolCall{
			Name: "submit_change_rationale",
			Arguments: map[string]any{
				"section_id":  "s1",
				"change_type": ct,
				"rationale":   "reason",
			},
		}
		r, err := HandleSubmitChangeRationale(call, "s", "r")
		if err != nil {
			t.Errorf("change_type %q: unexpected error: %v", ct, err)
		}
		if r.ChangeType != ct {
			t.Errorf("ChangeType = %q, want %q", r.ChangeType, ct)
		}
	}
}
