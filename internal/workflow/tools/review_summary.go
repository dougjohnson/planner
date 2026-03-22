package tools

import (
	"fmt"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

// ReviewSummary is the structured result of a submit_review_summary tool call.
type ReviewSummary struct {
	Summary     string   `json:"summary"`
	KeyFindings []string `json:"key_findings"`
	RunID       string   `json:"run_id"`
	Stage       string   `json:"stage"`
}

// HandleSubmitReviewSummary validates and processes a submit_review_summary
// tool call. The summary and findings are stored as metadata on the workflow run.
func HandleSubmitReviewSummary(
	call models.ToolCall,
	stage string,
	runID string,
) (*ReviewSummary, error) {
	summary, err := requireString(call.Arguments, "summary")
	if err != nil {
		return nil, err
	}

	var findings []string
	if raw, ok := call.Arguments["key_findings"]; ok {
		arr, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("submit_review_summary: 'key_findings' must be an array of strings")
		}
		for i, item := range arr {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("submit_review_summary: key_findings[%d] must be a string", i)
			}
			findings = append(findings, s)
		}
	}

	return &ReviewSummary{
		Summary:     summary,
		KeyFindings: findings,
		RunID:       runID,
		Stage:       stage,
	}, nil
}
