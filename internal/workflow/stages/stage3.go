// Package stages provides the stage-specific handlers for each step of the
// flywheel-planner workflow pipeline.
package stages

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/documents/decomposer"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
)

// Stage3Result contains the outcome of processing a Stage 3 submit_document call.
type Stage3Result struct {
	// ArtifactID is the created artifact.
	ArtifactID string
	// VersionLabel is the human-readable version label.
	VersionLabel string
	// FragmentCount is the number of fragments decomposed.
	FragmentCount int
	// NewFragments is the count of newly created fragments.
	NewFragments int
	// NewVersions is the count of new fragment versions.
	NewVersions int
}

// Stage3Handler processes submit_document tool calls from Stage 3
// (Parallel PRD Generation). Each model produces its own PRD artifact;
// none is promoted to canonical until Stage 4 synthesis.
type Stage3Handler struct {
	db              *sql.DB
	fragmentStore   *fragments.Store
	snapshotCreator *artifacts.SnapshotCreator
	artifactStore   *artifacts.Store
	logger          *slog.Logger
}

// NewStage3Handler creates a new Stage 3 handler.
func NewStage3Handler(db *sql.DB, fragStore *fragments.Store, snapCreator *artifacts.SnapshotCreator, artStore *artifacts.Store, logger *slog.Logger) *Stage3Handler {
	return &Stage3Handler{
		db:              db,
		fragmentStore:   fragStore,
		snapshotCreator: snapCreator,
		artifactStore:   artStore,
		logger:          logger,
	}
}

// HandleSubmitDocument processes a submitted PRD document from a model.
// It decomposes the markdown into fragments and creates a non-canonical artifact.
func (h *Stage3Handler) HandleSubmitDocument(ctx context.Context, projectID, sourceModel, workflowRunID string, content []byte) (*Stage3Result, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("submitted document content is empty")
	}

	// Step 1: Decompose the submitted markdown into fragments.
	dec := decomposer.New(h.fragmentStore)
	decResult, err := dec.Decompose(ctx, projectID, "prd", content, "stage-3", workflowRunID)
	if err != nil {
		return nil, fmt.Errorf("decomposing submitted document: %w", err)
	}

	h.logger.Info("stage 3: document decomposed",
		"project_id", projectID,
		"model", sourceModel,
		"fragments", len(decResult.Fragments),
		"new_fragments", decResult.NewFragments,
		"new_versions", decResult.NewVersions,
		"reused_versions", decResult.ReusedVersions,
	)

	// Step 2: Collect fragment version IDs for the artifact snapshot.
	versionIDs := make([]string, len(decResult.Versions))
	for i, v := range decResult.Versions {
		versionIDs[i] = v.ID
	}

	// Step 3: Create a non-canonical artifact snapshot.
	// Stage 3 artifacts are NOT promoted — Stage 4 synthesis will choose and merge.
	snapResult, err := h.snapshotCreator.CreateSnapshot(ctx, artifacts.SnapshotInput{
		ProjectID:          projectID,
		ArtifactType:       "prd",
		SourceStage:        "stage-3",
		SourceModel:        sourceModel,
		VersionSuffix:      ".generated",
		FragmentVersionIDs: versionIDs,
		IsCanonical:        false, // Explicitly not canonical until Stage 4.
	})
	if err != nil {
		return nil, fmt.Errorf("creating artifact snapshot: %w", err)
	}

	h.logger.Info("stage 3: artifact created",
		"project_id", projectID,
		"model", sourceModel,
		"artifact_id", snapResult.ArtifactID,
		"version_label", snapResult.VersionLabel,
	)

	// Step 4: Persist raw provider response to project's raw/ directory.
	rawPath := fmt.Sprintf("projects/%s/raw/stage3-%s-%s.md", projectID, sourceModel, workflowRunID)
	if h.artifactStore != nil {
		if _, err := h.artifactStore.WriteFile(rawPath, content); err != nil {
			h.logger.Warn("stage 3: failed to persist raw response",
				"path", rawPath, "error", err)
			// Non-fatal — the fragment-backed artifact is the important output.
		}
	}

	return &Stage3Result{
		ArtifactID:    snapResult.ArtifactID,
		VersionLabel:  snapResult.VersionLabel,
		FragmentCount: len(decResult.Fragments),
		NewFragments:  decResult.NewFragments,
		NewVersions:   decResult.NewVersions,
	}, nil
}
