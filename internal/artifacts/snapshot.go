package artifacts

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SnapshotInput contains everything needed to create an artifact snapshot.
type SnapshotInput struct {
	ProjectID          string
	ArtifactType       string // "prd" or "plan"
	SourceStage        string
	SourceModel        string
	VersionSuffix      string   // e.g. ".seed", ".synthesized", ".integrated"
	FragmentVersionIDs []string // ordered by position
	SourceArtifactIDs  []string // for lineage recording
	RelationType       string   // e.g. "synthesized_from", "integrated_from"
	IsCanonical        bool
}

// SnapshotResult contains the created artifact metadata.
type SnapshotResult struct {
	ArtifactID   string
	VersionLabel string
}

// SnapshotCreator creates artifact snapshots via the junction table.
type SnapshotCreator struct {
	db *sql.DB
}

// NewSnapshotCreator creates a new SnapshotCreator.
func NewSnapshotCreator(db *sql.DB) *SnapshotCreator {
	return &SnapshotCreator{db: db}
}

// CreateSnapshot creates an artifact row, populates the artifact_fragments
// junction table, and optionally records lineage relations.
func (sc *SnapshotCreator) CreateSnapshot(ctx context.Context, input SnapshotInput) (*SnapshotResult, error) {
	if len(input.FragmentVersionIDs) == 0 {
		return nil, fmt.Errorf("at least one fragment version ID is required")
	}

	artifactID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := sc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Compute version label INSIDE the transaction to prevent race conditions
	// where concurrent snapshot creations could get the same count.
	versionLabel, err := sc.nextVersionLabelTx(ctx, tx, input.ProjectID, input.ArtifactType, input.VersionSuffix)
	if err != nil {
		return nil, fmt.Errorf("computing version label: %w", err)
	}

	// If promoting to canonical, clear is_canonical on all existing artifacts
	// for this project+type first (within the same transaction).
	canonical := 0
	if input.IsCanonical {
		canonical = 1
		_, err = tx.ExecContext(ctx, `
			UPDATE artifacts SET is_canonical = 0
			WHERE project_id = ? AND artifact_type = ? AND is_canonical = 1
		`, input.ProjectID, input.ArtifactType)
		if err != nil {
			return nil, fmt.Errorf("clearing previous canonical: %w", err)
		}
	}

	// Create artifact row.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO artifacts (id, project_id, artifact_type, version_label, source_stage, source_model, is_canonical, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, artifactID, input.ProjectID, input.ArtifactType, versionLabel, input.SourceStage, input.SourceModel, canonical, now)
	if err != nil {
		return nil, fmt.Errorf("inserting artifact: %w", err)
	}

	// Populate artifact_fragments junction.
	for pos, fvID := range input.FragmentVersionIDs {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
			VALUES (?, ?, ?)
		`, artifactID, fvID, pos)
		if err != nil {
			return nil, fmt.Errorf("inserting artifact_fragment at position %d: %w", pos, err)
		}
	}

	// Record lineage relations.
	if len(input.SourceArtifactIDs) > 0 && input.RelationType != "" {
		for _, sourceID := range input.SourceArtifactIDs {
			relID := uuid.NewString()
			_, err := tx.ExecContext(ctx, `
				INSERT INTO artifact_relations (id, artifact_id, related_artifact_id, relation_type, created_at)
				VALUES (?, ?, ?, ?, ?)
			`, relID, artifactID, sourceID, input.RelationType, now)
			if err != nil {
				return nil, fmt.Errorf("inserting lineage relation: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &SnapshotResult{
		ArtifactID:   artifactID,
		VersionLabel: versionLabel,
	}, nil
}

// nextVersionLabelTx computes the next version label INSIDE a transaction
// to prevent race conditions with concurrent snapshot creations.
func (sc *SnapshotCreator) nextVersionLabelTx(ctx context.Context, tx *sql.Tx, projectID, artifactType, suffix string) (string, error) {
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM artifacts WHERE project_id = ? AND artifact_type = ?
	`, projectID, artifactType).Scan(&count)
	if err != nil {
		return "", err
	}

	version := count + 1
	label := fmt.Sprintf("%s.v%02d", artifactType, version)
	if suffix != "" {
		label += suffix
	}
	return label, nil
}
