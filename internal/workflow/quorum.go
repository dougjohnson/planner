package workflow

import (
	"context"
	"database/sql"
	"fmt"
)

// QuorumResult describes whether the parallel generation quorum is satisfied.
type QuorumResult struct {
	Satisfied    bool   `json:"satisfied"`
	Reason       string `json:"reason"`
	GPTSuccesses int    `json:"gpt_successes"`
	OpusSuccesses int   `json:"opus_successes"`
	GPTFailures  int    `json:"gpt_failures"`
	OpusFailures int    `json:"opus_failures"`
}

// QuorumChecker evaluates whether the parallel generation quorum (§3)
// is satisfied for synthesis stages.
type QuorumChecker struct {
	db *sql.DB
}

// NewQuorumChecker creates a new quorum checker.
func NewQuorumChecker(db *sql.DB) *QuorumChecker {
	return &QuorumChecker{db: db}
}

// CheckParallelQuorum evaluates whether at least one GPT-family and one
// Opus-family run completed successfully for the given parallel stage.
// This is the 'parallelQuorumSatisfied' guard condition.
func (qc *QuorumChecker) CheckParallelQuorum(ctx context.Context, projectID, parallelStage string) (*QuorumResult, error) {
	result := &QuorumResult{}

	// Count successful GPT-family runs for this stage.
	err := qc.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_runs wr
		 JOIN model_configs mc ON wr.model_config_id = mc.id
		 WHERE wr.project_id = ? AND wr.stage = ? AND wr.status = 'completed' AND mc.provider = 'openai'`,
		projectID, parallelStage,
	).Scan(&result.GPTSuccesses)
	if err != nil {
		return nil, fmt.Errorf("counting GPT successes: %w", err)
	}

	// Count successful Opus-family runs.
	err = qc.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_runs wr
		 JOIN model_configs mc ON wr.model_config_id = mc.id
		 WHERE wr.project_id = ? AND wr.stage = ? AND wr.status = 'completed' AND mc.provider = 'anthropic'`,
		projectID, parallelStage,
	).Scan(&result.OpusSuccesses)
	if err != nil {
		return nil, fmt.Errorf("counting Opus successes: %w", err)
	}

	// Count failures for diagnostics.
	qc.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_runs wr
		 JOIN model_configs mc ON wr.model_config_id = mc.id
		 WHERE wr.project_id = ? AND wr.stage = ? AND wr.status = 'failed' AND mc.provider = 'openai'`,
		projectID, parallelStage,
	).Scan(&result.GPTFailures)

	qc.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_runs wr
		 JOIN model_configs mc ON wr.model_config_id = mc.id
		 WHERE wr.project_id = ? AND wr.stage = ? AND wr.status = 'failed' AND mc.provider = 'anthropic'`,
		projectID, parallelStage,
	).Scan(&result.OpusFailures)

	// Evaluate quorum.
	if result.GPTSuccesses >= 1 && result.OpusSuccesses >= 1 {
		result.Satisfied = true
		result.Reason = fmt.Sprintf("quorum met: %d GPT + %d Opus successful runs",
			result.GPTSuccesses, result.OpusSuccesses)
	} else if result.GPTSuccesses == 0 && result.OpusSuccesses == 0 {
		result.Reason = "quorum not met: no successful runs from either model family"
	} else if result.GPTSuccesses == 0 {
		result.Reason = "quorum not met: no successful GPT-family runs"
	} else {
		result.Reason = "quorum not met: no successful Opus-family runs"
	}

	return result, nil
}

// CanProceedWithOverride checks if the user can override a failed quorum.
// Override is allowed when at least one model family succeeded — proceeding
// with partial results is valid when the user explicitly acknowledges the gap.
func (qr *QuorumResult) CanProceedWithOverride() bool {
	return qr.GPTSuccesses > 0 || qr.OpusSuccesses > 0
}
