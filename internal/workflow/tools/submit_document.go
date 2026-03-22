// Package tools implements the tool-call handlers for the flywheel-planner
// workflow engine. Each tool validates its arguments, performs the operation,
// and returns a structured result.
package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/markdown"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

// Errors for tool validation.
var (
	ErrMissingContent   = errors.New("submit_document: 'content' argument is required")
	ErrEmptyContent     = errors.New("submit_document: 'content' must not be empty")
	ErrToolNotRecognized = errors.New("tool call not recognized")
)

// SubmitDocumentResult holds the outcome of processing a submit_document tool call.
type SubmitDocumentResult struct {
	// FragmentCount is the number of fragments created or matched.
	FragmentCount int `json:"fragment_count"`
	// NewFragments is the count of newly created fragments.
	NewFragments int `json:"new_fragments"`
	// UpdatedFragments is the count of fragments with new versions.
	UpdatedFragments int `json:"updated_fragments"`
	// UnchangedFragments is the count of fragments whose content was identical.
	UnchangedFragments int `json:"unchanged_fragments"`
	// ChangeSummary is the summary provided by the model.
	ChangeSummary string `json:"change_summary"`
}

// SubmitDocumentHandler validates and processes submit_document tool calls.
// It integrates the markdown decomposition pipeline with the fragment store.
type SubmitDocumentHandler struct {
	store *fragments.Store
}

// NewSubmitDocumentHandler creates a new handler backed by the given fragment store.
func NewSubmitDocumentHandler(store *fragments.Store) *SubmitDocumentHandler {
	return &SubmitDocumentHandler{store: store}
}

// Validate checks that the tool call has valid arguments for submit_document.
// Returns a descriptive error suitable for sending back to the model.
func (h *SubmitDocumentHandler) Validate(call models.ToolCall) error {
	if call.Name != "submit_document" {
		return fmt.Errorf("%w: %s", ErrToolNotRecognized, call.Name)
	}

	content, ok := call.Arguments["content"]
	if !ok {
		return ErrMissingContent
	}

	contentStr, ok := content.(string)
	if !ok || contentStr == "" {
		return ErrEmptyContent
	}

	return nil
}

// Execute processes a validated submit_document tool call. It decomposes the
// submitted markdown into sections, matches them to existing fragments by
// heading, creates new fragments as needed, and creates versions for any
// changed content.
//
// Parameters:
//   - ctx: context for cancellation
//   - call: the validated tool call
//   - projectID: the project this submission belongs to
//   - documentType: "prd" or "plan"
//   - stage: the stage that produced this document
//   - runID: the workflow run ID for provenance
func (h *SubmitDocumentHandler) Execute(
	ctx context.Context,
	call models.ToolCall,
	projectID string,
	documentType string,
	stage string,
	runID string,
) (*SubmitDocumentResult, error) {
	content := call.Arguments["content"].(string)
	changeSummary, _ := call.Arguments["change_summary"].(string)

	// Decompose the markdown into sections at ## heading boundaries.
	sections := markdown.Decompose([]byte(content))

	result := &SubmitDocumentResult{
		ChangeSummary: changeSummary,
	}

	for _, section := range sections {
		heading := section.Heading
		if heading == "" {
			heading = "_preamble"
		}

		// Try to match to an existing fragment by heading.
		existing, err := h.store.FindByHeading(ctx, projectID, documentType, heading)
		if err != nil && !errors.Is(err, fragments.ErrNotFound) {
			return nil, fmt.Errorf("looking up fragment %q: %w", heading, err)
		}

		if existing != nil {
			// Fragment exists — check if content changed.
			latestVersion, verr := h.store.LatestVersion(ctx, existing.ID)
			if verr != nil && !errors.Is(verr, fragments.ErrNotFound) {
				return nil, fmt.Errorf("getting latest version for %s: %w", existing.ID, verr)
			}

			if latestVersion != nil && latestVersion.Content == section.Content {
				// Content unchanged — reuse existing version.
				result.UnchangedFragments++
			} else {
				// Content changed — create new version.
				_, err := h.store.CreateVersion(ctx, existing.ID, section.Content, stage, runID, changeSummary)
				if err != nil {
					return nil, fmt.Errorf("creating version for fragment %s: %w", existing.ID, err)
				}
				result.UpdatedFragments++
			}
		} else {
			// New fragment — create fragment and initial version.
			frag, err := h.store.CreateFragment(ctx, projectID, documentType, heading, section.Depth)
			if err != nil {
				return nil, fmt.Errorf("creating fragment %q: %w", heading, err)
			}
			_, err = h.store.CreateVersion(ctx, frag.ID, section.Content, stage, runID, changeSummary)
			if err != nil {
				return nil, fmt.Errorf("creating initial version for %s: %w", frag.ID, err)
			}
			result.NewFragments++
		}

		result.FragmentCount++
	}

	return result, nil
}

// ValidateToolCall validates any tool call against the canonical tool schemas.
// Returns nil if the call is valid, or a descriptive error for the model.
func ValidateToolCall(call models.ToolCall, stageTools []models.ToolSchema) error {
	// Find the matching schema.
	var schema *models.ToolSchema
	for i, t := range stageTools {
		if t.Name == call.Name {
			schema = &stageTools[i]
			break
		}
	}
	if schema == nil {
		return fmt.Errorf("%w: %q is not available for this stage", ErrToolNotRecognized, call.Name)
	}

	// Validate required properties.
	for _, req := range schema.Parameters.Required {
		val, ok := call.Arguments[req]
		if !ok {
			return fmt.Errorf("tool %q: required argument %q is missing", call.Name, req)
		}
		// Check for empty strings on required fields.
		if strVal, isStr := val.(string); isStr && strVal == "" {
			return fmt.Errorf("tool %q: required argument %q must not be empty", call.Name, req)
		}
	}

	// Validate enum constraints.
	for propName, prop := range schema.Parameters.Properties {
		val, ok := call.Arguments[propName]
		if !ok || len(prop.Enum) == 0 {
			continue
		}
		strVal, isStr := val.(string)
		if !isStr {
			continue
		}
		valid := false
		for _, allowed := range prop.Enum {
			if strVal == allowed {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("tool %q: argument %q value %q is not one of %v", call.Name, propName, strVal, prop.Enum)
		}
	}

	return nil
}
