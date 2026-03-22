package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

// Agreement categories (§11.4.2).
var validAgreementCategories = map[string]bool{
	"wholeheartedly_agrees": true,
	"somewhat_agrees":       true,
}

// Disagreement severities (§11.4.2).
var validDisagreementSeverities = map[string]bool{
	"minor":    true,
	"moderate": true,
	"major":    true,
}

// AgreementReport is the structured result of a report_agreement tool call.
type AgreementReport struct {
	FragmentID string `json:"fragment_id"`
	Category   string `json:"category"`
	Rationale  string `json:"rationale"`
	RunID      string `json:"run_id"`
	Stage      string `json:"stage"`
}

// DisagreementReport is the structured result of a report_disagreement tool call.
// Each disagreement directly creates a review_item record (§11.4.2).
type DisagreementReport struct {
	FragmentID      string `json:"fragment_id"`
	Severity        string `json:"severity"`
	Summary         string `json:"summary"`
	Rationale       string `json:"rationale"`
	SuggestedChange string `json:"suggested_change"`
	RunID           string `json:"run_id"`
	Stage           string `json:"stage"`
}

// DispositionHandler processes report_agreement and report_disagreement
// tool calls from integration stages (§11.4.2 Integration Tools).
type DispositionHandler struct {
	store *fragments.Store
}

// NewDispositionHandler creates a handler backed by the given fragment store.
func NewDispositionHandler(store *fragments.Store) *DispositionHandler {
	return &DispositionHandler{store: store}
}

// HandleReportAgreement validates and processes a report_agreement tool call.
func (h *DispositionHandler) HandleReportAgreement(
	ctx context.Context,
	call models.ToolCall,
	stage string,
	runID string,
) (*AgreementReport, error) {
	fragmentID, err := requireString(call.Arguments, "fragment_id")
	if err != nil {
		return nil, err
	}
	category, err := requireString(call.Arguments, "category")
	if err != nil {
		return nil, err
	}
	rationale, err := requireString(call.Arguments, "rationale")
	if err != nil {
		return nil, err
	}

	// Validate category enum.
	if !validAgreementCategories[category] {
		return nil, fmt.Errorf("report_agreement: invalid category %q; must be one of: wholeheartedly_agrees, somewhat_agrees", category)
	}

	// Validate fragment exists.
	_, err = h.store.GetFragment(ctx, fragmentID)
	if err != nil {
		if errors.Is(err, fragments.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrFragmentNotFound, fragmentID)
		}
		return nil, fmt.Errorf("looking up fragment: %w", err)
	}

	return &AgreementReport{
		FragmentID: fragmentID,
		Category:   category,
		Rationale:  rationale,
		RunID:      runID,
		Stage:      stage,
	}, nil
}

// HandleReportDisagreement validates and processes a report_disagreement tool call.
// The returned DisagreementReport contains all fields needed to create a
// review_item record directly — no parsing or extraction step is needed.
func (h *DispositionHandler) HandleReportDisagreement(
	ctx context.Context,
	call models.ToolCall,
	stage string,
	runID string,
) (*DisagreementReport, error) {
	fragmentID, err := requireString(call.Arguments, "fragment_id")
	if err != nil {
		return nil, err
	}
	severity, err := requireString(call.Arguments, "severity")
	if err != nil {
		return nil, err
	}
	summary, err := requireString(call.Arguments, "summary")
	if err != nil {
		return nil, err
	}
	rationale, err := requireString(call.Arguments, "rationale")
	if err != nil {
		return nil, err
	}
	suggestedChange, err := requireString(call.Arguments, "suggested_change")
	if err != nil {
		return nil, err
	}

	// Validate severity enum.
	if !validDisagreementSeverities[severity] {
		return nil, fmt.Errorf("report_disagreement: invalid severity %q; must be one of: minor, moderate, major", severity)
	}

	// Validate fragment exists.
	_, err = h.store.GetFragment(ctx, fragmentID)
	if err != nil {
		if errors.Is(err, fragments.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrFragmentNotFound, fragmentID)
		}
		return nil, fmt.Errorf("looking up fragment: %w", err)
	}

	return &DisagreementReport{
		FragmentID:      fragmentID,
		Severity:        severity,
		Summary:         summary,
		Rationale:       rationale,
		SuggestedChange: suggestedChange,
		RunID:           runID,
		Stage:           stage,
	}, nil
}
