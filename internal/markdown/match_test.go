package markdown

import (
	"testing"
)

func TestMatchSections_ExactMatch(t *testing.T) {
	sections := []Section{
		{Heading: "Overview", Position: 0},
		{Heading: "Requirements", Position: 1},
	}
	frags := []FragmentRef{
		{ID: "f1", Heading: "Overview"},
		{ID: "f2", Heading: "Requirements"},
	}

	results := MatchSections(sections, frags)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].FragmentID != "f1" || results[0].MatchType != MatchExact {
		t.Errorf("expected exact match to f1, got %s/%s", results[0].FragmentID, results[0].MatchType)
	}
	if results[1].FragmentID != "f2" || results[1].MatchType != MatchExact {
		t.Errorf("expected exact match to f2, got %s/%s", results[1].FragmentID, results[1].MatchType)
	}
}

func TestMatchSections_DuplicateHeadings(t *testing.T) {
	sections := []Section{
		{Heading: "Section", Position: 0},
		{Heading: "Section", Position: 1},
		{Heading: "Section", Position: 2},
	}
	frags := []FragmentRef{
		{ID: "f1", Heading: "Section"},
		{ID: "f2", Heading: "Section"},
		{ID: "f3", Heading: "Section"},
	}

	results := MatchSections(sections, frags)

	if results[0].FragmentID != "f1" || results[0].MatchType != MatchPositional {
		t.Errorf("first: expected positional match to f1, got %s/%s", results[0].FragmentID, results[0].MatchType)
	}
	if results[1].FragmentID != "f2" {
		t.Errorf("second: expected f2, got %s", results[1].FragmentID)
	}
	if results[2].FragmentID != "f3" {
		t.Errorf("third: expected f3, got %s", results[2].FragmentID)
	}
}

func TestMatchSections_NewSection(t *testing.T) {
	sections := []Section{
		{Heading: "Overview", Position: 0},
		{Heading: "New Section", Position: 1},
	}
	frags := []FragmentRef{
		{ID: "f1", Heading: "Overview"},
	}

	results := MatchSections(sections, frags)

	if results[0].FragmentID != "f1" {
		t.Errorf("expected match to f1")
	}
	if results[1].MatchType != MatchNew {
		t.Errorf("expected MatchNew for new section, got %s", results[1].MatchType)
	}
	if results[1].FragmentID != "" {
		t.Errorf("new section should have empty fragment ID")
	}
}

func TestMatchSections_RemovedSection(t *testing.T) {
	sections := []Section{
		{Heading: "Overview", Position: 0},
	}
	frags := []FragmentRef{
		{ID: "f1", Heading: "Overview"},
		{ID: "f2", Heading: "Removed Section"},
	}

	results := MatchSections(sections, frags)
	unmatched := UnmatchedFragments(results, frags)

	if len(unmatched) != 1 {
		t.Fatalf("expected 1 unmatched fragment, got %d", len(unmatched))
	}
	if unmatched[0] != "f2" {
		t.Errorf("expected f2 unmatched, got %s", unmatched[0])
	}
}

func TestMatchSections_Preamble(t *testing.T) {
	sections := []Section{
		{Heading: "", Position: 0}, // preamble
		{Heading: "Section", Position: 1},
	}
	frags := []FragmentRef{
		{ID: "f-pre", Heading: ""},
		{ID: "f1", Heading: "Section"},
	}

	results := MatchSections(sections, frags)
	if results[0].FragmentID != "f-pre" {
		t.Errorf("expected preamble match to f-pre, got %s", results[0].FragmentID)
	}
}

func TestMatchSections_MoreDuplicatesThanFragments(t *testing.T) {
	sections := []Section{
		{Heading: "Item", Position: 0},
		{Heading: "Item", Position: 1},
		{Heading: "Item", Position: 2},
	}
	frags := []FragmentRef{
		{ID: "f1", Heading: "Item"},
		{ID: "f2", Heading: "Item"},
	}

	results := MatchSections(sections, frags)

	if results[0].FragmentID != "f1" {
		t.Errorf("first should match f1")
	}
	if results[1].FragmentID != "f2" {
		t.Errorf("second should match f2")
	}
	if results[2].MatchType != MatchNew {
		t.Errorf("third should be new, got %s", results[2].MatchType)
	}
}

func TestMatchSections_EmptyInputs(t *testing.T) {
	results := MatchSections(nil, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil inputs")
	}

	results = MatchSections([]Section{}, []FragmentRef{})
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty inputs")
	}
}

func TestMatchSections_NoExistingFragments(t *testing.T) {
	sections := []Section{
		{Heading: "A", Position: 0},
		{Heading: "B", Position: 1},
	}

	results := MatchSections(sections, nil)
	for _, r := range results {
		if r.MatchType != MatchNew {
			t.Errorf("all should be new when no existing fragments, got %s for %q", r.MatchType, r.Section.Heading)
		}
	}
}

func TestUnmatchedFragments_AllMatched(t *testing.T) {
	results := []MatchResult{
		{FragmentID: "f1"},
		{FragmentID: "f2"},
	}
	frags := []FragmentRef{
		{ID: "f1"},
		{ID: "f2"},
	}

	unmatched := UnmatchedFragments(results, frags)
	if len(unmatched) != 0 {
		t.Errorf("expected no unmatched, got %v", unmatched)
	}
}

func TestUnmatchedFragments_NoneMatched(t *testing.T) {
	results := []MatchResult{
		{MatchType: MatchNew},
	}
	frags := []FragmentRef{
		{ID: "f1"},
		{ID: "f2"},
	}

	unmatched := UnmatchedFragments(results, frags)
	if len(unmatched) != 2 {
		t.Errorf("expected 2 unmatched, got %d", len(unmatched))
	}
}
