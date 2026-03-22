package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/google/uuid"
)

// FragmentOperation represents a validated tool-call operation on a fragment.
type FragmentOperation struct {
	Type            string `json:"type"` // "update", "add", "remove"
	FragmentID      string `json:"fragment_id,omitempty"`
	AfterFragmentID string `json:"after_fragment_id,omitempty"`
	Heading         string `json:"heading,omitempty"`
	NewContent      string `json:"new_content,omitempty"`
	Rationale       string `json:"rationale,omitempty"`
}

// CommitResult holds the outcome of applying fragment operations.
type CommitResult struct {
	ArtifactID     string `json:"artifact_id"`
	VersionLabel   string `json:"version_label"`
	UpdateCount    int    `json:"update_count"`
	AddCount       int    `json:"add_count"`
	RemoveCount    int    `json:"remove_count"`
	UnchangedCount int    `json:"unchanged_count"`
	NoChanges      bool   `json:"no_changes"`
}

// CommitFragmentOperations applies a set of fragment operations from a review
// pass (Stage 7/14) to the canonical artifact and promotes the result.
// This is the core of Stage 8 / Stage 15.
func CommitFragmentOperations(
	ctx context.Context,
	db *sql.DB,
	projectID string,
	canonicalArtifactID string,
	ops []FragmentOperation,
	sourceStage string,
	sourceRunID string,
	documentType string,
) (*CommitResult, error) {
	store := fragments.NewStore(db)

	// If zero operations, classify as no_changes_proposed.
	if len(ops) == 0 {
		return &CommitResult{
			ArtifactID: canonicalArtifactID,
			NoChanges:  true,
		}, nil
	}

	// Load current canonical artifact's fragment versions.
	currentFragments, err := loadArtifactFragments(ctx, db, canonicalArtifactID)
	if err != nil {
		return nil, fmt.Errorf("loading canonical fragments: %w", err)
	}

	// Track which fragments to remove.
	removedIDs := make(map[string]bool)

	// Apply operations.
	result := &CommitResult{}
	// Map fragment_id → new version ID for updates.
	updatedVersions := make(map[string]string)

	// New fragments to insert (from add operations).
	type addedFragment struct {
		afterFragmentID string
		fragmentID      string
		versionID       string
	}
	var additions []addedFragment

	for _, op := range ops {
		switch op.Type {
		case "update":
			// Create new fragment version with the updated content.
			ver, err := store.CreateVersion(ctx, op.FragmentID, op.NewContent,
				sourceStage, sourceRunID, op.Rationale)
			if err != nil {
				return nil, fmt.Errorf("creating version for fragment %s: %w", op.FragmentID, err)
			}
			updatedVersions[op.FragmentID] = ver.ID
			result.UpdateCount++

		case "add":
			// Create new fragment + initial version.
			frag, err := store.CreateFragment(ctx, projectID, documentType, op.Heading, 2)
			if err != nil {
				return nil, fmt.Errorf("creating fragment for heading %q: %w", op.Heading, err)
			}
			ver, err := store.CreateVersion(ctx, frag.ID, op.NewContent,
				sourceStage, sourceRunID, op.Rationale)
			if err != nil {
				return nil, fmt.Errorf("creating version for new fragment %s: %w", frag.ID, err)
			}
			additions = append(additions, addedFragment{
				afterFragmentID: op.AfterFragmentID,
				fragmentID:      frag.ID,
				versionID:       ver.ID,
			})
			result.AddCount++

		case "remove":
			removedIDs[op.FragmentID] = true
			result.RemoveCount++
		}
	}

	// Build the new artifact's fragment list with correct positions.
	type artifactEntry struct {
		fragmentID string
		versionID  string
	}
	var newEntries []artifactEntry

	for _, cf := range currentFragments {
		if removedIDs[cf.fragmentID] {
			continue
		}

		// Use updated version if available, otherwise keep existing.
		versionID := cf.versionID
		if updated, ok := updatedVersions[cf.fragmentID]; ok {
			versionID = updated
		} else {
			result.UnchangedCount++
		}

		newEntries = append(newEntries, artifactEntry{
			fragmentID: cf.fragmentID,
			versionID:  versionID,
		})

		// Insert any additions that go after this fragment.
		for _, add := range additions {
			if add.afterFragmentID == cf.fragmentID {
				newEntries = append(newEntries, artifactEntry{
					fragmentID: add.fragmentID,
					versionID:  add.versionID,
				})
			}
		}
	}

	// Wrap artifact creation, junction population, and lineage in a transaction
	// so a crash can't leave partial state.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning commit transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the new artifact.
	artifactID := uuid.New().String()
	versionLabel, err := nextVersionLabel(ctx, db, projectID, documentType)
	if err != nil {
		return nil, fmt.Errorf("computing version label: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO artifacts (id, project_id, artifact_type, version_label, source_stage, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		artifactID, projectID, documentType, versionLabel, sourceStage, now)
	if err != nil {
		return nil, fmt.Errorf("creating artifact: %w", err)
	}

	// Populate the junction table.
	for pos, entry := range newEntries {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
			 VALUES (?, ?, ?)`,
			artifactID, entry.versionID, pos)
		if err != nil {
			return nil, fmt.Errorf("inserting artifact fragment at position %d: %w", pos, err)
		}
	}

	// Record lineage: new artifact was derived from the canonical.
	relationID := uuid.New().String()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO artifact_relations (id, artifact_id, related_artifact_id, relation_type, created_at)
		 VALUES (?, ?, ?, 'derived_from', ?)`,
		relationID, artifactID, canonicalArtifactID, now)
	if err != nil {
		return nil, fmt.Errorf("recording lineage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing artifact transaction: %w", err)
	}

	result.ArtifactID = artifactID
	result.VersionLabel = versionLabel
	return result, nil
}

// --- Helpers ---

type canonicalFragment struct {
	fragmentID string
	versionID  string
	position   int
}

func loadArtifactFragments(ctx context.Context, db *sql.DB, artifactID string) ([]canonicalFragment, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT fv.fragment_id, af.fragment_version_id, af.position
		 FROM artifact_fragments af
		 JOIN fragment_versions fv ON fv.id = af.fragment_version_id
		 WHERE af.artifact_id = ?
		 ORDER BY af.position ASC`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var frags []canonicalFragment
	for rows.Next() {
		var f canonicalFragment
		if err := rows.Scan(&f.fragmentID, &f.versionID, &f.position); err != nil {
			return nil, err
		}
		frags = append(frags, f)
	}
	return frags, rows.Err()
}

func nextVersionLabel(ctx context.Context, db *sql.DB, projectID, documentType string) (string, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifacts WHERE project_id = ? AND artifact_type = ?`,
		projectID, documentType).Scan(&count)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.v%02d", documentType, count+1), nil
}
