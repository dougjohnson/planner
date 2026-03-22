package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

// RecoveryAction indicates what the engine should do after validation.
type RecoveryAction string

const (
	RecoveryProceed      RecoveryAction = "proceed"       // all valid, continue
	RecoveryRetry        RecoveryAction = "retry"         // targeted retry with error feedback
	RecoveryPartial      RecoveryAction = "partial"       // accept valid calls, skip invalid
	RecoveryUserRequired RecoveryAction = "user_required" // exhausted, route to user
)

// RecoveryDecision holds the outcome of the recovery ladder evaluation.
type RecoveryDecision struct {
	Action           RecoveryAction                   `json:"action"`
	ValidResults     []models.NormalizedToolCallResult `json:"valid_results,omitempty"`
	InvalidResults   []models.NormalizedToolCallResult `json:"invalid_results,omitempty"`
	RetryMessage     string                           `json:"retry_message,omitempty"`
	AttemptNumber    int                              `json:"attempt_number"`
	AllAttempts      [][]models.NormalizedToolCallResult `json:"all_attempts,omitempty"`
}

// RecoveryConfig controls the behavior of the recovery ladder.
type RecoveryConfig struct {
	MaxRetries           int  `json:"max_retries"`
	AllowPartialAccept   bool `json:"allow_partial_accept"`
	RequiredToolNames    []string `json:"required_tool_names,omitempty"`
}

// DefaultRecoveryConfig returns sensible defaults for the recovery ladder.
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		MaxRetries:         2,
		AllowPartialAccept: true,
	}
}

// EvaluateRecovery runs the bounded recovery ladder (§11.4.3) on a set of
// normalized tool-call results.
func EvaluateRecovery(
	results []models.NormalizedToolCallResult,
	config RecoveryConfig,
	attemptNumber int,
	priorAttempts [][]models.NormalizedToolCallResult,
) RecoveryDecision {
	var valid, invalid []models.NormalizedToolCallResult
	for _, r := range results {
		if r.Valid {
			valid = append(valid, r)
		} else {
			invalid = append(invalid, r)
		}
	}

	allAttempts := append(priorAttempts, results)

	// All valid → proceed.
	if len(invalid) == 0 {
		// Check required tools were called.
		if missing := checkRequiredTools(valid, config.RequiredToolNames); len(missing) > 0 {
			if attemptNumber < config.MaxRetries {
				return RecoveryDecision{
					Action:        RecoveryRetry,
					ValidResults:  valid,
					InvalidResults: invalid,
					RetryMessage:  fmt.Sprintf("required tools not called: %s", strings.Join(missing, ", ")),
					AttemptNumber: attemptNumber,
					AllAttempts:   allAttempts,
				}
			}
			return RecoveryDecision{
				Action:        RecoveryUserRequired,
				ValidResults:  valid,
				AttemptNumber: attemptNumber,
				AllAttempts:   allAttempts,
			}
		}
		return RecoveryDecision{
			Action:       RecoveryProceed,
			ValidResults: valid,
			AttemptNumber: attemptNumber,
			AllAttempts:  allAttempts,
		}
	}

	// Step 1: Targeted retry if attempts remain.
	if attemptNumber < config.MaxRetries {
		return RecoveryDecision{
			Action:         RecoveryRetry,
			ValidResults:   valid,
			InvalidResults: invalid,
			RetryMessage:   buildRetryMessage(invalid),
			AttemptNumber:  attemptNumber,
			AllAttempts:    allAttempts,
		}
	}

	// Step 2: Partial acceptance if allowed and we have valid calls.
	if config.AllowPartialAccept && len(valid) > 0 {
		return RecoveryDecision{
			Action:         RecoveryPartial,
			ValidResults:   valid,
			InvalidResults: invalid,
			AttemptNumber:  attemptNumber,
			AllAttempts:    allAttempts,
		}
	}

	// Step 3: Route to user.
	return RecoveryDecision{
		Action:         RecoveryUserRequired,
		ValidResults:   valid,
		InvalidResults: invalid,
		AttemptNumber:  attemptNumber,
		AllAttempts:    allAttempts,
	}
}

// ValidateFragmentIDs checks that all fragment_id references in tool calls
// exist in the current canonical artifact's fragment set. Returns enriched
// results with validation errors for invalid IDs.
func ValidateFragmentIDs(ctx context.Context, db *sql.DB, projectID string, results []models.NormalizedToolCallResult) []models.NormalizedToolCallResult {
	// Load available fragment IDs for this project.
	availableIDs := loadFragmentIDs(ctx, db, projectID)
	availableSet := make(map[string]bool, len(availableIDs))
	for _, id := range availableIDs {
		availableSet[id] = true
	}

	enriched := make([]models.NormalizedToolCallResult, len(results))
	for i, r := range results {
		enriched[i] = r
		// Deep-copy ValidationErrors to avoid aliasing the caller's slice.
		if len(r.ValidationErrors) > 0 {
			enriched[i].ValidationErrors = make([]string, len(r.ValidationErrors))
			copy(enriched[i].ValidationErrors, r.ValidationErrors)
		}
	}

	for i := range enriched {
		args := enriched[i].ToolCall.Arguments

		// Validate fragment_id references.
		if fragID, ok := args["fragment_id"]; ok {
			if fragIDStr, ok := fragID.(string); ok && !availableSet[fragIDStr] {
				enriched[i].Valid = false
				enriched[i].ValidationErrors = append(enriched[i].ValidationErrors,
					fmt.Sprintf("fragment_id %q does not exist in the current document; available fragment IDs are: %s",
						fragIDStr, strings.Join(availableIDs, ", ")))
			}
		}

		// Also validate after_fragment_id references (used by add_fragment).
		if afterID, ok := args["after_fragment_id"]; ok {
			if afterIDStr, ok := afterID.(string); ok && !availableSet[afterIDStr] {
				enriched[i].Valid = false
				enriched[i].ValidationErrors = append(enriched[i].ValidationErrors,
					fmt.Sprintf("after_fragment_id %q does not exist in the current document",
						afterIDStr))
			}
		}
	}

	return enriched
}

// --- Helpers ---

func checkRequiredTools(valid []models.NormalizedToolCallResult, required []string) []string {
	if len(required) == 0 {
		return nil
	}
	called := make(map[string]bool)
	for _, r := range valid {
		called[r.ToolCall.Name] = true
	}
	var missing []string
	for _, name := range required {
		if !called[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

func buildRetryMessage(invalid []models.NormalizedToolCallResult) string {
	var msgs []string
	for _, r := range invalid {
		for _, e := range r.ValidationErrors {
			msgs = append(msgs, fmt.Sprintf("tool %q: %s", r.ToolCall.Name, e))
		}
	}
	return strings.Join(msgs, "; ")
}

func loadFragmentIDs(ctx context.Context, db *sql.DB, projectID string) []string {
	rows, err := db.QueryContext(ctx,
		`SELECT id FROM fragments WHERE project_id = ? ORDER BY id`, projectID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
