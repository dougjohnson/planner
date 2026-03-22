package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/dougflynn/flywheel-planner/internal/documents/fragments"
	"github.com/dougflynn/flywheel-planner/internal/models"
)

// Fragment operation errors.
var (
	ErrFragmentNotFound = errors.New("fragment not found in canonical set")
	ErrMissingArgument  = errors.New("required argument missing")
)

// FragmentOperation records a single fragment operation from a review stage.
type FragmentOperation struct {
	Type        string `json:"type"` // "update", "add", "remove"
	FragmentID  string `json:"fragment_id"`
	Heading     string `json:"heading,omitempty"`
	NewContent  string `json:"new_content,omitempty"`
	Rationale   string `json:"rationale"`
	RunID       string `json:"run_id"`
	Stage       string `json:"stage"`
}

// FragmentOpsHandler processes update_fragment, add_fragment, and
// remove_fragment tool calls from review stages (§11.4.2 Review Tools).
type FragmentOpsHandler struct {
	store *fragments.Store
}

// NewFragmentOpsHandler creates a handler backed by the given fragment store.
func NewFragmentOpsHandler(store *fragments.Store) *FragmentOpsHandler {
	return &FragmentOpsHandler{store: store}
}

// HandleUpdateFragment validates and executes an update_fragment tool call.
// Creates a new fragment version with the replacement content.
func (h *FragmentOpsHandler) HandleUpdateFragment(
	ctx context.Context,
	call models.ToolCall,
	stage string,
	runID string,
) (*FragmentOperation, error) {
	fragmentID, err := requireString(call.Arguments, "fragment_id")
	if err != nil {
		return nil, err
	}
	newContent, err := requireString(call.Arguments, "new_content")
	if err != nil {
		return nil, err
	}
	rationale, err := requireString(call.Arguments, "rationale")
	if err != nil {
		return nil, err
	}

	// Validate fragment exists.
	frag, err := h.store.GetFragment(ctx, fragmentID)
	if err != nil {
		if errors.Is(err, fragments.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrFragmentNotFound, fragmentID)
		}
		return nil, fmt.Errorf("looking up fragment: %w", err)
	}
	_ = frag // existence validated

	// Create new version with replacement content.
	_, err = h.store.CreateVersion(ctx, fragmentID, newContent, stage, runID, rationale)
	if err != nil {
		return nil, fmt.Errorf("creating fragment version: %w", err)
	}

	return &FragmentOperation{
		Type:       "update",
		FragmentID: fragmentID,
		NewContent: newContent,
		Rationale:  rationale,
		RunID:      runID,
		Stage:      stage,
	}, nil
}

// HandleAddFragment validates and executes an add_fragment tool call.
// Creates a new fragment and its initial version.
func (h *FragmentOpsHandler) HandleAddFragment(
	ctx context.Context,
	call models.ToolCall,
	projectID string,
	documentType string,
	stage string,
	runID string,
) (*FragmentOperation, error) {
	afterFragmentID, err := requireString(call.Arguments, "after_fragment_id")
	if err != nil {
		return nil, err
	}
	heading, err := requireString(call.Arguments, "heading")
	if err != nil {
		return nil, err
	}
	content, err := requireString(call.Arguments, "content")
	if err != nil {
		return nil, err
	}
	rationale, err := requireString(call.Arguments, "rationale")
	if err != nil {
		return nil, err
	}

	// Validate the "after" fragment exists.
	_, err = h.store.GetFragment(ctx, afterFragmentID)
	if err != nil {
		if errors.Is(err, fragments.ErrNotFound) {
			return nil, fmt.Errorf("%w: after_fragment_id %s", ErrFragmentNotFound, afterFragmentID)
		}
		return nil, fmt.Errorf("looking up after fragment: %w", err)
	}

	// Create new fragment and initial version.
	frag, err := h.store.CreateFragment(ctx, projectID, documentType, heading, 2)
	if err != nil {
		return nil, fmt.Errorf("creating fragment: %w", err)
	}

	_, err = h.store.CreateVersion(ctx, frag.ID, content, stage, runID, rationale)
	if err != nil {
		return nil, fmt.Errorf("creating initial version: %w", err)
	}

	return &FragmentOperation{
		Type:       "add",
		FragmentID: frag.ID,
		Heading:    heading,
		NewContent: content,
		Rationale:  rationale,
		RunID:      runID,
		Stage:      stage,
	}, nil
}

// HandleRemoveFragment validates and executes a remove_fragment tool call.
// Records the removal operation. The actual exclusion from the next artifact
// snapshot is handled by the artifact assembly pipeline.
func (h *FragmentOpsHandler) HandleRemoveFragment(
	ctx context.Context,
	call models.ToolCall,
) (*FragmentOperation, error) {
	fragmentID, err := requireString(call.Arguments, "fragment_id")
	if err != nil {
		return nil, err
	}
	rationale, err := requireString(call.Arguments, "rationale")
	if err != nil {
		return nil, err
	}

	// Validate fragment exists.
	_, err = h.store.GetFragment(ctx, fragmentID)
	if err != nil {
		if errors.Is(err, fragments.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrFragmentNotFound, fragmentID)
		}
		return nil, fmt.Errorf("looking up fragment: %w", err)
	}

	return &FragmentOperation{
		Type:       "remove",
		FragmentID: fragmentID,
		Rationale:  rationale,
	}, nil
}

// requireString extracts a non-empty string argument from the tool call.
func requireString(args map[string]any, key string) (string, error) {
	val, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrMissingArgument, key)
	}
	str, ok := val.(string)
	if !ok || str == "" {
		return "", fmt.Errorf("%w: %q must be a non-empty string", ErrMissingArgument, key)
	}
	return str, nil
}
