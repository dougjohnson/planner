package tools

import (
	"fmt"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

var validChangeTypes = map[string]bool{
	"added":        true,
	"modified":     true,
	"removed":      true,
	"reorganized":  true,
}

// ChangeRationale is the structured result of a submit_change_rationale tool call.
type ChangeRationale struct {
	SectionID   string `json:"section_id"`
	ChangeType  string `json:"change_type"`
	Rationale   string `json:"rationale"`
	SourceModel string `json:"source_model,omitempty"`
	RunID       string `json:"run_id"`
	Stage       string `json:"stage"`
}

// HandleSubmitChangeRationale validates and processes a submit_change_rationale
// tool call from synthesis stages (4, 11). Records why a change was made and
// which competing model influenced it.
func HandleSubmitChangeRationale(
	call models.ToolCall,
	stage string,
	runID string,
) (*ChangeRationale, error) {
	sectionID, err := requireString(call.Arguments, "section_id")
	if err != nil {
		return nil, err
	}
	changeType, err := requireString(call.Arguments, "change_type")
	if err != nil {
		return nil, err
	}
	rationale, err := requireString(call.Arguments, "rationale")
	if err != nil {
		return nil, err
	}

	if !validChangeTypes[changeType] {
		return nil, fmt.Errorf("submit_change_rationale: invalid change_type %q; must be one of: added, modified, removed, reorganized", changeType)
	}

	sourceModel, _ := call.Arguments["source_model"].(string)

	return &ChangeRationale{
		SectionID:   sectionID,
		ChangeType:  changeType,
		Rationale:   rationale,
		SourceModel: sourceModel,
		RunID:       runID,
		Stage:       stage,
	}, nil
}
