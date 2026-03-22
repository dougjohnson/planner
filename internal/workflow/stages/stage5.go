package stages

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/artifacts/lineage"
	"github.com/dougflynn/flywheel-planner/internal/documents/decomposer"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/google/uuid"
)

// Stage5Result holds the outcome of Stage 5 Opus integration.
type Stage5Result struct {
	IntegrationArtifactID string `json:"integration_artifact_id"`
	VersionLabel          string `json:"version_label"`
	PromotedToCanonical   bool   `json:"promoted_to_canonical"`
	DisagreementCount     int    `json:"disagreement_count"`
	AgreementCount        int    `json:"agreement_count"`
}

// DisagreementReport represents a report_disagreement tool call result.
type DisagreementReport struct {
	FragmentID      string `json:"fragment_id"`
	Severity        string `json:"severity"`
	Summary         string `json:"summary"`
	Rationale       string `json:"rationale"`
	SuggestedChange string `json:"suggested_change"`
}

// AgreementReport represents a report_agreement tool call result.
type AgreementReport struct {
	FragmentID string `json:"fragment_id"`
	Category   string `json:"category"`
	Rationale  string `json:"rationale"`
}

// Stage5Handler implements Stage 5: Opus PRD Integration Pass.
type Stage5Handler struct {
	db              *sql.DB
	fragmentStore   *fragments.Store
	snapshotCreator *artifacts.SnapshotCreator
	artifactStore   *artifacts.Store
	logger          *slog.Logger
}

// NewStage5Handler creates a new Stage 5 handler.
func NewStage5Handler(
	db *sql.DB,
	fragStore *fragments.Store,
	snapCreator *artifacts.SnapshotCreator,
	artStore *artifacts.Store,
	logger *slog.Logger,
) *Stage5Handler {
	return &Stage5Handler{
		db:              db,
		fragmentStore:   fragStore,
		snapshotCreator: snapCreator,
		artifactStore:   artStore,
		logger:          logger,
	}
}

// HandleIntegrationSubmission processes Opus's integrated PRD.
func (h *Stage5Handler) HandleIntegrationSubmission(
	ctx context.Context,
	projectID string,
	workflowRunID string,
	submittedContent []byte,
	synthesisArtifactID string,
	disagreements []DisagreementReport,
	agreements []AgreementReport,
	rawPayload []byte,
) (*Stage5Result, error) {
	if len(submittedContent) == 0 {
		return nil, fmt.Errorf("submitted integration content is empty")
	}

	// Persist raw response.
	if len(rawPayload) > 0 && h.artifactStore != nil {
		rawPath := fmt.Sprintf("projects/%s/raw/stage5-integration-%s.json", projectID, workflowRunID)
		h.artifactStore.WriteFile(rawPath, rawPayload)
	}

	// Decompose integrated PRD.
	dec := decomposer.New(h.fragmentStore)
	decResult, err := dec.Decompose(ctx, projectID, "prd", submittedContent, "stage-5", workflowRunID)
	if err != nil {
		return nil, fmt.Errorf("decomposing integration: %w", err)
	}

	// Create canonical artifact with lineage.
	versionIDs := make([]string, len(decResult.Versions))
	for i, v := range decResult.Versions {
		versionIDs[i] = v.ID
	}

	snapResult, err := h.snapshotCreator.CreateSnapshot(ctx, artifacts.SnapshotInput{
		ProjectID:          projectID,
		ArtifactType:       "prd",
		SourceStage:        "stage-5",
		SourceModel:        string(models.ProviderAnthropic),
		VersionSuffix:      ".integrated",
		FragmentVersionIDs: versionIDs,
		SourceArtifactIDs:  []string{synthesisArtifactID},
		RelationType:       string(lineage.IntegratedFrom),
		IsCanonical:        true,
	})
	if err != nil {
		return nil, fmt.Errorf("creating integration artifact: %w", err)
	}

	// Record disagreements as review_items.
	ts := time.Now().UTC().Format(time.RFC3339)
	for _, d := range disagreements {
		_, err := h.db.ExecContext(ctx, `
			INSERT INTO review_items (id, project_id, stage, fragment_id, classification, summary, status, created_at)
			VALUES (?, ?, 'stage-5', ?, ?, ?, 'pending', ?)
		`, uuid.NewString(), projectID, d.FragmentID, d.Severity, d.Summary, ts)
		if err != nil {
			h.logger.Warn("failed to create review item", "fragment_id", d.FragmentID, "error", err)
		}
	}

	h.logger.Info("stage 5: integration complete",
		"artifact_id", snapResult.ArtifactID,
		"agreements", len(agreements),
		"disagreements", len(disagreements),
	)

	return &Stage5Result{
		IntegrationArtifactID: snapResult.ArtifactID,
		VersionLabel:          snapResult.VersionLabel,
		PromotedToCanonical:   true,
		DisagreementCount:     len(disagreements),
		AgreementCount:        len(agreements),
	}, nil
}
