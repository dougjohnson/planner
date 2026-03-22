package documents

import (
	"strings"
)

// QualityWarning represents a single quality concern about a seed PRD.
type QualityWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// QualityAssessment contains the results of a deterministic PRD quality check.
type QualityAssessment struct {
	Warnings    []QualityWarning `json:"warnings"`
	SectionCount int            `json:"section_count"`
	CharCount    int            `json:"character_count"`
	HasWarnings  bool           `json:"has_warnings"`
}

// commonElements are headings/keywords expected in a well-structured PRD.
var commonElements = []struct {
	keywords []string
	label    string
}{
	{[]string{"success criteria", "acceptance criteria", "definition of done"}, "success criteria"},
	{[]string{"technical", "constraint", "limitation", "non-functional"}, "technical constraints"},
	{[]string{"user", "requirement", "story", "persona"}, "user requirements"},
	{[]string{"scope", "out of scope", "boundary", "boundaries"}, "scope boundaries"},
}

// placeholderPatterns indicate unfilled template content.
var placeholderPatterns = []string{
	"[TODO",
	"[INSERT",
	"[FILL",
	"[YOUR",
	"[REPLACE",
	"<TODO",
	"<INSERT",
	"{{",
	"TBD",
	"Lorem ipsum",
}

// AssessPRDQuality performs a deterministic structural assessment of a seed PRD.
// All checks are advisory — none are blocking.
func AssessPRDQuality(content string) *QualityAssessment {
	result := &QualityAssessment{
		CharCount: len(strings.TrimSpace(content)),
	}

	// Count ## sections.
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			result.SectionCount++
		}
	}

	// Check: fewer than 3 sections.
	if result.SectionCount < 3 {
		result.Warnings = append(result.Warnings, QualityWarning{
			Code:    "few_sections",
			Message: "Document has fewer than 3 sections — consider adding more structure for better model output.",
		})
	}

	// Check: fewer than 500 characters.
	if result.CharCount < 500 {
		result.Warnings = append(result.Warnings, QualityWarning{
			Code:    "short_content",
			Message: "Document is shorter than 500 characters — more detail typically produces better results.",
		})
	}

	// Check: missing common elements.
	lower := strings.ToLower(content)
	for _, elem := range commonElements {
		found := false
		for _, kw := range elem.keywords {
			if strings.Contains(lower, kw) {
				found = true
				break
			}
		}
		if !found {
			result.Warnings = append(result.Warnings, QualityWarning{
				Code:    "missing_" + strings.ReplaceAll(elem.label, " ", "_"),
				Message: "No mention of " + elem.label + " found — consider adding this for completeness.",
			})
		}
	}

	// Check: unfilled placeholders.
	for _, pattern := range placeholderPatterns {
		if strings.Contains(content, pattern) {
			result.Warnings = append(result.Warnings, QualityWarning{
				Code:    "unfilled_placeholders",
				Message: "Document contains template placeholders that should be filled in before proceeding.",
			})
			break // One warning is enough.
		}
	}

	result.HasWarnings = len(result.Warnings) > 0
	return result
}
