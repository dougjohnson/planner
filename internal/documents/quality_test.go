package documents

import (
	"strings"
	"testing"
)

func TestAssessPRDQuality_GoodPRD(t *testing.T) {
	prd := `# Task Manager PRD

## Overview
A task management application for teams with real-time collaboration, user stories,
and comprehensive scope definition.

## User Requirements
- As a user, I can create tasks with titles and descriptions
- As a user, I can assign tasks to team members
- As a team lead, I can view all tasks across the team

## Technical Constraints
- Must support 1000 concurrent users
- Response time under 200ms for all operations
- Data stored in PostgreSQL with proper indexing

## Success Criteria
- All user stories implemented and tested
- Performance benchmarks met
- Security audit passed

## Scope Boundaries
- V1 focuses on task CRUD and assignment
- Out of scope: Gantt charts, resource planning, billing
`
	result := AssessPRDQuality(prd)

	if result.HasWarnings {
		for _, w := range result.Warnings {
			t.Errorf("unexpected warning: %s — %s", w.Code, w.Message)
		}
	}
	if result.SectionCount < 3 {
		t.Errorf("expected >= 3 sections, got %d", result.SectionCount)
	}
}

func TestAssessPRDQuality_FewSections(t *testing.T) {
	prd := "# PRD\n\n## Overview\n\nJust one section with enough content to pass the length check. " +
		strings.Repeat("More content here. ", 30)

	result := AssessPRDQuality(prd)

	found := false
	for _, w := range result.Warnings {
		if w.Code == "few_sections" {
			found = true
		}
	}
	if !found {
		t.Error("expected few_sections warning")
	}
}

func TestAssessPRDQuality_ShortContent(t *testing.T) {
	prd := "# PRD\n\n## A\n\n## B\n\n## C\n\nShort."

	result := AssessPRDQuality(prd)

	found := false
	for _, w := range result.Warnings {
		if w.Code == "short_content" {
			found = true
		}
	}
	if !found {
		t.Error("expected short_content warning")
	}
}

func TestAssessPRDQuality_MissingElements(t *testing.T) {
	prd := "# PRD\n\n## Overview\n\n" + strings.Repeat("Generic content without key elements. ", 20) +
		"\n\n## Design\n\nMore content.\n\n## Implementation\n\nEven more content.\n"

	result := AssessPRDQuality(prd)

	codes := make(map[string]bool)
	for _, w := range result.Warnings {
		codes[w.Code] = true
	}

	expectedMissing := []string{"missing_success_criteria", "missing_scope_boundaries"}
	for _, code := range expectedMissing {
		if !codes[code] {
			t.Errorf("expected warning %s", code)
		}
	}
}

func TestAssessPRDQuality_UnfilledPlaceholders(t *testing.T) {
	prd := "# PRD\n\n## Overview\n\n[TODO: Fill in overview]\n\n## Requirements\n\n[INSERT requirements here]\n\n## Scope\n\nScope content.\n"

	result := AssessPRDQuality(prd)

	found := false
	for _, w := range result.Warnings {
		if w.Code == "unfilled_placeholders" {
			found = true
		}
	}
	if !found {
		t.Error("expected unfilled_placeholders warning")
	}
}

func TestAssessPRDQuality_SectionCount(t *testing.T) {
	prd := "## A\n\nContent\n\n## B\n\nContent\n\n## C\n\nContent\n\n## D\n\nContent\n"
	result := AssessPRDQuality(prd)

	if result.SectionCount != 4 {
		t.Errorf("expected 4 sections, got %d", result.SectionCount)
	}
}

func TestAssessPRDQuality_EmptyDocument(t *testing.T) {
	result := AssessPRDQuality("")

	if !result.HasWarnings {
		t.Error("empty document should have warnings")
	}
	if result.SectionCount != 0 {
		t.Errorf("expected 0 sections, got %d", result.SectionCount)
	}
}

func TestAssessPRDQuality_TBDPlaceholder(t *testing.T) {
	prd := "# PRD\n\n## Overview\n\nThis feature is TBD and will be defined later.\n\n## More\n\nContent.\n\n## Even More\n\nContent.\n"
	result := AssessPRDQuality(prd)

	found := false
	for _, w := range result.Warnings {
		if w.Code == "unfilled_placeholders" {
			found = true
		}
	}
	if !found {
		t.Error("expected unfilled_placeholders warning for TBD")
	}
}
