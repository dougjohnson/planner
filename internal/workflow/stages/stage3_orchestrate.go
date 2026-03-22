package stages

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/registry"
	"github.com/dougflynn/flywheel-planner/internal/prompts/rendering"
	"github.com/dougflynn/flywheel-planner/internal/workflow/engine"
)

// Stage3OrchestrateResult holds the outcome of a full Stage 3 run.
type Stage3OrchestrateResult struct {
	ArtifactIDs []string `json:"artifact_ids"`
	Failures    []string `json:"failures"`
	QuorumMet   bool     `json:"quorum_met"`
}

// Stage3Orchestrator runs the full Stage 3 pipeline: assemble prompts,
// fan out to providers via ParallelOrchestrator, decompose results.
type Stage3Orchestrator struct {
	handler       *Stage3Handler
	orchestrator  *engine.ParallelOrchestrator
	registry      *registry.Registry
	assembler     *rendering.Assembler
	logger        *slog.Logger
}

// NewStage3Orchestrator creates a new Stage 3 orchestrator.
func NewStage3Orchestrator(
	fragStore *fragments.Store,
	snapCreator *artifacts.SnapshotCreator,
	artStore *artifacts.Store,
	orch *engine.ParallelOrchestrator,
	reg *registry.Registry,
	asm *rendering.Assembler,
	logger *slog.Logger,
) *Stage3Orchestrator {
	return &Stage3Orchestrator{
		handler:      NewStage3Handler(nil, fragStore, snapCreator, artStore, logger),
		orchestrator: orch,
		registry:     reg,
		assembler:    asm,
		logger:       logger,
	}
}

// Execute runs Stage 3: parallel PRD generation.
// 1. Assemble prompts for each provider
// 2. Fan out via ParallelOrchestrator
// 3. Process each successful submission (decompose + snapshot)
func (o *Stage3Orchestrator) Execute(ctx context.Context, projectID, workflowRunID string, seedContent string, foundationContext string) (*Stage3OrchestrateResult, error) {
	// Get all registered providers.
	providerNames := o.registry.RegisteredProviders()
	if len(providerNames) == 0 {
		return nil, fmt.Errorf("no providers registered")
	}

	// Assemble per-provider sessions.
	sessions := make(map[models.ProviderName]models.SessionRequest)
	for _, prov := range providerNames {
		// Assemble prompt for this provider.
		rp := o.assembler.Assemble(ctx, rendering.AssemblyInput{
			Stage:               "stage_3",
			SystemInstructions:  "You are a PRD expansion assistant. Use the submit_document tool to submit your expanded PRD.",
			FoundationalContext: foundationContext,
			PromptText:          "Expand the seed PRD into a comprehensive Product Requirements Document.",
			ArtifactContext:     seedContent,
		})

		sessions[prov] = models.SessionRequest{
			Messages: []models.Message{
				{Role: "user", Content: rp.FullText()},
			},
			Tools: []models.ToolDefinition{{
				Name:        "submit_document",
				Description: "Submit the expanded PRD document",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{"content": map[string]string{"type": "string"}}},
				Required:    true,
			}},
		}
	}

	// Fan out to all providers.
	orchResult, err := o.orchestrator.Execute(ctx, engine.ParallelGenerationRequest{
		ProjectID:          projectID,
		DocumentStream:     "prd",
		Providers:          providerNames,
		SessionsByProvider: sessions,
	})
	if err != nil {
		return nil, fmt.Errorf("parallel generation: %w", err)
	}

	// Process each successful submission.
	var artifactIDs []string
	for _, sub := range orchResult.Submissions {
		// Extract document content from tool calls.
		docContent := extractDocumentContent(sub)
		if docContent == "" {
			o.logger.Warn("no document content in submission", "provider", sub.Provider)
			continue
		}

		result, err := o.handler.HandleSubmitDocument(ctx, projectID, string(sub.Provider), workflowRunID, []byte(docContent))
		if err != nil {
			o.logger.Error("failed to process submission", "provider", sub.Provider, "error", err)
			continue
		}
		artifactIDs = append(artifactIDs, result.ArtifactID)
	}

	// Collect failure messages.
	var failures []string
	for _, f := range orchResult.Failures {
		failures = append(failures, fmt.Sprintf("%s: %s", f.Provider, f.Error))
	}

	return &Stage3OrchestrateResult{
		ArtifactIDs: artifactIDs,
		Failures:    failures,
		QuorumMet:   orchResult.QuorumMet,
	}, nil
}

// extractDocumentContent gets the submitted document from tool calls.
func extractDocumentContent(sub engine.ProviderSubmission) string {
	for _, tc := range sub.ToolCalls {
		if tc.Name == "submit_document" {
			if content, ok := tc.Arguments["content"].(string); ok {
				return content
			}
		}
	}
	// Fall back to text response if no tool call.
	if sub.Response != nil {
		return sub.Response.Text
	}
	return ""
}
