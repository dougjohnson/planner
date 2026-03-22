// Package foundations manages foundation artifact intake, assembly,
// and provenance tracking for Stage 1.
package foundations

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ProvenanceType classifies the origin of a foundation artifact.
type ProvenanceType string

const (
	ProvenanceBuiltIn     ProvenanceType = "built_in_template" // AGENTS.md, built-in guides
	ProvenanceUserUpload  ProvenanceType = "user_upload"       // custom guides
	ProvenanceUserText    ProvenanceType = "user_text_entry"   // pasted text
	ProvenanceGenerated   ProvenanceType = "generated"         // tech stack file, architecture direction
)

// FoundationInput represents a foundation artifact with its provenance.
type FoundationInput struct {
	ID             string         `json:"id"`
	ProjectID      string         `json:"project_id"`
	Role           string         `json:"role"`
	SourceType     ProvenanceType `json:"source_type"`
	ContentPath    string         `json:"content_path"`
	OriginalName   string         `json:"original_filename,omitempty"`
	CreatedAt      string         `json:"created_at"`
}

// RecordFoundationInput creates a project_input with provenance tagging.
func RecordFoundationInput(ctx context.Context, db *sql.DB, projectID, role string, provenance ProvenanceType, contentPath, originalName string) (*FoundationInput, error) {
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := db.ExecContext(ctx,
		`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, original_filename, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, role, string(provenance), contentPath, originalName, now, now)
	if err != nil {
		return nil, fmt.Errorf("recording foundation input: %w", err)
	}

	return &FoundationInput{
		ID:           id,
		ProjectID:    projectID,
		Role:         role,
		SourceType:   provenance,
		ContentPath:  contentPath,
		OriginalName: originalName,
		CreatedAt:    now,
	}, nil
}

// ListFoundationInputs returns all foundation artifacts for a project with provenance.
func ListFoundationInputs(ctx context.Context, db *sql.DB, projectID string) ([]FoundationInput, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, project_id, role, source_type, content_path, original_filename, created_at
		 FROM project_inputs WHERE project_id = ? AND role = 'foundation'
		 ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing foundation inputs: %w", err)
	}
	defer rows.Close()

	var inputs []FoundationInput
	for rows.Next() {
		var fi FoundationInput
		var sourceType string
		if err := rows.Scan(&fi.ID, &fi.ProjectID, &fi.Role, &sourceType,
			&fi.ContentPath, &fi.OriginalName, &fi.CreatedAt); err != nil {
			return nil, err
		}
		fi.SourceType = ProvenanceType(sourceType)
		inputs = append(inputs, fi)
	}
	return inputs, rows.Err()
}

// GetProvenanceSummary returns a map of provenance type to count for a project.
func GetProvenanceSummary(ctx context.Context, db *sql.DB, projectID string) (map[ProvenanceType]int, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT source_type, COUNT(*) FROM project_inputs
		 WHERE project_id = ? AND role = 'foundation'
		 GROUP BY source_type`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := make(map[ProvenanceType]int)
	for rows.Next() {
		var sourceType string
		var count int
		if err := rows.Scan(&sourceType, &count); err != nil {
			return nil, err
		}
		summary[ProvenanceType(sourceType)] = count
	}
	return summary, rows.Err()
}
