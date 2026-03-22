// Package export handles final stabilization checks and bundle assembly.
package export

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// Severity classifies a stabilization finding.
type Severity string

const (
	SeverityBlocking Severity = "blocking" // blocks export
	SeverityWarning  Severity = "warning"  // advisory, does not block
)

// Finding describes a single issue found during stabilization.
type Finding struct {
	Severity    Severity `json:"severity"`
	Check       string   `json:"check"`
	ArtifactID  string   `json:"artifact_id,omitempty"`
	FragmentID  string   `json:"fragment_id,omitempty"`
	Location    string   `json:"location"`
	Message     string   `json:"message"`
}

// StabilizationReport holds the outcome of the final stabilization pass.
type StabilizationReport struct {
	Passed        bool      `json:"passed"` // true if no blocking findings
	BlockingCount int       `json:"blocking_count"`
	WarningCount  int       `json:"warning_count"`
	Findings      []Finding `json:"findings"`
}

// placeholderPatterns matches common unresolved placeholder markers.
var placeholderPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bTODO\b`),
	regexp.MustCompile(`(?i)\bTBD\b`),
	regexp.MustCompile(`(?i)\bFIXME\b`),
	regexp.MustCompile(`(?i)\bXXX\b`),
	regexp.MustCompile(`\[INSERT .+?\]`),
	regexp.MustCompile(`\{INSERT .+?\}`),
	regexp.MustCompile(`<INSERT .+?>`),
	regexp.MustCompile(`(?i)\[placeholder\]`),
	regexp.MustCompile(`(?i)\[fill in\]`),
}

// RunStabilization executes all final checks on canonical artifacts before export.
// It produces a reviewable report without modifying any artifacts.
func RunStabilization(ctx context.Context, db *sql.DB, projectID string) (*StabilizationReport, error) {
	report := &StabilizationReport{}

	// Load canonical artifact content.
	artifacts, err := loadCanonicalContent(ctx, db, projectID)
	if err != nil {
		return nil, fmt.Errorf("loading canonical artifacts: %w", err)
	}

	for _, art := range artifacts {
		// Check 1: Unresolved placeholders.
		checkPlaceholders(art, report)

		// Check 2: Duplicate headings.
		checkDuplicateHeadings(art, report)
	}

	// Check 3: Cross-reference consistency.
	checkCrossReferences(artifacts, report)

	// Check 4: Export manifest completeness.
	checkManifestCompleteness(ctx, db, projectID, report)

	// Compute summary.
	for _, f := range report.Findings {
		if f.Severity == SeverityBlocking {
			report.BlockingCount++
		} else {
			report.WarningCount++
		}
	}
	report.Passed = report.BlockingCount == 0

	return report, nil
}

// FormatStabilizationReport returns a human-readable report.
func FormatStabilizationReport(r *StabilizationReport) string {
	if r.Passed && len(r.Findings) == 0 {
		return "Stabilization passed: all checks clean."
	}

	var b strings.Builder
	if r.Passed {
		fmt.Fprintf(&b, "Stabilization passed with %d warning(s).\n\n", r.WarningCount)
	} else {
		fmt.Fprintf(&b, "Stabilization FAILED: %d blocking issue(s), %d warning(s).\n\n", r.BlockingCount, r.WarningCount)
	}

	for i, f := range r.Findings {
		icon := "  "
		if f.Severity == SeverityBlocking {
			icon = "!!"
		}
		fmt.Fprintf(&b, "%s %d. [%s] %s\n", icon, i+1, f.Check, f.Message)
		if f.Location != "" {
			fmt.Fprintf(&b, "      Location: %s\n", f.Location)
		}
	}
	return b.String()
}

// --- Internal types and helpers ---

type artifactContent struct {
	id           string
	artifactType string
	headings     []string
	content      string
}

func loadCanonicalContent(ctx context.Context, db *sql.DB, projectID string) ([]artifactContent, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, a.artifact_type, f.heading, fv.content
		FROM artifacts a
		JOIN artifact_fragments af ON af.artifact_id = a.id
		JOIN fragment_versions fv ON fv.id = af.fragment_version_id
		JOIN fragments f ON f.id = fv.fragment_id
		WHERE a.project_id = ? AND a.is_canonical = 1
		ORDER BY a.id, af.position ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	artMap := make(map[string]*artifactContent)
	var order []string

	for rows.Next() {
		var artID, artType, heading, content string
		if err := rows.Scan(&artID, &artType, &heading, &content); err != nil {
			return nil, err
		}
		if _, ok := artMap[artID]; !ok {
			artMap[artID] = &artifactContent{id: artID, artifactType: artType}
			order = append(order, artID)
		}
		a := artMap[artID]
		if heading != "" {
			a.headings = append(a.headings, heading)
		}
		a.content += content + "\n"
	}

	var result []artifactContent
	for _, id := range order {
		result = append(result, *artMap[id])
	}
	return result, rows.Err()
}

func checkPlaceholders(art artifactContent, report *StabilizationReport) {
	lines := strings.Split(art.content, "\n")
	for lineNum, line := range lines {
		for _, pat := range placeholderPatterns {
			if pat.MatchString(line) {
				report.Findings = append(report.Findings, Finding{
					Severity:   SeverityWarning,
					Check:      "unresolved_placeholder",
					ArtifactID: art.id,
					Location:   fmt.Sprintf("%s line %d", art.artifactType, lineNum+1),
					Message:    fmt.Sprintf("unresolved placeholder in %s: %q", art.artifactType, strings.TrimSpace(line)),
				})
				break // one finding per line
			}
		}
	}
}

func checkDuplicateHeadings(art artifactContent, report *StabilizationReport) {
	seen := make(map[string]int)
	for _, h := range art.headings {
		seen[h]++
	}
	for heading, count := range seen {
		if count > 1 {
			report.Findings = append(report.Findings, Finding{
				Severity:   SeverityWarning,
				Check:      "duplicate_heading",
				ArtifactID: art.id,
				Location:   art.artifactType,
				Message:    fmt.Sprintf("heading %q appears %d times in %s", heading, count, art.artifactType),
			})
		}
	}
}

func checkCrossReferences(artifacts []artifactContent, report *StabilizationReport) {
	// Check that if a plan references sections from the PRD, those sections exist.
	var prdHeadings map[string]bool
	var planContent string

	for _, art := range artifacts {
		if art.artifactType == "prd" {
			prdHeadings = make(map[string]bool)
			for _, h := range art.headings {
				prdHeadings[h] = true
			}
		}
		if art.artifactType == "plan" {
			planContent = art.content
		}
	}

	if prdHeadings == nil || planContent == "" {
		return // not enough artifacts to cross-reference
	}

	// Simple heuristic: check for "§" or "Section:" references in plan that don't match PRD headings.
	sectionRefPattern := regexp.MustCompile(`(?i)(?:section|§)\s*[":]\s*(.+?)(?:["\n,.]|$)`)
	matches := sectionRefPattern.FindAllStringSubmatch(planContent, -1)
	for _, match := range matches {
		ref := strings.TrimSpace(match[1])
		if ref != "" && !prdHeadings[ref] {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityWarning,
				Check:    "cross_reference",
				Location: "plan",
				Message:  fmt.Sprintf("plan references section %q which may not exist in PRD", ref),
			})
		}
	}
}

func checkManifestCompleteness(ctx context.Context, db *sql.DB, projectID string, report *StabilizationReport) {
	// Verify required artifacts exist.
	requiredTypes := []string{"prd", "plan"}
	for _, artType := range requiredTypes {
		var count int
		db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM artifacts WHERE project_id = ? AND artifact_type = ? AND is_canonical = 1`,
			projectID, artType).Scan(&count)
		if count == 0 {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityBlocking,
				Check:    "manifest_completeness",
				Location: artType,
				Message:  fmt.Sprintf("no canonical %s artifact found — cannot export", artType),
			})
		}
	}

	// Verify foundations exist.
	var foundationCount int
	db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_inputs WHERE project_id = ? AND role = 'foundation'`,
		projectID).Scan(&foundationCount)
	if foundationCount == 0 {
		report.Findings = append(report.Findings, Finding{
			Severity: SeverityBlocking,
			Check:    "manifest_completeness",
			Location: "foundations",
			Message:  "no foundation artifacts found — cannot export",
		})
	}
}
