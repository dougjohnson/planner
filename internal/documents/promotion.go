// Package documents provides document lifecycle operations for flywheel-planner,
// including canonical promotion and stream management.
package documents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrArtifactNotFound is returned when the target artifact does not exist.
	ErrArtifactNotFound = errors.New("artifact not found")

	// ErrStreamNotFound is returned when the target stream does not exist.
	ErrStreamNotFound = errors.New("document stream not found")

	// ErrArtifactStreamMismatch is returned when the artifact does not belong
	// to the expected project/stream.
	ErrArtifactStreamMismatch = errors.New("artifact does not match the target stream")
)

// PromotionResult contains the outcome of a canonical promotion.
type PromotionResult struct {
	// StreamID is the document stream that was updated.
	StreamID string
	// NewCanonicalID is the artifact that is now canonical.
	NewCanonicalID string
	// PreviousCanonicalID is the artifact that was previously canonical (empty if none).
	PreviousCanonicalID string
	// PromotedAt is the timestamp of the promotion.
	PromotedAt time.Time
}

// PromoteCanonical atomically advances the canonical artifact pointer for a
// document stream. In a single transaction it:
//  1. Updates stream_heads to point to the new artifact
//  2. Clears is_canonical on the previous canonical artifact
//  3. Sets is_canonical on the new artifact
//
// Both stream_heads and artifacts.is_canonical are updated together to maintain
// dual-truth consistency (§9.6). If any step fails, the entire transaction is
// rolled back and no changes are persisted.
//
// The transaction is deliberately narrow: no provider calls, filesystem I/O,
// or network operations occur within it.
func PromoteCanonical(ctx context.Context, db *sql.DB, streamID, newArtifactID string) (*PromotionResult, error) {
	if streamID == "" {
		return nil, fmt.Errorf("stream ID is required")
	}
	if newArtifactID == "" {
		return nil, fmt.Errorf("artifact ID is required")
	}

	now := time.Now().UTC()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning promotion transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 0: Verify the stream exists and get the project context.
	var projectID, streamType string
	err = tx.QueryRowContext(ctx,
		"SELECT project_id, stream_type FROM document_streams WHERE id = ?",
		streamID,
	).Scan(&projectID, &streamType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrStreamNotFound, streamID)
	}
	if err != nil {
		return nil, fmt.Errorf("looking up stream: %w", err)
	}

	// Step 1: Verify the new artifact exists and belongs to the right project.
	var artifactProjectID, artifactType string
	err = tx.QueryRowContext(ctx,
		"SELECT project_id, artifact_type FROM artifacts WHERE id = ?",
		newArtifactID,
	).Scan(&artifactProjectID, &artifactType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrArtifactNotFound, newArtifactID)
	}
	if err != nil {
		return nil, fmt.Errorf("looking up artifact: %w", err)
	}
	if artifactProjectID != projectID {
		return nil, fmt.Errorf("%w: artifact project %s != stream project %s",
			ErrArtifactStreamMismatch, artifactProjectID, projectID)
	}

	// Step 2: Find the current canonical artifact (if any) via stream_heads.
	var previousCanonicalID string
	err = tx.QueryRowContext(ctx,
		"SELECT artifact_id FROM stream_heads WHERE stream_id = ?",
		streamID,
	).Scan(&previousCanonicalID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("reading current stream head: %w", err)
	}

	// Step 3: Clear is_canonical on ALL artifacts for this project+type
	// (defensive — should be at most one, but ensures consistency).
	_, err = tx.ExecContext(ctx,
		`UPDATE artifacts SET is_canonical = 0
		 WHERE project_id = ? AND artifact_type = ? AND is_canonical = 1`,
		projectID, artifactType,
	)
	if err != nil {
		return nil, fmt.Errorf("clearing previous canonical flag: %w", err)
	}

	// Step 4: Set is_canonical on the new artifact.
	res, err := tx.ExecContext(ctx,
		"UPDATE artifacts SET is_canonical = 1 WHERE id = ?",
		newArtifactID,
	)
	if err != nil {
		return nil, fmt.Errorf("setting new canonical flag: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking canonical update: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("%w: %s (update affected 0 rows)", ErrArtifactNotFound, newArtifactID)
	}

	// Step 5: Update or insert stream_heads to point to the new artifact.
	nowStr := now.Format(time.RFC3339)
	if previousCanonicalID != "" {
		// Update existing stream head.
		_, err = tx.ExecContext(ctx,
			"UPDATE stream_heads SET artifact_id = ?, updated_at = ? WHERE stream_id = ?",
			newArtifactID, nowStr, streamID,
		)
	} else {
		// Insert new stream head (first promotion for this stream).
		_, err = tx.ExecContext(ctx,
			"INSERT INTO stream_heads (stream_id, artifact_id, updated_at) VALUES (?, ?, ?)",
			streamID, newArtifactID, nowStr,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("updating stream head: %w", err)
	}

	// Commit the transaction.
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing promotion transaction: %w", err)
	}

	return &PromotionResult{
		StreamID:            streamID,
		NewCanonicalID:      newArtifactID,
		PreviousCanonicalID: previousCanonicalID,
		PromotedAt:          now,
	}, nil
}

// GetCanonicalArtifactID returns the current canonical artifact ID for a stream.
// Returns ErrStreamNotFound if no stream head exists.
func GetCanonicalArtifactID(ctx context.Context, db *sql.DB, streamID string) (string, error) {
	var artifactID string
	err := db.QueryRowContext(ctx,
		"SELECT artifact_id FROM stream_heads WHERE stream_id = ?",
		streamID,
	).Scan(&artifactID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: no canonical artifact for stream %s", ErrStreamNotFound, streamID)
	}
	if err != nil {
		return "", fmt.Errorf("reading stream head: %w", err)
	}
	return artifactID, nil
}
