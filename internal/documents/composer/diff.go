package composer

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// FragmentDiffEntry represents a single fragment's diff status.
type FragmentDiffEntry struct {
	FragmentID string `json:"fragment_id"`
	Heading    string `json:"heading"`
	Status     string `json:"status"` // "added", "removed", "modified", "unchanged"
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
	LineDiff   string `json:"line_diff,omitempty"` // unified-style line diff for modified
}

// FragmentDiffResult contains the full fragment-level diff between two artifacts.
type FragmentDiffResult struct {
	ArtifactA string              `json:"artifact_a"`
	ArtifactB string              `json:"artifact_b"`
	Entries   []FragmentDiffEntry `json:"entries"`
	Summary   DiffSummary         `json:"summary"`
}

// DiffSummary counts changes by type.
type DiffSummary struct {
	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Modified  int `json:"modified"`
	Unchanged int `json:"unchanged"`
}

// ComposedDiffResult contains a unified diff of the composed markdown.
type ComposedDiffResult struct {
	ArtifactA string `json:"artifact_a"`
	ArtifactB string `json:"artifact_b"`
	UnifiedDiff string `json:"unified_diff"`
}

// fragmentInfo holds resolved fragment data for diff comparison.
type fragmentInfo struct {
	FragmentID        string
	FragmentVersionID string
	Heading           string
	Content           string
	Position          int
}

// DiffEngine computes diffs between artifact snapshots.
type DiffEngine struct {
	db *sql.DB
	c  *Composer
}

// NewDiffEngine creates a new DiffEngine.
func NewDiffEngine(db *sql.DB) *DiffEngine {
	return &DiffEngine{
		db: db,
		c:  New(db),
	}
}

// FragmentDiff compares two artifacts at the fragment level.
func (d *DiffEngine) FragmentDiff(ctx context.Context, artifactA, artifactB string) (*FragmentDiffResult, error) {
	fragsA, err := d.loadFragments(ctx, artifactA)
	if err != nil {
		return nil, fmt.Errorf("loading artifact A fragments: %w", err)
	}
	fragsB, err := d.loadFragments(ctx, artifactB)
	if err != nil {
		return nil, fmt.Errorf("loading artifact B fragments: %w", err)
	}

	// Index by fragment ID.
	mapA := make(map[string]fragmentInfo)
	for _, f := range fragsA {
		mapA[f.FragmentID] = f
	}
	mapB := make(map[string]fragmentInfo)
	for _, f := range fragsB {
		mapB[f.FragmentID] = f
	}

	var entries []FragmentDiffEntry
	var summary DiffSummary

	// Check fragments in A.
	seen := make(map[string]bool)
	for _, fa := range fragsA {
		seen[fa.FragmentID] = true
		fb, inB := mapB[fa.FragmentID]
		if !inB {
			entries = append(entries, FragmentDiffEntry{
				FragmentID: fa.FragmentID,
				Heading:    fa.Heading,
				Status:     "removed",
				OldContent: fa.Content,
			})
			summary.Removed++
		} else if fa.FragmentVersionID != fb.FragmentVersionID {
			entries = append(entries, FragmentDiffEntry{
				FragmentID: fa.FragmentID,
				Heading:    fa.Heading,
				Status:     "modified",
				OldContent: fa.Content,
				NewContent: fb.Content,
				LineDiff:   simpleLineDiff(fa.Content, fb.Content),
			})
			summary.Modified++
		} else {
			entries = append(entries, FragmentDiffEntry{
				FragmentID: fa.FragmentID,
				Heading:    fa.Heading,
				Status:     "unchanged",
			})
			summary.Unchanged++
		}
	}

	// Check fragments only in B (added).
	for _, fb := range fragsB {
		if !seen[fb.FragmentID] {
			entries = append(entries, FragmentDiffEntry{
				FragmentID: fb.FragmentID,
				Heading:    fb.Heading,
				Status:     "added",
				NewContent: fb.Content,
			})
			summary.Added++
		}
	}

	return &FragmentDiffResult{
		ArtifactA: artifactA,
		ArtifactB: artifactB,
		Entries:   entries,
		Summary:   summary,
	}, nil
}

// ComposedDiff composes both artifacts to markdown and computes a unified diff.
func (d *DiffEngine) ComposedDiff(ctx context.Context, artifactA, artifactB string) (*ComposedDiffResult, error) {
	textA, err := d.c.Compose(ctx, artifactA)
	if err != nil {
		return nil, fmt.Errorf("composing artifact A: %w", err)
	}
	textB, err := d.c.Compose(ctx, artifactB)
	if err != nil {
		return nil, fmt.Errorf("composing artifact B: %w", err)
	}

	return &ComposedDiffResult{
		ArtifactA:   artifactA,
		ArtifactB:   artifactB,
		UnifiedDiff: simpleLineDiff(textA, textB),
	}, nil
}

// loadFragments loads fragment info for an artifact from the junction table.
func (d *DiffEngine) loadFragments(ctx context.Context, artifactID string) ([]fragmentInfo, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT f.id, fv.id, f.heading, fv.content, af.position
		FROM artifact_fragments af
		JOIN fragment_versions fv ON fv.id = af.fragment_version_id
		JOIN fragments f ON f.id = fv.fragment_id
		WHERE af.artifact_id = ?
		ORDER BY af.position ASC
	`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var frags []fragmentInfo
	for rows.Next() {
		var fi fragmentInfo
		if err := rows.Scan(&fi.FragmentID, &fi.FragmentVersionID, &fi.Heading, &fi.Content, &fi.Position); err != nil {
			return nil, err
		}
		frags = append(frags, fi)
	}
	return frags, rows.Err()
}

// simpleLineDiff produces a basic line-by-line diff showing added/removed lines.
func simpleLineDiff(a, b string) string {
	linesA := strings.Split(a, "\n")
	linesB := strings.Split(b, "\n")

	var diff strings.Builder

	// Simple LCS-based diff would be ideal, but for now use a basic approach:
	// mark lines unique to A as removed, lines unique to B as added.
	setA := make(map[string]int)
	for _, l := range linesA {
		setA[l]++
	}
	setB := make(map[string]int)
	for _, l := range linesB {
		setB[l]++
	}

	for _, l := range linesA {
		if setB[l] <= 0 {
			diff.WriteString("- " + l + "\n")
		} else {
			setB[l]--
		}
	}

	// Reset setA for added lines check.
	setA2 := make(map[string]int)
	for _, l := range linesA {
		setA2[l]++
	}
	for _, l := range linesB {
		if setA2[l] <= 0 {
			diff.WriteString("+ " + l + "\n")
		} else {
			setA2[l]--
		}
	}

	return diff.String()
}
