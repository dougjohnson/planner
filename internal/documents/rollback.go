package documents

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/google/uuid"
)

// RollbackResult contains the outcome of a rollback operation.
type RollbackResult struct {
	NewArtifactID string `json:"new_artifact_id"`
	VersionLabel  string `json:"version_label"`
	RolledBackTo  string `json:"rolled_back_to"` // source artifact ID
}

// Rollback creates a new artifact referencing the same fragment versions as
// an earlier artifact, then promotes it to canonical. This preserves the
// append-only lineage model (§9.6) — rollback never deletes later history.
func Rollback(ctx context.Context, db *sql.DB, streamID, sourceArtifactID string) (*RollbackResult, error) {
	if streamID == "" {
		return nil, fmt.Errorf("stream_id is required")
	}
	if sourceArtifactID == "" {
		return nil, fmt.Errorf("source_artifact_id is required")
	}

	// Step 1: Verify source artifact exists and get its metadata.
	var projectID, artifactType, sourceLabel string
	err := db.QueryRowContext(ctx,
		"SELECT project_id, artifact_type, version_label FROM artifacts WHERE id = ?",
		sourceArtifactID,
	).Scan(&projectID, &artifactType, &sourceLabel)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("%w: %s", ErrArtifactNotFound, sourceArtifactID)
		}
		return nil, fmt.Errorf("looking up source artifact: %w", err)
	}

	// Step 2: Get fragment versions from the source artifact.
	rows, err := db.QueryContext(ctx,
		`SELECT fragment_version_id, position
		 FROM artifact_fragments
		 WHERE artifact_id = ?
		 ORDER BY position ASC`,
		sourceArtifactID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying source fragment versions: %w", err)
	}
	defer rows.Close()

	var versionIDs []string
	for rows.Next() {
		var fvID string
		var pos int
		if err := rows.Scan(&fvID, &pos); err != nil {
			return nil, fmt.Errorf("scanning fragment version: %w", err)
		}
		versionIDs = append(versionIDs, fvID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating fragment versions: %w", err)
	}

	if len(versionIDs) == 0 {
		return nil, fmt.Errorf("source artifact %s has no fragment versions", sourceArtifactID)
	}

	// Step 3: Create new artifact snapshot referencing the same versions.
	snapCreator := artifacts.NewSnapshotCreator(db)
	suffix := fmt.Sprintf(".rollback_from_%s", sourceLabel)

	snapResult, err := snapCreator.CreateSnapshot(ctx, artifacts.SnapshotInput{
		ProjectID:          projectID,
		ArtifactType:       artifactType,
		SourceStage:        "rollback",
		SourceModel:        "user",
		VersionSuffix:      suffix,
		FragmentVersionIDs: versionIDs,
		SourceArtifactIDs:  []string{sourceArtifactID},
		RelationType:       "rolled_back_to",
		IsCanonical:        false, // Will be promoted next.
	})
	if err != nil {
		return nil, fmt.Errorf("creating rollback artifact: %w", err)
	}

	// Step 4: Promote the rollback artifact to canonical.
	_, err = PromoteCanonical(ctx, db, streamID, snapResult.ArtifactID)
	if err != nil {
		return nil, fmt.Errorf("promoting rollback artifact: %w", err)
	}

	return &RollbackResult{
		NewArtifactID: snapResult.ArtifactID,
		VersionLabel:  snapResult.VersionLabel,
		RolledBackTo:  sourceArtifactID,
	}, nil
}

// RollbackEvent records a rollback in the workflow events table.
func RecordRollbackEvent(ctx context.Context, db *sql.DB, projectID, newArtifactID, sourceArtifactID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx,
		`INSERT INTO workflow_events (id, project_id, event_type, payload_json, created_at)
		 VALUES (?, ?, 'artifact:rollback', ?, ?)`,
		uuid.NewString(), projectID,
		fmt.Sprintf(`{"new_artifact_id":"%s","source_artifact_id":"%s"}`, newArtifactID, sourceArtifactID),
		now,
	)
	return err
}
