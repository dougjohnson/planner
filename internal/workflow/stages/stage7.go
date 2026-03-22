package stages

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/documents/composer"
	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

// FragmentOperation represents a proposed fragment change from the review model.
type FragmentOperation struct {
	Type       string `json:"type"` // "update", "add", "remove"
	FragmentID string `json:"fragment_id,omitempty"`
	Heading    string `json:"heading,omitempty"`     // for "add"
	NewContent string `json:"new_content,omitempty"` // for "update" and "add"
	Rationale  string `json:"rationale"`
	AfterID    string `json:"after_fragment_id,omitempty"` // for "add"
}

// ReviewSummary is the model's overall review summary.
type ReviewSummary struct {
	Summary     string   `json:"summary"`
	KeyFindings []string `json:"key_findings"`
}

// Stage7Result holds the outcome of a Stage 7 review pass.
type Stage7Result struct {
	Operations    []FragmentOperation `json:"operations"`
	Summary       *ReviewSummary      `json:"summary,omitempty"`
	OperationCount int               `json:"operation_count"`
}

// Stage7Handler implements Stage 7: Review pass with fragment operations.
type Stage7Handler struct {
	fragmentStore *fragments.Store
	composer      *composer.Composer
	logger        *slog.Logger
}

// NewStage7Handler creates a new Stage 7 handler.
func NewStage7Handler(
	fragStore *fragments.Store,
	comp *composer.Composer,
	logger *slog.Logger,
) *Stage7Handler {
	return &Stage7Handler{
		fragmentStore: fragStore,
		composer:      comp,
		logger:        logger,
	}
}

// ProcessToolCalls extracts fragment operations and review summary from model tool calls.
func (h *Stage7Handler) ProcessToolCalls(toolCalls []models.ToolCall) *Stage7Result {
	result := &Stage7Result{}

	for _, tc := range toolCalls {
		switch tc.Name {
		case "update_fragment":
			result.Operations = append(result.Operations, FragmentOperation{
				Type:       "update",
				FragmentID: strArg(tc.Arguments, "fragment_id"),
				NewContent: strArg(tc.Arguments, "new_content"),
				Rationale:  strArg(tc.Arguments, "rationale"),
			})
		case "add_fragment":
			result.Operations = append(result.Operations, FragmentOperation{
				Type:       "add",
				AfterID:    strArg(tc.Arguments, "after_fragment_id"),
				Heading:    strArg(tc.Arguments, "heading"),
				NewContent: strArg(tc.Arguments, "content"),
				Rationale:  strArg(tc.Arguments, "rationale"),
			})
		case "remove_fragment":
			result.Operations = append(result.Operations, FragmentOperation{
				Type:       "remove",
				FragmentID: strArg(tc.Arguments, "fragment_id"),
				Rationale:  strArg(tc.Arguments, "rationale"),
			})
		case "submit_review_summary":
			result.Summary = &ReviewSummary{
				Summary: strArg(tc.Arguments, "summary"),
			}
			if findings, ok := tc.Arguments["key_findings"].([]any); ok {
				for _, f := range findings {
					if s, ok := f.(string); ok {
						result.Summary.KeyFindings = append(result.Summary.KeyFindings, s)
					}
				}
			}
		}
	}

	result.OperationCount = len(result.Operations)
	return result
}

// PrepareContext assembles the review context: annotated canonical PRD.
func (h *Stage7Handler) PrepareContext(ctx context.Context, canonicalArtifactID string) (string, error) {
	annotated, err := h.composer.ComposeWithAnnotations(ctx, canonicalArtifactID)
	if err != nil {
		return "", fmt.Errorf("composing annotated PRD: %w", err)
	}
	return annotated, nil
}

// strArg extracts a string argument from tool call args.
func strArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}
