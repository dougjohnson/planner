package stages

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/artifacts/lineage"
	"github.com/dougflynn/flywheel-planner/internal/documents/composer"
	"github.com/dougflynn/flywheel-planner/internal/documents/decomposer"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

// Stage4Result holds the outcome of Stage 4 synthesis.
type Stage4Result struct {
	SynthesisArtifactID string `json:"synthesis_artifact_id"`
	VersionLabel        string `json:"version_label"`
	PromotedToCanonical bool   `json:"promoted_to_canonical"`
	FragmentCount       int    `json:"fragment_count"`
}

// Stage4Handler implements Stage 4: GPT PRD Synthesis.
// It takes all Stage 3 PRD artifacts, sends them to GPT for synthesis,
// decomposes the result, creates an artifact, and promotes it to canonical.
type Stage4Handler struct {
	db              *sql.DB
	fragmentStore   *fragments.Store
	snapshotCreator *artifacts.SnapshotCreator
	lineageService  *lineage.Service
	composer        *composer.Composer
	artifactStore   *artifacts.Store
	logger          *slog.Logger
}

// NewStage4Handler creates a new Stage 4 handler.
func NewStage4Handler(
	db *sql.DB,
	fragStore *fragments.Store,
	snapCreator *artifacts.SnapshotCreator,
	lineageSvc *lineage.Service,
	comp *composer.Composer,
	artStore *artifacts.Store,
	logger *slog.Logger,
) *Stage4Handler {
	return &Stage4Handler{
		db:              db,
		fragmentStore:   fragStore,
		snapshotCreator: snapCreator,
		lineageService:  lineageSvc,
		composer:        comp,
		artifactStore:   artStore,
		logger:          logger,
	}
}

// HandleSynthesisSubmission processes the synthesized PRD from Stage 4.
// It decomposes the document, creates an artifact, records lineage from
// all Stage 3 artifacts, and promotes the result to canonical.
func (h *Stage4Handler) HandleSynthesisSubmission(
	ctx context.Context,
	projectID string,
	workflowRunID string,
	submittedContent []byte,
	sourceArtifactIDs []string, // Stage 3 artifact IDs
	rawPayload []byte,
) (*Stage4Result, error) {
	if len(submittedContent) == 0 {
		return nil, fmt.Errorf("submitted synthesis content is empty")
	}

	// Step 1: Persist raw provider response.
	if len(rawPayload) > 0 && h.artifactStore != nil {
		rawPath := fmt.Sprintf("projects/%s/raw/stage4-synthesis-%s.json", projectID, workflowRunID)
		if _, err := h.artifactStore.WriteFile(rawPath, rawPayload); err != nil {
			h.logger.Warn("stage 4: failed to persist raw response", "error", err)
		}
	}

	// Step 2: Decompose the synthesized PRD into fragments.
	dec := decomposer.New(h.fragmentStore)
	decResult, err := dec.Decompose(ctx, projectID, "prd", submittedContent, "stage-4", workflowRunID)
	if err != nil {
		return nil, fmt.Errorf("decomposing synthesis: %w", err)
	}

	h.logger.Info("stage 4: synthesis decomposed",
		"project_id", projectID,
		"fragments", len(decResult.Fragments),
		"new_versions", decResult.NewVersions,
	)

	// Step 3: Create artifact snapshot — marked as canonical.
	versionIDs := make([]string, len(decResult.Versions))
	for i, v := range decResult.Versions {
		versionIDs[i] = v.ID
	}

	snapResult, err := h.snapshotCreator.CreateSnapshot(ctx, artifacts.SnapshotInput{
		ProjectID:          projectID,
		ArtifactType:       "prd",
		SourceStage:        "stage-4",
		SourceModel:        string(models.ProviderOpenAI),
		VersionSuffix:      ".synthesized",
		FragmentVersionIDs: versionIDs,
		SourceArtifactIDs:  sourceArtifactIDs,
		RelationType:       string(lineage.SynthesizedFrom),
		IsCanonical:        true, // Stage 4 output IS the canonical PRD.
	})
	if err != nil {
		return nil, fmt.Errorf("creating synthesis artifact: %w", err)
	}

	h.logger.Info("stage 4: synthesis artifact created and promoted to canonical",
		"artifact_id", snapResult.ArtifactID,
		"version_label", snapResult.VersionLabel,
	)

	// Step 4: Update stream_heads to point to this artifact.
	// The canonical promotion is handled by the SnapshotCreator's IsCanonical flag
	// and the stream_heads update will be done by the canonical promotion service.

	return &Stage4Result{
		SynthesisArtifactID: snapResult.ArtifactID,
		VersionLabel:        snapResult.VersionLabel,
		PromotedToCanonical: true,
		FragmentCount:       len(decResult.Fragments),
	}, nil
}
