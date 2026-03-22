package stages

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/documents/composer"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

// Stage14Result holds the outcome of a Stage 14 plan review pass.
type Stage14Result struct {
	Operations     []FragmentOperation `json:"operations"`
	Summary        *ReviewSummary      `json:"summary,omitempty"`
	OperationCount int                 `json:"operation_count"`
}

// Stage14Handler implements Stage 14: Plan review pass with fragment operations.
// Mirrors Stage 7 but operates on the plan document stream. Uses the same
// tool call processing (update_fragment, add_fragment, remove_fragment,
// submit_review_summary) with plan-specific prompts and context.
type Stage14Handler struct {
	inner *Stage7Handler // reuse Stage 7 logic
}

// NewStage14Handler creates a new Stage 14 handler.
func NewStage14Handler(
	fragStore *fragments.Store,
	comp *composer.Composer,
	logger *slog.Logger,
) *Stage14Handler {
	return &Stage14Handler{
		inner: NewStage7Handler(fragStore, comp, logger),
	}
}

// ProcessToolCalls extracts fragment operations from model tool calls.
// Same as Stage 7 — the tool interface is identical for PRD and plan reviews.
func (h *Stage14Handler) ProcessToolCalls(toolCalls []models.ToolCall) *Stage14Result {
	s7Result := h.inner.ProcessToolCalls(toolCalls)
	return &Stage14Result{
		Operations:     s7Result.Operations,
		Summary:        s7Result.Summary,
		OperationCount: s7Result.OperationCount,
	}
}

// PrepareContext assembles the plan review context: annotated canonical plan.
func (h *Stage14Handler) PrepareContext(ctx context.Context, canonicalPlanArtifactID string) (string, error) {
	annotated, err := h.inner.PrepareContext(ctx, canonicalPlanArtifactID)
	if err != nil {
		return "", fmt.Errorf("composing annotated plan: %w", err)
	}
	return annotated, nil
}
