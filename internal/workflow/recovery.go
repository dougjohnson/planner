package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// RecoverInterruptedRuns finds all workflow runs left in "running" status from
// a prior process crash and marks them as "interrupted". This MUST be called
// before any new workflow actions are accepted (§6.5 startup constraint).
//
// Returns the number of runs marked as interrupted.
func RecoverInterruptedRuns(ctx context.Context, db *sql.DB, logger *slog.Logger) (int, error) {
	repo := NewRunRepository(db)

	// First find what's interrupted for logging.
	interrupted, err := repo.FindInterruptedRuns(ctx)
	if err != nil {
		return 0, fmt.Errorf("finding interrupted runs: %w", err)
	}

	if len(interrupted) == 0 {
		logger.Info("startup recovery: no interrupted runs found")
		return 0, nil
	}

	// Log each interrupted run before marking.
	for _, run := range interrupted {
		logger.Warn("startup recovery: marking interrupted run",
			"run_id", run.ID,
			"project_id", run.ProjectID,
			"stage", run.Stage,
			"attempt", run.Attempt,
			"started_at", run.StartedAt,
		)
	}

	// Mark all running runs as interrupted.
	count, err := repo.MarkInterrupted(ctx)
	if err != nil {
		return 0, fmt.Errorf("marking interrupted runs: %w", err)
	}

	logger.Info("startup recovery complete",
		"interrupted_count", count,
	)

	return count, nil
}
