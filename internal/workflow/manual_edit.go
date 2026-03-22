package workflow

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
)

// ManualEditResult holds the outcome of a user manual fragment edit.
type ManualEditResult struct {
	FragmentID      string `json:"fragment_id"`
	NewVersionID    string `json:"new_version_id"`
	PreviousContent string `json:"previous_content"`
	NewContent      string `json:"new_content"`
}

// ApplyManualFragmentEdit creates a new fragment version tagged as a user
// manual edit. This is the checkpoint-only escape hatch for factual fixes,
// terminology corrections, and model-resistant edits (§3 Decisions).
//
// The edit creates a new fragment_version with source_stage="user_manual_edit"
// and remains fully inspectable in lineage views.
func ApplyManualFragmentEdit(
	ctx context.Context,
	db *sql.DB,
	fragmentID string,
	newContent string,
	rationale string,
) (*ManualEditResult, error) {
	store := fragments.NewStore(db)

	// Verify fragment exists.
	frag, err := store.GetFragment(ctx, fragmentID)
	if err != nil {
		return nil, fmt.Errorf("fragment %s not found: %w", fragmentID, err)
	}
	_ = frag

	// Get current version for the diff record.
	currentVersion, err := store.LatestVersion(ctx, fragmentID)
	if err != nil {
		return nil, fmt.Errorf("loading current version: %w", err)
	}

	// Don't create a new version if content hasn't changed.
	if currentVersion.Content == newContent {
		return &ManualEditResult{
			FragmentID:      fragmentID,
			NewVersionID:    currentVersion.ID,
			PreviousContent: currentVersion.Content,
			NewContent:      newContent,
		}, nil
	}

	// Create new version tagged as manual edit.
	newVersion, err := store.CreateVersion(ctx, fragmentID, newContent,
		"user_manual_edit", "", rationale)
	if err != nil {
		return nil, fmt.Errorf("creating manual edit version: %w", err)
	}

	return &ManualEditResult{
		FragmentID:      fragmentID,
		NewVersionID:    newVersion.ID,
		PreviousContent: currentVersion.Content,
		NewContent:      newContent,
	}, nil
}
