package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoverInterruptedRuns_NoRuns(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	logger := testutil.NewTestLogger(t)

	count, err := RecoverInterruptedRuns(context.Background(), tdb.DB, logger)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestRecoverInterruptedRuns_WithRunning(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	logger := testutil.NewTestLogger(t)
	ctx := context.Background()

	// Setup: project + model config + running runs.
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-1', 'openai', 'gpt-4o', datetime('now'), datetime('now'))`)

	repo := NewRunRepository(tdb.DB)
	run1, err := repo.Create(ctx, "p-1", "parallel_prd_generation", "mc-1")
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(ctx, run1.ID, RunRunning))

	run2, err := repo.Create(ctx, "p-1", "prd_synthesis", "mc-1")
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(ctx, run2.ID, RunRunning))

	// A pending run should NOT be affected.
	_, err = repo.Create(ctx, "p-1", "prd_review", "mc-1")
	require.NoError(t, err)

	// Recover.
	count, err := RecoverInterruptedRuns(ctx, tdb.DB, logger)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify runs are now interrupted.
	r1, _ := repo.GetByID(ctx, run1.ID)
	assert.Equal(t, RunInterrupted, r1.Status)
	assert.NotNil(t, r1.CompletedAt)

	r2, _ := repo.GetByID(ctx, run2.ID)
	assert.Equal(t, RunInterrupted, r2.Status)
}

func TestRecoverInterruptedRuns_Idempotent(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	logger := testutil.NewTestLogger(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-1', 'openai', 'gpt-4o', datetime('now'), datetime('now'))`)

	repo := NewRunRepository(tdb.DB)
	run, err := repo.Create(ctx, "p-1", "prd_review", "mc-1")
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(ctx, run.ID, RunRunning))

	// First recovery.
	count, err := RecoverInterruptedRuns(ctx, tdb.DB, logger)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Second recovery should find nothing.
	count, err = RecoverInterruptedRuns(ctx, tdb.DB, logger)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
