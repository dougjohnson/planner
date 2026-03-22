// Package workflow defines the fixed, inspectable stage definitions and
// checkpoint logic for the flywheel-planner workflow engine.
package workflow

// StageCategory classifies stages by their role in the workflow.
type StageCategory string

const (
	CategoryFoundations        StageCategory = "foundations"
	CategoryIntake             StageCategory = "intake"
	CategoryParallelGeneration StageCategory = "parallel_generation"
	CategorySynthesis          StageCategory = "synthesis"
	CategoryIntegration        StageCategory = "integration"
	CategoryReview             StageCategory = "review"
	CategoryReviewLoop         StageCategory = "review_loop"
	CategoryLoopControl        StageCategory = "loop_control"
	CategoryExport             StageCategory = "export"
	CategoryCommit             StageCategory = "commit"
)

// Transition defines a legal state change from one stage to another.
type Transition struct {
	ToStageID string `json:"to_stage_id"`
	Guard     string `json:"guard"`
}

// StagePolicy controls failure handling and recovery behavior for a stage.
type StagePolicy struct {
	MaxAutoRetries                int      `json:"max_auto_retries"`
	AllowsPartialSuccess          bool     `json:"allows_partial_success"`
	AllowsManualOverride          bool     `json:"allows_manual_override"`
	ValidationRecoveryMode        string   `json:"validation_recovery_mode"`
	ContextOverflowMode           string   `json:"context_overflow_mode"`
	RequiredProviderFamilies      []string `json:"required_provider_families"`
	RequiresCanonicalBaseArtifact bool     `json:"requires_canonical_base_artifact"`
}

// StageDefinition is the executable specification for a single workflow step.
// The full set of 17 definitions encodes the entire product workflow contract
// in a single inspectable, testable data structure.
type StageDefinition struct {
	ID                  string        `json:"id"`
	PRDNumber           int           `json:"prd_number"`
	Name                string        `json:"name"`
	Category            StageCategory `json:"category"`
	RequiresModels      bool          `json:"requires_models"`
	RequiresUserInput   bool          `json:"requires_user_input"`
	IsParallel          bool          `json:"is_parallel"`
	IsLoopControl       bool          `json:"is_loop_control"`
	RequiredInputTypes  []string      `json:"required_input_types"`
	OutputArtifactTypes []string      `json:"output_artifact_types"`
	PromptTemplateNames []string      `json:"prompt_template_names"`
	ToolNames           []string      `json:"tool_names"`
	NextTransitions     []Transition  `json:"next_transitions"`
	Policy              StagePolicy   `json:"policy"`
}

// AllStages returns the immutable, ordered list of all 17 stage definitions.
// This is the product contract — it must not be modified at runtime.
func AllStages() []StageDefinition {
	return []StageDefinition{
		stage1(), stage2(), stage3(), stage4(), stage5(),
		stage6(), stage7(), stage8(), stage9(), stage10(),
		stage11(), stage12(), stage13(), stage14(), stage15(),
		stage16(), stage17(),
	}
}

// StageByNumber returns the stage definition for the given PRD number (1-17),
// or nil if out of range.
func StageByNumber(n int) *StageDefinition {
	stages := AllStages()
	for i := range stages {
		if stages[i].PRDNumber == n {
			return &stages[i]
		}
	}
	return nil
}

// StageByID returns the stage definition with the given ID, or nil if not found.
func StageByID(id string) *StageDefinition {
	stages := AllStages()
	for i := range stages {
		if stages[i].ID == id {
			return &stages[i]
		}
	}
	return nil
}

// --- Stage Definitions ---

func stage1() StageDefinition {
	return StageDefinition{
		ID:                "foundations",
		PRDNumber:         1,
		Name:              "Foundations and Project Creation",
		Category:          CategoryFoundations,
		RequiresModels:    false,
		RequiresUserInput: true,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"project_name", "tech_stack", "architecture_direction",
		},
		OutputArtifactTypes: []string{
			"agents_md", "tech_stack_file", "architecture_direction_file", "best_practice_guides",
		},
		PromptTemplateNames: nil,
		ToolNames:           nil,
		NextTransitions: []Transition{
			{ToStageID: "prd_intake", Guard: "foundationsApproved"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:       0,
			AllowsPartialSuccess: false,
			AllowsManualOverride: false,
		},
	}
}

func stage2() StageDefinition {
	return StageDefinition{
		ID:                "prd_intake",
		PRDNumber:         2,
		Name:              "Initial PRD Intake",
		Category:          CategoryIntake,
		RequiresModels:    false,
		RequiresUserInput: true,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"seed_prd_markdown",
		},
		OutputArtifactTypes: []string{
			"seed_prd",
		},
		PromptTemplateNames: nil,
		ToolNames:           nil,
		NextTransitions: []Transition{
			{ToStageID: "parallel_prd_generation", Guard: "seedPrdSubmitted"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:       0,
			AllowsPartialSuccess: false,
			AllowsManualOverride: false,
		},
	}
}

func stage3() StageDefinition {
	return StageDefinition{
		ID:                "parallel_prd_generation",
		PRDNumber:         3,
		Name:              "Parallel PRD Generation",
		Category:          CategoryParallelGeneration,
		RequiresModels:    true,
		RequiresUserInput: false,
		IsParallel:        true,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"seed_prd", "foundation_context",
		},
		OutputArtifactTypes: []string{
			"generated_prd",
		},
		PromptTemplateNames: []string{
			"PRD_EXPANSION_V1",
		},
		ToolNames: []string{
			"submit_document",
		},
		NextTransitions: []Transition{
			{ToStageID: "prd_synthesis", Guard: "parallelQuorumSatisfied"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:           2,
			AllowsPartialSuccess:     false,
			AllowsManualOverride:     true,
			ValidationRecoveryMode:   "bounded_retry",
			RequiredProviderFamilies: []string{"gpt", "opus"},
		},
	}
}

func stage4() StageDefinition {
	return StageDefinition{
		ID:                "prd_synthesis",
		PRDNumber:         4,
		Name:              "GPT Extended-Reasoning PRD Synthesis",
		Category:          CategorySynthesis,
		RequiresModels:    true,
		RequiresUserInput: false,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"generated_prd",
		},
		OutputArtifactTypes: []string{
			"synthesized_prd", "change_rationale",
		},
		PromptTemplateNames: []string{
			"GPT_PRD_SYNTHESIS_V1",
		},
		ToolNames: []string{
			"submit_document", "submit_change_rationale",
		},
		NextTransitions: []Transition{
			{ToStageID: "prd_integration", Guard: "runCompleted"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:                2,
			AllowsPartialSuccess:          true,
			AllowsManualOverride:          true,
			ValidationRecoveryMode:        "bounded_retry",
			RequiresCanonicalBaseArtifact: true,
		},
	}
}

func stage5() StageDefinition {
	return StageDefinition{
		ID:                "prd_integration",
		PRDNumber:         5,
		Name:              "Opus Integration Pass on PRD",
		Category:          CategoryIntegration,
		RequiresModels:    true,
		RequiresUserInput: false,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"synthesized_prd", "fragment_diff",
		},
		OutputArtifactTypes: []string{
			"integrated_prd", "agreement_report", "disagreement_report",
		},
		PromptTemplateNames: []string{
			"OPUS_PRD_INTEGRATION_V1",
		},
		ToolNames: []string{
			"submit_document", "report_agreement", "report_disagreement",
		},
		NextTransitions: []Transition{
			{ToStageID: "prd_disagreement_review", Guard: "hasDisagreements"},
			{ToStageID: "prd_review", Guard: "noDisagreements"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:                2,
			AllowsPartialSuccess:          true,
			AllowsManualOverride:          true,
			ValidationRecoveryMode:        "bounded_retry",
			RequiresCanonicalBaseArtifact: true,
		},
	}
}

func stage6() StageDefinition {
	return StageDefinition{
		ID:                "prd_disagreement_review",
		PRDNumber:         6,
		Name:              "User Review of PRD Disagreements",
		Category:          CategoryReview,
		RequiresModels:    false,
		RequiresUserInput: true,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"disagreement_report", "integrated_prd",
		},
		OutputArtifactTypes: []string{
			"review_decisions", "resolved_prd",
		},
		PromptTemplateNames: nil,
		ToolNames:           nil,
		NextTransitions: []Transition{
			{ToStageID: "prd_review", Guard: "allDecisionsMade"},
		},
		Policy: StagePolicy{
			AllowsManualOverride: true,
		},
	}
}

func stage7() StageDefinition {
	return StageDefinition{
		ID:                "prd_review",
		PRDNumber:         7,
		Name:              "PRD Review Pass",
		Category:          CategoryReviewLoop,
		RequiresModels:    true,
		RequiresUserInput: false,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"canonical_prd",
		},
		OutputArtifactTypes: []string{
			"fragment_operations", "review_summary",
		},
		PromptTemplateNames: []string{
			"GPT_PRD_REVIEW_V1", "OPUS_PRD_REVIEW_V1",
		},
		ToolNames: []string{
			"update_fragment", "add_fragment", "remove_fragment", "submit_review_summary",
		},
		NextTransitions: []Transition{
			{ToStageID: "prd_commit", Guard: "fragmentOperationsRecorded"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:                2,
			AllowsPartialSuccess:          true,
			AllowsManualOverride:          true,
			ValidationRecoveryMode:        "bounded_retry",
			RequiresCanonicalBaseArtifact: true,
		},
	}
}

func stage8() StageDefinition {
	return StageDefinition{
		ID:                "prd_commit",
		PRDNumber:         8,
		Name:              "Commit PRD Fragment Revisions",
		Category:          CategoryCommit,
		RequiresModels:    false,
		RequiresUserInput: false,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"fragment_operations",
		},
		OutputArtifactTypes: []string{
			"canonical_prd",
		},
		PromptTemplateNames: nil,
		ToolNames:           nil,
		NextTransitions: []Transition{
			{ToStageID: "prd_loop_control", Guard: "runCompleted"},
		},
		Policy: StagePolicy{
			RequiresCanonicalBaseArtifact: true,
		},
	}
}

func stage9() StageDefinition {
	return StageDefinition{
		ID:                "prd_loop_control",
		PRDNumber:         9,
		Name:              "PRD Improvement Loop Control",
		Category:          CategoryLoopControl,
		RequiresModels:    false,
		RequiresUserInput: true,
		IsParallel:        false,
		IsLoopControl:     true,
		RequiredInputTypes: nil,
		OutputArtifactTypes: nil,
		PromptTemplateNames: nil,
		ToolNames:           nil,
		NextTransitions: []Transition{
			{ToStageID: "prd_review", Guard: "loopNotExhausted"},
			{ToStageID: "parallel_plan_generation", Guard: "loopExhausted"},
			{ToStageID: "parallel_plan_generation", Guard: "loopConverged"},
		},
		Policy: StagePolicy{
			AllowsManualOverride: true,
		},
	}
}

func stage10() StageDefinition {
	return StageDefinition{
		ID:                "parallel_plan_generation",
		PRDNumber:         10,
		Name:              "Parallel Implementation Plan Generation",
		Category:          CategoryParallelGeneration,
		RequiresModels:    true,
		RequiresUserInput: false,
		IsParallel:        true,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"canonical_prd", "foundation_context",
		},
		OutputArtifactTypes: []string{
			"generated_plan",
		},
		PromptTemplateNames: []string{
			"PLAN_GENERATION_V1",
		},
		ToolNames: []string{
			"submit_document",
		},
		NextTransitions: []Transition{
			{ToStageID: "plan_synthesis", Guard: "parallelQuorumSatisfied"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:           2,
			AllowsPartialSuccess:     false,
			AllowsManualOverride:     true,
			ValidationRecoveryMode:   "bounded_retry",
			RequiredProviderFamilies: []string{"gpt", "opus"},
		},
	}
}

func stage11() StageDefinition {
	return StageDefinition{
		ID:                "plan_synthesis",
		PRDNumber:         11,
		Name:              "GPT Extended-Reasoning Plan Synthesis",
		Category:          CategorySynthesis,
		RequiresModels:    true,
		RequiresUserInput: false,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"generated_plan",
		},
		OutputArtifactTypes: []string{
			"synthesized_plan", "change_rationale",
		},
		PromptTemplateNames: []string{
			"GPT_PLAN_SYNTHESIS_V1",
		},
		ToolNames: []string{
			"submit_document", "submit_change_rationale",
		},
		NextTransitions: []Transition{
			{ToStageID: "plan_integration", Guard: "runCompleted"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:                2,
			AllowsPartialSuccess:          true,
			AllowsManualOverride:          true,
			ValidationRecoveryMode:        "bounded_retry",
			RequiresCanonicalBaseArtifact: true,
		},
	}
}

func stage12() StageDefinition {
	return StageDefinition{
		ID:                "plan_integration",
		PRDNumber:         12,
		Name:              "Opus Integration Pass on Plan",
		Category:          CategoryIntegration,
		RequiresModels:    true,
		RequiresUserInput: false,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"synthesized_plan", "fragment_diff",
		},
		OutputArtifactTypes: []string{
			"integrated_plan", "agreement_report", "disagreement_report",
		},
		PromptTemplateNames: []string{
			"OPUS_PLAN_INTEGRATION_V1",
		},
		ToolNames: []string{
			"submit_document", "report_agreement", "report_disagreement",
		},
		NextTransitions: []Transition{
			{ToStageID: "plan_disagreement_review", Guard: "hasDisagreements"},
			{ToStageID: "plan_review", Guard: "noDisagreements"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:                2,
			AllowsPartialSuccess:          true,
			AllowsManualOverride:          true,
			ValidationRecoveryMode:        "bounded_retry",
			RequiresCanonicalBaseArtifact: true,
		},
	}
}

func stage13() StageDefinition {
	return StageDefinition{
		ID:                "plan_disagreement_review",
		PRDNumber:         13,
		Name:              "User Review of Plan Disagreements",
		Category:          CategoryReview,
		RequiresModels:    false,
		RequiresUserInput: true,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"disagreement_report", "integrated_plan",
		},
		OutputArtifactTypes: []string{
			"review_decisions", "resolved_plan",
		},
		PromptTemplateNames: nil,
		ToolNames:           nil,
		NextTransitions: []Transition{
			{ToStageID: "plan_review", Guard: "allDecisionsMade"},
		},
		Policy: StagePolicy{
			AllowsManualOverride: true,
		},
	}
}

func stage14() StageDefinition {
	return StageDefinition{
		ID:                "plan_review",
		PRDNumber:         14,
		Name:              "Plan Review Pass",
		Category:          CategoryReviewLoop,
		RequiresModels:    true,
		RequiresUserInput: false,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"canonical_plan",
		},
		OutputArtifactTypes: []string{
			"fragment_operations", "review_summary",
		},
		PromptTemplateNames: []string{
			"GPT_PLAN_REVIEW_V1", "OPUS_PLAN_REVIEW_V1",
		},
		ToolNames: []string{
			"update_fragment", "add_fragment", "remove_fragment", "submit_review_summary",
		},
		NextTransitions: []Transition{
			{ToStageID: "plan_commit", Guard: "fragmentOperationsRecorded"},
		},
		Policy: StagePolicy{
			MaxAutoRetries:                2,
			AllowsPartialSuccess:          true,
			AllowsManualOverride:          true,
			ValidationRecoveryMode:        "bounded_retry",
			RequiresCanonicalBaseArtifact: true,
		},
	}
}

func stage15() StageDefinition {
	return StageDefinition{
		ID:                "plan_commit",
		PRDNumber:         15,
		Name:              "Commit Plan Fragment Revisions",
		Category:          CategoryCommit,
		RequiresModels:    false,
		RequiresUserInput: false,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"fragment_operations",
		},
		OutputArtifactTypes: []string{
			"canonical_plan",
		},
		PromptTemplateNames: nil,
		ToolNames:           nil,
		NextTransitions: []Transition{
			{ToStageID: "plan_loop_control", Guard: "runCompleted"},
		},
		Policy: StagePolicy{
			RequiresCanonicalBaseArtifact: true,
		},
	}
}

func stage16() StageDefinition {
	return StageDefinition{
		ID:                "plan_loop_control",
		PRDNumber:         16,
		Name:              "Plan Improvement Loop Control",
		Category:          CategoryLoopControl,
		RequiresModels:    false,
		RequiresUserInput: true,
		IsParallel:        false,
		IsLoopControl:     true,
		RequiredInputTypes: nil,
		OutputArtifactTypes: nil,
		PromptTemplateNames: nil,
		ToolNames:           nil,
		NextTransitions: []Transition{
			{ToStageID: "plan_review", Guard: "loopNotExhausted"},
			{ToStageID: "final_export", Guard: "loopExhausted"},
			{ToStageID: "final_export", Guard: "loopConverged"},
		},
		Policy: StagePolicy{
			AllowsManualOverride: true,
		},
	}
}

func stage17() StageDefinition {
	return StageDefinition{
		ID:                "final_export",
		PRDNumber:         17,
		Name:              "Final Review and Export",
		Category:          CategoryExport,
		RequiresModels:    false,
		RequiresUserInput: true,
		IsParallel:        false,
		IsLoopControl:     false,
		RequiredInputTypes: []string{
			"canonical_prd", "canonical_plan", "foundation_context",
		},
		OutputArtifactTypes: []string{
			"export_bundle", "export_manifest",
		},
		PromptTemplateNames: nil,
		ToolNames:           nil,
		NextTransitions:     nil,
		Policy: StagePolicy{
			AllowsManualOverride: true,
		},
	}
}
