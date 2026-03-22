package workflow

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// LoopConfig holds configuration for a review improvement loop.
type LoopConfig struct {
	// DocumentType is "prd" or "plan".
	DocumentType string `json:"document_type"`
	// ReviewStageID is the stage ID for review passes (e.g., "prd_review" or "plan_review").
	ReviewStageID string `json:"review_stage_id"`
	// CommitStageID is the stage ID for commit passes (e.g., "prd_commit" or "plan_commit").
	CommitStageID string `json:"commit_stage_id"`
	// LoopControlStageID is "prd_loop_control" or "plan_loop_control".
	LoopControlStageID string `json:"loop_control_stage_id"`
	// ReviewPromptTemplates are the prompt template names for review (GPT + Opus variants).
	ReviewPromptTemplates []string `json:"review_prompt_templates"`
	// MaxIterations is the total number of review loops (default 4).
	MaxIterations int `json:"max_iterations"`
	// PauseBetweenLoops enables a user pause after each iteration.
	PauseBetweenLoops bool `json:"pause_between_loops"`
}

// PRDLoopConfig returns the default loop configuration for PRD review (Stages 7-9).
func PRDLoopConfig() LoopConfig {
	return LoopConfig{
		DocumentType:       "prd",
		ReviewStageID:      "prd_review",
		CommitStageID:      "prd_commit",
		LoopControlStageID: "prd_loop_control",
		ReviewPromptTemplates: []string{
			"GPT_PRD_REVIEW_V1", "OPUS_PRD_REVIEW_V1",
		},
		MaxIterations: 4,
	}
}

// PlanLoopConfig returns the default loop configuration for plan review (Stages 14-16).
func PlanLoopConfig() LoopConfig {
	return LoopConfig{
		DocumentType:       "plan",
		ReviewStageID:      "plan_review",
		CommitStageID:      "plan_commit",
		LoopControlStageID: "plan_loop_control",
		ReviewPromptTemplates: []string{
			"GPT_PLAN_REVIEW_V1", "OPUS_PLAN_REVIEW_V1",
		},
		MaxIterations: 4,
	}
}

// LoopState tracks the current state of a review improvement loop.
type LoopState struct {
	Config           LoopConfig        `json:"config"`
	CurrentIteration int               `json:"current_iteration"`
	Convergence      ConvergenceStatus `json:"convergence"`
	// ModelForIteration returns "gpt" or "opus" for model rotation.
	// Default: GPT for all, Opus at midpoint.
	Iterations []LoopIterationRecord `json:"iterations"`
}

// LoopIterationRecord captures metadata for a single loop iteration.
type LoopIterationRecord struct {
	Number         int       `json:"number"`
	ModelFamily    string    `json:"model_family"`
	ReviewRunID    string    `json:"review_run_id"`
	CommitRunID    string    `json:"commit_run_id"`
	OperationCount int       `json:"operation_count"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at"`
}

// LoopAction indicates what the loop controller should do next.
type LoopAction string

const (
	LoopActionContinue       LoopAction = "continue"        // run next review+commit
	LoopActionPauseForUser   LoopAction = "pause_for_user"   // pause between loops
	LoopActionConverged      LoopAction = "converged"        // zero ops, offer early exit
	LoopActionExhausted      LoopAction = "exhausted"        // max iterations reached
)

// NewLoopState creates a fresh loop state from configuration.
func NewLoopState(config LoopConfig) *LoopState {
	return &LoopState{
		Config:           config,
		CurrentIteration: 0,
		Convergence:      ConvergenceNone,
	}
}

// LoadLoopConfig reads the project's loop configuration from the database,
// falling back to defaults.
func LoadLoopConfig(ctx context.Context, db *sql.DB, projectID, documentType string) LoopConfig {
	var config LoopConfig
	if documentType == "plan" {
		config = PlanLoopConfig()
	} else {
		config = PRDLoopConfig()
	}

	// Override from database if configured.
	var maxIter int
	var pauseLoops int
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(iteration_count, ?), COALESCE(pause_between_loops, 0)
		 FROM loop_configs WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`,
		config.MaxIterations, projectID).Scan(&maxIter, &pauseLoops)
	if err == nil {
		config.MaxIterations = maxIter
		config.PauseBetweenLoops = pauseLoops == 1
	}

	return config
}

// NextAction determines what the loop controller should do after a commit pass.
func (ls *LoopState) NextAction(commitResult *CommitResult) LoopAction {
	ls.CurrentIteration++

	// Check convergence.
	conv := CheckConvergence(commitResult, ls.CurrentIteration, ls.Config.MaxIterations)
	if conv.Status == ConvergenceDetected {
		ls.Convergence = ConvergenceDetected
		return LoopActionConverged
	}

	// Check exhaustion.
	if ls.CurrentIteration >= ls.Config.MaxIterations {
		return LoopActionExhausted
	}

	// Check pause setting.
	if ls.Config.PauseBetweenLoops {
		return LoopActionPauseForUser
	}

	return LoopActionContinue
}

// ModelFamilyForIteration returns which model family to use for the given
// iteration. Default: GPT for all iterations, Opus at the midpoint for
// model diversity (§10.3 Stage 9 Loop Model Rotation).
func (ls *LoopState) ModelFamilyForIteration(iteration int) string {
	midpoint := ls.Config.MaxIterations / 2
	if midpoint > 0 && iteration == midpoint {
		return "opus"
	}
	return "gpt"
}

// PromptTemplateForFamily returns the prompt template name for the given
// model family from the loop configuration.
func (ls *LoopState) PromptTemplateForFamily(family string) string {
	if family == "opus" && len(ls.Config.ReviewPromptTemplates) > 1 {
		return ls.Config.ReviewPromptTemplates[1]
	}
	if len(ls.Config.ReviewPromptTemplates) > 0 {
		return ls.Config.ReviewPromptTemplates[0]
	}
	return ""
}

// RecordIteration adds an iteration record to the loop state.
func (ls *LoopState) RecordIteration(record LoopIterationRecord) {
	ls.Iterations = append(ls.Iterations, record)
}

// LogLoopStatus logs the current loop state for observability.
func (ls *LoopState) LogLoopStatus(logger *slog.Logger) {
	logger.Info("loop status",
		"document_type", ls.Config.DocumentType,
		"iteration", ls.CurrentIteration,
		"max", ls.Config.MaxIterations,
		"convergence", ls.Convergence,
		"completed_iterations", len(ls.Iterations),
	)
}
