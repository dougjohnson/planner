package models

// ToolSchema defines the system's canonical tool schema that providers translate
// into their native format. This is the single source of truth for tool definitions
// used across all stages.
type ToolSchema struct {
	// Name is the canonical tool name (e.g., "submit_document").
	Name string `json:"name"`
	// Description explains the tool's purpose to the model.
	Description string `json:"description"`
	// Parameters defines the JSON Schema for tool arguments.
	Parameters JSONSchema `json:"parameters"`
	// Required indicates this tool must be called during the stage.
	Required bool `json:"required"`
}

// JSONSchema is a simplified JSON Schema representation for tool parameters.
type JSONSchema struct {
	Type        string                `json:"type"`
	Properties  map[string]SchemaProperty `json:"properties,omitempty"`
	Required    []string              `json:"required,omitempty"`
	Description string                `json:"description,omitempty"`
}

// SchemaProperty describes a single property in a JSON Schema.
type SchemaProperty struct {
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Enum        []string          `json:"enum,omitempty"`
	Items       *SchemaProperty   `json:"items,omitempty"`
	Properties  map[string]SchemaProperty `json:"properties,omitempty"`
	Required    []string          `json:"required,omitempty"`
}

// ToolCallResult is a normalized record of a single tool invocation and its
// validation status, returned from the translation layer.
type ToolCallResult struct {
	ToolCall  ToolCall `json:"tool_call"`
	Valid     bool     `json:"valid"`
	Error     string   `json:"error,omitempty"`
}

// --- Stage Tool Catalog (§11.4.2) ---

// GenerationTools returns tool schemas for generation stages (3, 10).
func GenerationTools() []ToolSchema {
	return []ToolSchema{
		submitDocumentTool(true),
	}
}

// SynthesisTools returns tool schemas for synthesis stages (4, 11).
func SynthesisTools() []ToolSchema {
	return []ToolSchema{
		submitDocumentTool(true),
		submitChangeRationaleTool(),
	}
}

// IntegrationTools returns tool schemas for integration stages (5, 12).
func IntegrationTools() []ToolSchema {
	return []ToolSchema{
		submitDocumentTool(true),
		reportAgreementTool(),
		reportDisagreementTool(),
	}
}

// ReviewTools returns tool schemas for review stages (7, 14).
func ReviewTools() []ToolSchema {
	return []ToolSchema{
		updateFragmentTool(),
		addFragmentTool(),
		removeFragmentTool(),
		submitReviewSummaryTool(),
	}
}

// ToolsForStage returns the canonical tool schemas for a given stage ID.
func ToolsForStage(stageID string) []ToolSchema {
	switch stageID {
	case "parallel_prd_generation", "parallel_plan_generation":
		return GenerationTools()
	case "prd_synthesis", "plan_synthesis":
		return SynthesisTools()
	case "prd_integration", "plan_integration":
		return IntegrationTools()
	case "prd_review", "plan_review":
		return ReviewTools()
	default:
		return nil
	}
}

// --- Individual Tool Definitions ---

func submitDocumentTool(required bool) ToolSchema {
	return ToolSchema{
		Name:        "submit_document",
		Description: "Submit a complete markdown document. The system will decompose it into fragments automatically by parsing ## headings.",
		Required:    required,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"content": {
					Type:        "string",
					Description: "The full markdown content of the document.",
				},
				"change_summary": {
					Type:        "string",
					Description: "A brief description of the changes made or the document's purpose.",
				},
			},
			Required: []string{"content", "change_summary"},
		},
	}
}

func submitChangeRationaleTool() ToolSchema {
	return ToolSchema{
		Name:        "submit_change_rationale",
		Description: "Record the rationale for a significant change made during synthesis. Call once per major change.",
		Required:    false,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"section_id": {
					Type:        "string",
					Description: "Identifier of the section that was changed.",
				},
				"change_type": {
					Type:        "string",
					Description: "The type of change made.",
					Enum:        []string{"added", "modified", "removed", "reorganized"},
				},
				"rationale": {
					Type:        "string",
					Description: "Explanation of why this change was made.",
				},
				"source_model": {
					Type:        "string",
					Description: "Which model's output influenced this change.",
				},
			},
			Required: []string{"section_id", "change_type", "rationale"},
		},
	}
}

func reportAgreementTool() ToolSchema {
	return ToolSchema{
		Name:        "report_agreement",
		Description: "Report agreement with a change to a specific fragment. Each integration run must emit at least one disposition report.",
		Required:    false,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"fragment_id": {
					Type:        "string",
					Description: "The fragment ID (from <!-- fragment:ID --> annotations) being assessed.",
				},
				"category": {
					Type:        "string",
					Description: "Degree of agreement with the change.",
					Enum:        []string{"wholeheartedly_agrees", "somewhat_agrees"},
				},
				"rationale": {
					Type:        "string",
					Description: "Brief explanation of why you agree with this change.",
				},
			},
			Required: []string{"fragment_id", "category", "rationale"},
		},
	}
}

func reportDisagreementTool() ToolSchema {
	return ToolSchema{
		Name:        "report_disagreement",
		Description: "Report a substantive disagreement with a change to a specific fragment. Creates a review item for user resolution.",
		Required:    false,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"fragment_id": {
					Type:        "string",
					Description: "The fragment ID (from <!-- fragment:ID --> annotations) being disputed.",
				},
				"severity": {
					Type:        "string",
					Description: "How significant the disagreement is.",
					Enum:        []string{"minor", "moderate", "major"},
				},
				"summary": {
					Type:        "string",
					Description: "One-line summary of the disagreement.",
				},
				"rationale": {
					Type:        "string",
					Description: "Detailed explanation of why you disagree.",
				},
				"suggested_change": {
					Type:        "string",
					Description: "The specific change you would make instead.",
				},
			},
			Required: []string{"fragment_id", "severity", "summary", "rationale", "suggested_change"},
		},
	}
}

func updateFragmentTool() ToolSchema {
	return ToolSchema{
		Name:        "update_fragment",
		Description: "Replace the content of an existing fragment. Reference the fragment ID from <!-- fragment:ID --> annotations in the document.",
		Required:    false,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"fragment_id": {
					Type:        "string",
					Description: "The ID of the fragment to update.",
				},
				"new_content": {
					Type:        "string",
					Description: "The complete replacement content for this section.",
				},
				"rationale": {
					Type:        "string",
					Description: "Explanation of why this change improves the document.",
				},
			},
			Required: []string{"fragment_id", "new_content", "rationale"},
		},
	}
}

func addFragmentTool() ToolSchema {
	return ToolSchema{
		Name:        "add_fragment",
		Description: "Insert a new section after an existing fragment.",
		Required:    false,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"after_fragment_id": {
					Type:        "string",
					Description: "The fragment ID after which to insert the new section.",
				},
				"heading": {
					Type:        "string",
					Description: "The ## heading text for the new section.",
				},
				"content": {
					Type:        "string",
					Description: "The markdown content of the new section (below the heading).",
				},
				"rationale": {
					Type:        "string",
					Description: "Explanation of why this section should be added.",
				},
			},
			Required: []string{"after_fragment_id", "heading", "content", "rationale"},
		},
	}
}

func removeFragmentTool() ToolSchema {
	return ToolSchema{
		Name:        "remove_fragment",
		Description: "Remove a section from the document.",
		Required:    false,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"fragment_id": {
					Type:        "string",
					Description: "The ID of the fragment to remove.",
				},
				"rationale": {
					Type:        "string",
					Description: "Explanation of why this section should be removed.",
				},
			},
			Required: []string{"fragment_id", "rationale"},
		},
	}
}

func submitReviewSummaryTool() ToolSchema {
	return ToolSchema{
		Name:        "submit_review_summary",
		Description: "Submit an overall review summary with key findings. Recommended after completing fragment operations.",
		Required:    false,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"summary": {
					Type:        "string",
					Description: "Overall assessment of the document's current state.",
				},
				"key_findings": {
					Type:        "array",
					Description: "List of the most important observations or changes made.",
					Items: &SchemaProperty{
						Type: "string",
					},
				},
			},
			Required: []string{"summary"},
		},
	}
}
