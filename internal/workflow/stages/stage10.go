package stages

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/documents/composer"
	"github.com/dougflynn/flywheel-planner/internal/documents/decomposer"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/registry"
	"github.com/dougflynn/flywheel-planner/internal/prompts/rendering"
	"github.com/dougflynn/flywheel-planner/internal/workflow/engine"
)

// Stage10Result holds the outcome of Stage 10 parallel plan generation.
type Stage10Result struct {
	ArtifactIDs []string `json:"artifact_ids"`
	Failures    []string `json:"failures"`
	QuorumMet   bool     `json:"quorum_met"`
}

// Stage10Orchestrator runs Stage 10: parallel plan generation.
// Mirrors Stage 3 but operates on the Plan document stream using the
// canonical PRD as input context.
type Stage10Orchestrator struct {
	fragmentStore   *fragments.Store
	snapshotCreator *artifacts.SnapshotCreator
	artifactStore   *artifacts.Store
	composer        *composer.Composer
	orchestrator    *engine.ParallelOrchestrator
	reg             *registry.Registry
	assembler       *rendering.Assembler
	logger          *slog.Logger
}

// NewStage10Orchestrator creates a new Stage 10 orchestrator.
func NewStage10Orchestrator(
	fragStore *fragments.Store,
	snapCreator *artifacts.SnapshotCreator,
	artStore *artifacts.Store,
	comp *composer.Composer,
	orch *engine.ParallelOrchestrator,
	reg *registry.Registry,
	asm *rendering.Assembler,
	logger *slog.Logger,
) *Stage10Orchestrator {
	return &Stage10Orchestrator{
		fragmentStore:   fragStore,
		snapshotCreator: snapCreator,
		artifactStore:   artStore,
		composer:        comp,
		orchestrator:    orch,
		reg:             reg,
		assembler:       asm,
		logger:          logger,
	}
}

// Execute runs Stage 10: parallel plan generation.
// 1. Compose the canonical PRD from fragments
// 2. Assemble prompts with PRD + foundations context
// 3. Fan out to all providers
// 4. Decompose each plan submission into fragments
func (o *Stage10Orchestrator) Execute(
	ctx context.Context,
	projectID string,
	workflowRunID string,
	canonicalPRDArtifactID string,
	foundationContext string,
) (*Stage10Result, error) {
	// Step 1: Compose the canonical PRD for inclusion in prompts.
	prdContent, err := o.composer.Compose(ctx, canonicalPRDArtifactID)
	if err != nil {
		return nil, fmt.Errorf("composing canonical PRD: %w", err)
	}

	// Get all registered providers.
	providerNames := o.reg.RegisteredProviders()
	if len(providerNames) == 0 {
		return nil, fmt.Errorf("no providers registered")
	}

	// Step 2: Assemble per-provider sessions.
	sessions := make(map[models.ProviderName]models.SessionRequest)
	for _, prov := range providerNames {
		rp := o.assembler.Assemble(ctx, rendering.AssemblyInput{
			Stage:               "stage_10",
			SystemInstructions:  "You are an implementation planning assistant. Use the submit_document tool to submit your implementation plan.",
			FoundationalContext: foundationContext,
			PromptText:          "Create a comprehensive implementation plan for the following PRD.",
			ArtifactContext:     prdContent,
			ArtifactIDs:        []string{canonicalPRDArtifactID},
		})

		sessions[prov] = models.SessionRequest{
			Messages: []models.Message{
				{Role: "user", Content: rp.FullText()},
			},
			Tools: []models.ToolDefinition{{
				Name:        "submit_document",
				Description: "Submit the implementation plan document",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{"content": map[string]string{"type": "string"}}},
				Required:    true,
			}},
		}
	}

	// Step 3: Fan out to all providers.
	orchResult, err := o.orchestrator.Execute(ctx, engine.ParallelGenerationRequest{
		ProjectID:          projectID,
		DocumentStream:     "plan",
		Providers:          providerNames,
		SessionsByProvider: sessions,
	})
	if err != nil {
		return nil, fmt.Errorf("parallel plan generation: %w", err)
	}

	// Step 4: Process each successful submission.
	var artifactIDs []string
	dec := decomposer.New(o.fragmentStore)

	for _, sub := range orchResult.Submissions {
		docContent := extractDocumentContent(sub)
		if docContent == "" {
			o.logger.Warn("stage 10: no document content", "provider", sub.Provider)
			continue
		}

		// Decompose into plan fragments.
		decResult, err := dec.Decompose(ctx, projectID, "plan", []byte(docContent), "stage-10", workflowRunID)
		if err != nil {
			o.logger.Error("stage 10: decomposition failed", "provider", sub.Provider, "error", err)
			continue
		}

		// Create non-canonical plan artifact.
		versionIDs := make([]string, len(decResult.Versions))
		for i, v := range decResult.Versions {
			versionIDs[i] = v.ID
		}

		snapResult, err := o.snapshotCreator.CreateSnapshot(ctx, artifacts.SnapshotInput{
			ProjectID:          projectID,
			ArtifactType:       "plan",
			SourceStage:        "stage-10",
			SourceModel:        string(sub.Provider),
			VersionSuffix:      ".generated",
			FragmentVersionIDs: versionIDs,
			SourceArtifactIDs:  []string{canonicalPRDArtifactID},
			RelationType:       "generated_from",
			IsCanonical:        false,
		})
		if err != nil {
			o.logger.Error("stage 10: snapshot failed", "provider", sub.Provider, "error", err)
			continue
		}

		artifactIDs = append(artifactIDs, snapResult.ArtifactID)
		o.logger.Info("stage 10: plan artifact created",
			"provider", sub.Provider,
			"artifact_id", snapResult.ArtifactID,
			"version_label", snapResult.VersionLabel,
		)
	}

	var failures []string
	for _, f := range orchResult.Failures {
		failures = append(failures, fmt.Sprintf("%s: %s", f.Provider, f.Error))
	}

	return &Stage10Result{
		ArtifactIDs: artifactIDs,
		Failures:    failures,
		QuorumMet:   orchResult.QuorumMet,
	}, nil
}
