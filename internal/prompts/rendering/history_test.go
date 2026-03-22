package rendering

import (
	"strings"
	"testing"
)

func TestAssembleChangeHistory_Empty(t *testing.T) {
	result := AssembleChangeHistory(nil)
	if result != "" {
		t.Errorf("expected empty for nil iterations, got %q", result)
	}
}

func TestAssembleChangeHistory_SingleIteration(t *testing.T) {
	iterations := []IterationRecord{
		{
			Iteration:   1,
			ModelFamily: "gpt",
			Changes: []IterationChange{
				{FragmentID: "f1", FragmentName: "Overview", ChangeType: "modified", Summary: "Expanded scope section"},
				{FragmentID: "f2", FragmentName: "Requirements", ChangeType: "added", Summary: "Added performance requirements"},
			},
		},
	}

	result := AssembleChangeHistory(iterations)

	if !strings.Contains(result, "Iteration 1 (gpt)") {
		t.Error("expected iteration header")
	}
	if !strings.Contains(result, "Overview") {
		t.Error("expected fragment name 'Overview'")
	}
	if !strings.Contains(result, "[modified]") {
		t.Error("expected [modified] tag")
	}
	if !strings.Contains(result, "[added]") {
		t.Error("expected [added] tag")
	}
}

func TestAssembleChangeHistory_MultipleIterations(t *testing.T) {
	iterations := []IterationRecord{
		{
			Iteration:   1,
			ModelFamily: "gpt",
			Changes: []IterationChange{
				{FragmentName: "Overview", ChangeType: "modified", Summary: "Added detail"},
			},
		},
		{
			Iteration:   2,
			ModelFamily: "opus",
			Changes: []IterationChange{
				{FragmentName: "Security", ChangeType: "added", Summary: "New section"},
			},
			Guidance: []string{"Focus on authentication requirements"},
		},
	}

	result := AssembleChangeHistory(iterations)

	if !strings.Contains(result, "Iteration 1 (gpt)") {
		t.Error("expected iteration 1")
	}
	if !strings.Contains(result, "Iteration 2 (opus)") {
		t.Error("expected iteration 2")
	}
	if !strings.Contains(result, "User guidance applied") {
		t.Error("expected user guidance section")
	}
	if !strings.Contains(result, "authentication") {
		t.Error("expected guidance content")
	}
}

func TestAssembleChangeHistory_Convergence(t *testing.T) {
	iterations := []IterationRecord{
		{
			Iteration:   1,
			ModelFamily: "gpt",
			Changes: []IterationChange{
				{FragmentName: "A", ChangeType: "modified", Summary: "Updated"},
			},
		},
		{
			Iteration:   2,
			ModelFamily: "opus",
			Changes:     nil, // No changes — convergence.
		},
	}

	result := AssembleChangeHistory(iterations)

	if !strings.Contains(result, "convergence") {
		t.Error("expected convergence message for empty changes")
	}
}

func TestAssembleChangeHistory_RemovedFragment(t *testing.T) {
	iterations := []IterationRecord{
		{
			Iteration:   1,
			ModelFamily: "gpt",
			Changes: []IterationChange{
				{FragmentName: "Legacy", ChangeType: "removed", Summary: "Removed outdated section"},
			},
		},
	}

	result := AssembleChangeHistory(iterations)

	if !strings.Contains(result, "[removed]") {
		t.Error("expected [removed] tag")
	}
	if !strings.Contains(result, "- **Legacy**") {
		t.Error("expected fragment name")
	}
}

func TestAssembleChangeHistory_Header(t *testing.T) {
	iterations := []IterationRecord{
		{Iteration: 1, ModelFamily: "gpt", Changes: []IterationChange{{ChangeType: "modified", Summary: "x"}}},
	}

	result := AssembleChangeHistory(iterations)

	if !strings.Contains(result, "Review Loop Change History") {
		t.Error("expected history header")
	}
	if !strings.Contains(result, "Do NOT re-propose") {
		t.Error("expected instruction about not re-proposing")
	}
}

func TestAssembleChangeHistory_FallbackToFragmentID(t *testing.T) {
	iterations := []IterationRecord{
		{
			Iteration:   1,
			ModelFamily: "gpt",
			Changes: []IterationChange{
				{FragmentID: "frag-123", FragmentName: "", ChangeType: "modified", Summary: "Updated"},
			},
		},
	}

	result := AssembleChangeHistory(iterations)

	if !strings.Contains(result, "frag-123") {
		t.Error("expected fragment ID fallback when name is empty")
	}
}
