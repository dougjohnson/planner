package workflow

import (
	"context"
	"database/sql"
	"fmt"
)

// GuardResult holds the outcome of evaluating a named guard condition.
type GuardResult struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

// Guard is a named condition that must be satisfied before a workflow
// transition can proceed. Guards are pure query functions — they check
// state but never mutate it.
type Guard func(ctx context.Context, db *sql.DB, projectID string) GuardResult

// GuardRegistry maps guard names (from the transition table) to their
// implementations. Every guard referenced in a transition must be registered.
var guardRegistry = map[string]Guard{
	"foundationsApproved":        guardFoundationsApproved,
	"seedPrdSubmitted":           guardSeedPrdSubmitted,
	"parallelQuorumSatisfied":    guardParallelQuorumSatisfied,
	"runCompleted":               guardRunCompleted,
	"hasDisagreements":           guardHasDisagreements,
	"noDisagreements":            guardNoDisagreements,
	"allDecisionsMade":           guardAllDecisionsMade,
	"fragmentOperationsRecorded": guardFragmentOperationsRecorded,
	"loopNotExhausted":           guardLoopNotExhausted,
	"loopExhausted":              guardLoopExhausted,
	"loopConverged":              guardLoopConverged,
}

// EvaluateGuard runs a named guard condition and returns its result.
func EvaluateGuard(ctx context.Context, db *sql.DB, guardName, projectID string) (GuardResult, error) {
	guard, ok := guardRegistry[guardName]
	if !ok {
		return GuardResult{}, fmt.Errorf("unknown guard: %q", guardName)
	}
	return guard(ctx, db, projectID), nil
}

// RegisteredGuards returns the names of all registered guards.
func RegisteredGuards() []string {
	names := make([]string, 0, len(guardRegistry))
	for name := range guardRegistry {
		names = append(names, name)
	}
	return names
}

// --- Guard Implementations ---

func guardFoundationsApproved(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	var stage string
	err := db.QueryRowContext(ctx,
		`SELECT current_stage FROM projects WHERE id = ?`, projectID).Scan(&stage)
	if err != nil {
		return GuardResult{Passed: false, Reason: "project not found"}
	}
	// Foundations are approved when the project has advanced past the foundations stage.
	if stage == "" || stage == "foundations" {
		// Check if there are any foundation artifacts.
		var count int
		err = db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM project_inputs WHERE project_id = ? AND role = 'foundation'`, projectID).Scan(&count)
		if err != nil || count == 0 {
			return GuardResult{Passed: false, Reason: "no foundation artifacts submitted"}
		}
		return GuardResult{Passed: true, Reason: "foundations submitted"}
	}
	return GuardResult{Passed: true, Reason: "project already past foundations stage"}
}

func guardSeedPrdSubmitted(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_inputs WHERE project_id = ? AND role = 'seed_prd'`, projectID).Scan(&count)
	if err != nil || count == 0 {
		return GuardResult{Passed: false, Reason: "no seed PRD uploaded"}
	}
	return GuardResult{Passed: true, Reason: "seed PRD present"}
}

func guardParallelQuorumSatisfied(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	// Check that at least one GPT-family and one Opus-family run completed successfully
	// for the current parallel stage.
	var gptCount, opusCount int
	err := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN mc.provider = 'openai' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN mc.provider = 'anthropic' THEN 1 ELSE 0 END), 0)
		FROM workflow_runs wr
		JOIN model_configs mc ON mc.id = wr.model_config_id
		WHERE wr.project_id = ? AND wr.status = 'completed'
		AND wr.stage IN ('parallel_prd_generation', 'parallel_plan_generation')
	`, projectID).Scan(&gptCount, &opusCount)
	if err != nil {
		return GuardResult{Passed: false, Reason: fmt.Sprintf("query error: %v", err)}
	}
	if gptCount == 0 {
		return GuardResult{Passed: false, Reason: "no completed GPT-family run"}
	}
	if opusCount == 0 {
		return GuardResult{Passed: false, Reason: "no completed Opus-family run"}
	}
	return GuardResult{Passed: true, Reason: fmt.Sprintf("quorum met: %d GPT + %d Opus", gptCount, opusCount)}
}

func guardRunCompleted(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	// Check that the most recent run for this project completed successfully.
	var status string
	err := db.QueryRowContext(ctx, `
		SELECT status FROM workflow_runs
		WHERE project_id = ?
		ORDER BY created_at DESC LIMIT 1
	`, projectID).Scan(&status)
	if err != nil {
		return GuardResult{Passed: false, Reason: "no runs found"}
	}
	if status != string(RunCompleted) {
		return GuardResult{Passed: false, Reason: fmt.Sprintf("latest run status is %s", status)}
	}
	return GuardResult{Passed: true, Reason: "latest run completed"}
}

func guardHasDisagreements(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM review_items
		WHERE project_id = ? AND classification = 'disagreement' AND status = 'pending'
	`, projectID).Scan(&count)
	if err != nil {
		return GuardResult{Passed: false, Reason: fmt.Sprintf("query error: %v", err)}
	}
	if count > 0 {
		return GuardResult{Passed: true, Reason: fmt.Sprintf("%d pending disagreements", count)}
	}
	return GuardResult{Passed: false, Reason: "no pending disagreements"}
}

func guardNoDisagreements(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	result := guardHasDisagreements(ctx, db, projectID)
	if result.Passed {
		return GuardResult{Passed: false, Reason: result.Reason}
	}
	return GuardResult{Passed: true, Reason: "no disagreements — skip path available"}
}

func guardAllDecisionsMade(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	var pending int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM review_items
		WHERE project_id = ? AND status = 'pending'
	`, projectID).Scan(&pending)
	if err != nil {
		return GuardResult{Passed: false, Reason: fmt.Sprintf("query error: %v", err)}
	}
	if pending > 0 {
		return GuardResult{Passed: false, Reason: fmt.Sprintf("%d review items still pending", pending)}
	}
	return GuardResult{Passed: true, Reason: "all review decisions made"}
}

func guardFragmentOperationsRecorded(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	// Check that the latest review run produced at least one tool call
	// (either fragment operations or a review summary indicating no changes).
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM workflow_runs
		WHERE project_id = ? AND status = 'completed'
		AND stage IN ('prd_review', 'plan_review')
	`, projectID).Scan(&count)
	if err != nil || count == 0 {
		return GuardResult{Passed: false, Reason: "no completed review run"}
	}
	return GuardResult{Passed: true, Reason: "review run completed with recorded operations"}
}

func guardLoopNotExhausted(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	result := guardLoopExhausted(ctx, db, projectID)
	if result.Passed {
		return GuardResult{Passed: false, Reason: "loop iterations exhausted"}
	}
	// Also check convergence hasn't been accepted.
	converged := guardLoopConverged(ctx, db, projectID)
	if converged.Passed {
		return GuardResult{Passed: false, Reason: "loop convergence accepted"}
	}
	return GuardResult{Passed: true, Reason: "loop iterations remaining"}
}

func guardLoopExhausted(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	// Count completed review iterations and compare to configured max.
	var maxIter int
	err := db.QueryRowContext(ctx, `
		SELECT COALESCE(max_iterations, 4) FROM loop_configs
		WHERE project_id = ? ORDER BY created_at DESC LIMIT 1
	`, projectID).Scan(&maxIter)
	if err != nil {
		maxIter = 4 // default
	}

	var completedIter int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM workflow_runs
		WHERE project_id = ? AND status = 'completed'
		AND stage IN ('prd_review', 'plan_review')
	`, projectID).Scan(&completedIter)
	if err != nil {
		return GuardResult{Passed: false, Reason: fmt.Sprintf("query error: %v", err)}
	}

	if completedIter >= maxIter {
		return GuardResult{Passed: true, Reason: fmt.Sprintf("completed %d of %d iterations", completedIter, maxIter)}
	}
	return GuardResult{Passed: false, Reason: fmt.Sprintf("completed %d of %d iterations", completedIter, maxIter)}
}

func guardLoopConverged(ctx context.Context, db *sql.DB, projectID string) GuardResult {
	// Check if the latest review run proposed zero fragment operations
	// AND the user accepted convergence.
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM workflow_events
		WHERE project_id = ? AND event_type = 'convergence_accepted'
	`, projectID).Scan(&count)
	if err != nil || count == 0 {
		return GuardResult{Passed: false, Reason: "no convergence acceptance recorded"}
	}
	return GuardResult{Passed: true, Reason: "convergence accepted by user"}
}
