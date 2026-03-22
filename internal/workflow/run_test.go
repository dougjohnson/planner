package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRunTest(t *testing.T) (*RunRepository, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)

	// Insert test project and model config since workflow_runs has FKs to both.
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('proj-1', 'Test Project', 'desc', 'active', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('model-1', 'openai', 'gpt-4o', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('model-2', 'anthropic', 'claude-opus-4-6', datetime('now'), datetime('now'))`)

	return NewRunRepository(tdb.DB), context.Background()
}

func TestRunRepository_Create(t *testing.T) {
	repo, ctx := setupRunTest(t)

	run, err := repo.Create(ctx, "proj-1", "parallel_prd_generation", "model-1")
	require.NoError(t, err)
	require.NotNil(t, run)

	assert.NotEmpty(t, run.ID)
	assert.Equal(t, "proj-1", run.ProjectID)
	assert.Equal(t, "parallel_prd_generation", run.Stage)
	assert.Equal(t, RunPending, run.Status)
	assert.Equal(t, 1, run.Attempt)
	assert.Nil(t, run.StartedAt)
	assert.Nil(t, run.CompletedAt)
}

func TestRunRepository_GetByID(t *testing.T) {
	repo, ctx := setupRunTest(t)

	created, err := repo.Create(ctx, "proj-1", "prd_review", "")
	require.NoError(t, err)

	found, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, RunPending, found.Status)
}

func TestRunRepository_GetByID_NotFound(t *testing.T) {
	repo, ctx := setupRunTest(t)

	_, err := repo.GetByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRunRepository_UpdateStatus_HappyPath(t *testing.T) {
	repo, ctx := setupRunTest(t)

	run, err := repo.Create(ctx, "proj-1", "parallel_prd_generation", "model-1")
	require.NoError(t, err)

	// pending -> running
	err = repo.UpdateStatus(ctx, run.ID, RunRunning)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, RunRunning, updated.Status)
	assert.NotNil(t, updated.StartedAt, "StartedAt should be set when transitioning to running")

	// running -> completed
	err = repo.UpdateStatus(ctx, run.ID, RunCompleted)
	require.NoError(t, err)

	completed, err := repo.GetByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, RunCompleted, completed.Status)
	assert.NotNil(t, completed.CompletedAt)
}

func TestRunRepository_UpdateStatus_IllegalTransition(t *testing.T) {
	repo, ctx := setupRunTest(t)

	run, err := repo.Create(ctx, "proj-1", "prd_synthesis", "")
	require.NoError(t, err)

	// pending -> completed is illegal (must go through running)
	err = repo.UpdateStatus(ctx, run.ID, RunCompleted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "illegal run transition")
}

func TestRunRepository_RecordAttempt(t *testing.T) {
	repo, ctx := setupRunTest(t)

	run, err := repo.Create(ctx, "proj-1", "prd_review", "model-1")
	require.NoError(t, err)
	assert.Equal(t, 1, run.Attempt)

	attempt, err := repo.RecordAttempt(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, attempt)

	attempt, err = repo.RecordAttempt(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, attempt)
}

func TestRunRepository_SetSessionHandle(t *testing.T) {
	repo, ctx := setupRunTest(t)

	run, err := repo.Create(ctx, "proj-1", "prd_synthesis", "model-1")
	require.NoError(t, err)

	err = repo.SetSessionHandle(ctx, run.ID, "sess_abc123", "continued")
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "sess_abc123", updated.SessionHandle)
	assert.Equal(t, "continued", updated.ContinuityMode)
}

func TestRunRepository_SetProviderRequestID(t *testing.T) {
	repo, ctx := setupRunTest(t)

	run, err := repo.Create(ctx, "proj-1", "parallel_prd_generation", "model-1")
	require.NoError(t, err)

	err = repo.SetProviderRequestID(ctx, run.ID, "req_xyz")
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "req_xyz", updated.ProviderRequestID)
}

func TestRunRepository_SetError(t *testing.T) {
	repo, ctx := setupRunTest(t)

	run, err := repo.Create(ctx, "proj-1", "prd_review", "")
	require.NoError(t, err)

	err = repo.SetError(ctx, run.ID, "context deadline exceeded")
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "context deadline exceeded", updated.ErrorMessage)
}

func TestRunRepository_ListByProjectStage(t *testing.T) {
	repo, ctx := setupRunTest(t)

	_, err := repo.Create(ctx, "proj-1", "prd_review", "model-1")
	require.NoError(t, err)
	_, err = repo.Create(ctx, "proj-1", "prd_review", "model-2")
	require.NoError(t, err)
	_, err = repo.Create(ctx, "proj-1", "prd_synthesis", "model-1")
	require.NoError(t, err)

	runs, err := repo.ListByProjectStage(ctx, "proj-1", "prd_review")
	require.NoError(t, err)
	assert.Len(t, runs, 2)

	runs, err = repo.ListByProjectStage(ctx, "proj-1", "prd_synthesis")
	require.NoError(t, err)
	assert.Len(t, runs, 1)
}

func TestRunRepository_MarkInterrupted(t *testing.T) {
	repo, ctx := setupRunTest(t)

	// Create two runs and move them to running.
	run1, err := repo.Create(ctx, "proj-1", "prd_review", "")
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(ctx, run1.ID, RunRunning))

	run2, err := repo.Create(ctx, "proj-1", "plan_review", "")
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(ctx, run2.ID, RunRunning))

	// Create a third run that stays pending (should not be affected).
	_, err = repo.Create(ctx, "proj-1", "prd_synthesis", "")
	require.NoError(t, err)

	// Mark interrupted (simulates startup recovery).
	count, err := repo.MarkInterrupted(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify the runs are now interrupted.
	updated1, err := repo.GetByID(ctx, run1.ID)
	require.NoError(t, err)
	assert.Equal(t, RunInterrupted, updated1.Status)
	assert.NotNil(t, updated1.CompletedAt)

	updated2, err := repo.GetByID(ctx, run2.ID)
	require.NoError(t, err)
	assert.Equal(t, RunInterrupted, updated2.Status)
}

func TestRunRepository_FindInterruptedRuns(t *testing.T) {
	repo, ctx := setupRunTest(t)

	run1, err := repo.Create(ctx, "proj-1", "prd_review", "")
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(ctx, run1.ID, RunRunning))

	runs, err := repo.FindInterruptedRuns(ctx)
	require.NoError(t, err)
	assert.Len(t, runs, 1)
	assert.Equal(t, run1.ID, runs[0].ID)
}
