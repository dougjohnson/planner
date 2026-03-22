package stages

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/artifacts/lineage"
	"github.com/dougflynn/flywheel-planner/internal/documents/composer"
	"github.com/dougflynn/flywheel-planner/internal/documents/decomposer"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

// Stage11Result holds the outcome of Stage 11 plan synthesis.
type Stage11Result struct {
	SynthesisArtifactID string `json:"synthesis_artifact_id"`
	VersionLabel        string `json:"version_label"`
	PromotedToCanonical bool   `json:"promoted_to_canonical"`
	FragmentCount       int    `json:"fragment_count"`
}

// Stage11Handler implements Stage 11: GPT Plan Synthesis.
// Mirrors Stage 4 but operates on the Plan document stream.
// Takes all Stage 10 plan artifacts, synthesizes via GPT, decomposes,
// and promotes the result to canonical for the plan stream.
type Stage11Handler struct {
	fragmentStore   *fragments.Store
	snapshotCreator *artifacts.SnapshotCreator
	composer        *composer.Composer
	artifactStore   *artifacts.Store
	logger          *slog.Logger
}

// NewStage11Handler creates a new Stage 11 handler.
func NewStage11Handler(
	fragStore *fragments.Store,
	snapCreator *artifacts.SnapshotCreator,
	comp *composer.Composer,
	artStore *artifacts.Store,
	logger *slog.Logger,
) *Stage11Handler {
	return &Stage11Handler{
		fragmentStore:   fragStore,
		snapshotCreator: snapCreator,
		composer:        comp,
		artifactStore:   artStore,
		logger:          logger,
	}
}

// HandleSynthesisSubmission processes the synthesized plan from Stage 11.
// Decomposes the submitted plan, creates a canonical artifact, and records
// lineage from all Stage 10 plan artifacts.
func (h *Stage11Handler) HandleSynthesisSubmission(
	ctx context.Context,
	projectID string,
	workflowRunID string,
	submittedContent []byte,
	sourceArtifactIDs []string, // Stage 10 artifact IDs
	rawPayload []byte,
) (*Stage11Result, error) {
	if len(submittedContent) == 0 {
		return nil, fmt.Errorf("submitted plan synthesis content is empty")
	}

	// Persist raw provider response.
	if len(rawPayload) > 0 && h.artifactStore != nil {
		rawPath := fmt.Sprintf("projects/%s/raw/stage11-synthesis-%s.json", projectID, workflowRunID)
		if _, err := h.artifactStore.WriteFile(rawPath, rawPayload); err != nil {
			h.logger.Warn("stage 11: failed to persist raw response", "error", err)
		}
	}

	// Decompose the synthesized plan into fragments.
	dec := decomposer.New(h.fragmentStore)
	decResult, err := dec.Decompose(ctx, projectID, "plan", submittedContent, "stage-11", workflowRunID)
	if err != nil {
		return nil, fmt.Errorf("decomposing plan synthesis: %w", err)
	}

	h.logger.Info("stage 11: plan synthesis decomposed",
		"project_id", projectID,
		"fragments", len(decResult.Fragments),
		"new_versions", decResult.NewVersions,
	)

	// Create canonical plan artifact.
	versionIDs := make([]string, len(decResult.Versions))
	for i, v := range decResult.Versions {
		versionIDs[i] = v.ID
	}

	snapResult, err := h.snapshotCreator.CreateSnapshot(ctx, artifacts.SnapshotInput{
		ProjectID:          projectID,
		ArtifactType:       "plan",
		SourceStage:        "stage-11",
		SourceModel:        string(models.ProviderOpenAI),
		VersionSuffix:      ".synthesized",
		FragmentVersionIDs: versionIDs,
		SourceArtifactIDs:  sourceArtifactIDs,
		RelationType:       string(lineage.SynthesizedFrom),
		IsCanonical:        true, // Stage 11 output IS the canonical plan.
	})
	if err != nil {
		return nil, fmt.Errorf("creating plan synthesis artifact: %w", err)
	}

	h.logger.Info("stage 11: plan synthesis artifact promoted to canonical",
		"artifact_id", snapResult.ArtifactID,
		"version_label", snapResult.VersionLabel,
	)

	return &Stage11Result{
		SynthesisArtifactID: snapResult.ArtifactID,
		VersionLabel:        snapResult.VersionLabel,
		PromotedToCanonical: true,
		FragmentCount:       len(decResult.Fragments),
	}, nil
}
