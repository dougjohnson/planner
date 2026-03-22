// Package composer reconstructs complete markdown documents from versioned
// fragments stored in the database. The composition pipeline queries the
// artifact_fragments junction table, orders by position, and concatenates
// heading + content for each fragment version.
package composer

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// composedFragment holds the data needed to reconstruct one section of a document.
type composedFragment struct {
	Heading  string
	Depth    int
	Content  string
	Position int
}

// Composer reconstructs markdown documents from fragment versions.
type Composer struct {
	db *sql.DB
}

// New creates a new Composer backed by the given database.
func New(db *sql.DB) *Composer {
	return &Composer{db: db}
}

// Compose reconstructs a complete markdown document for the given artifact ID.
// It queries the artifact_fragments junction table, resolves each fragment version's
// content, and concatenates them in position order with appropriate headings.
func (c *Composer) Compose(ctx context.Context, artifactID string) (string, error) {
	frags, err := c.queryFragments(ctx, artifactID)
	if err != nil {
		return "", err
	}

	if len(frags) == 0 {
		return "", nil
	}

	var b strings.Builder
	for i, f := range frags {
		if i > 0 {
			b.WriteString("\n")
		}

		// Write heading (## at the fragment's depth level).
		if f.Heading != "" {
			prefix := strings.Repeat("#", f.Depth)
			b.WriteString(prefix)
			b.WriteString(" ")
			b.WriteString(f.Heading)
			b.WriteString("\n\n")
		}

		// Write content.
		content := strings.TrimRight(f.Content, "\n")
		if content != "" {
			b.WriteString(content)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}

// ComposeWithAnnotations is like Compose but adds HTML comment annotations
// with fragment IDs before each section, useful for model context and debugging.
func (c *Composer) ComposeWithAnnotations(ctx context.Context, artifactID string) (string, error) {
	type annotatedFrag struct {
		composedFragment
		FragmentID        string
		FragmentVersionID string
	}

	// Query fragments with IDs for annotations.
	rows, err := c.db.QueryContext(ctx, `
		SELECT f.id, fv.id, f.heading, f.depth, fv.content, af.position
		FROM artifact_fragments af
		JOIN fragment_versions fv ON fv.id = af.fragment_version_id
		JOIN fragments f ON f.id = fv.fragment_id
		WHERE af.artifact_id = ?
		ORDER BY af.position ASC
	`, artifactID)
	if err != nil {
		return "", fmt.Errorf("querying annotated fragments: %w", err)
	}
	defer rows.Close()

	var annotated []annotatedFrag
	for rows.Next() {
		var a annotatedFrag
		if err := rows.Scan(&a.FragmentID, &a.FragmentVersionID, &a.Heading, &a.Depth, &a.Content, &a.Position); err != nil {
			return "", fmt.Errorf("scanning annotated fragment: %w", err)
		}
		annotated = append(annotated, a)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	if len(annotated) == 0 {
		return "", nil
	}

	var b strings.Builder
	for i, a := range annotated {
		if i > 0 {
			b.WriteString("\n")
		}

		// Write annotation comment.
		fmt.Fprintf(&b, "<!-- fragment:%s version:%s -->\n", a.FragmentID, a.FragmentVersionID)

		if a.Heading != "" {
			prefix := strings.Repeat("#", a.Depth)
			b.WriteString(prefix)
			b.WriteString(" ")
			b.WriteString(a.Heading)
			b.WriteString("\n\n")
		}

		content := strings.TrimRight(a.Content, "\n")
		if content != "" {
			b.WriteString(content)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}

// queryFragments loads the fragment data for an artifact, ordered by position.
func (c *Composer) queryFragments(ctx context.Context, artifactID string) ([]composedFragment, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT f.heading, f.depth, fv.content, af.position
		FROM artifact_fragments af
		JOIN fragment_versions fv ON fv.id = af.fragment_version_id
		JOIN fragments f ON f.id = fv.fragment_id
		WHERE af.artifact_id = ?
		ORDER BY af.position ASC
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("querying artifact fragments: %w", err)
	}
	defer rows.Close()

	var frags []composedFragment
	for rows.Next() {
		var f composedFragment
		if err := rows.Scan(&f.Heading, &f.Depth, &f.Content, &f.Position); err != nil {
			return nil, fmt.Errorf("scanning composed fragment: %w", err)
		}
		frags = append(frags, f)
	}
	return frags, rows.Err()
}
