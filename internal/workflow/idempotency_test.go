package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIdempotencyTest(t *testing.T) (*IdempotencyStore, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	return NewIdempotencyStore(tdb.DB), context.Background()
}

func TestIdempotencyStore_Acquire_NewKey(t *testing.T) {
	store, ctx := setupIdempotencyTest(t)

	existing, err := store.Acquire(ctx, "key-1", "p-1", "start_stage")
	require.NoError(t, err)
	assert.Nil(t, existing, "new key should return nil record")
}

func TestIdempotencyStore_Acquire_DuplicateKey(t *testing.T) {
	store, ctx := setupIdempotencyTest(t)

	// First acquire succeeds.
	_, err := store.Acquire(ctx, "key-1", "p-1", "start_stage")
	require.NoError(t, err)

	// Second acquire returns the existing record.
	existing, err := store.Acquire(ctx, "key-1", "p-1", "start_stage")
	assert.ErrorIs(t, err, ErrDuplicateCommand)
	require.NotNil(t, existing)
	assert.Equal(t, "key-1", existing.Key)
	assert.Equal(t, IdempotencyReceived, existing.Status)
}

func TestIdempotencyStore_Complete(t *testing.T) {
	store, ctx := setupIdempotencyTest(t)

	_, err := store.Acquire(ctx, "key-2", "p-1", "retry_stage")
	require.NoError(t, err)

	result := map[string]string{"run_id": "run-abc"}
	err = store.Complete(ctx, "key-2", result)
	require.NoError(t, err)

	rec, err := store.Get(ctx, "key-2")
	require.NoError(t, err)
	assert.Equal(t, IdempotencyCompleted, rec.Status)
	assert.NotNil(t, rec.CompletedAt)

	var parsed map[string]string
	err = json.Unmarshal(rec.ResultJSON, &parsed)
	require.NoError(t, err)
	assert.Equal(t, "run-abc", parsed["run_id"])
}

func TestIdempotencyStore_Fail(t *testing.T) {
	store, ctx := setupIdempotencyTest(t)

	_, err := store.Acquire(ctx, "key-3", "p-1", "export")
	require.NoError(t, err)

	err = store.Fail(ctx, "key-3", "export failed: disk full")
	require.NoError(t, err)

	rec, err := store.Get(ctx, "key-3")
	require.NoError(t, err)
	assert.Equal(t, IdempotencyFailed, rec.Status)
	assert.NotNil(t, rec.CompletedAt)
}

func TestIdempotencyStore_DuplicateReturnsCompletedResult(t *testing.T) {
	store, ctx := setupIdempotencyTest(t)

	// Acquire + complete.
	_, err := store.Acquire(ctx, "key-4", "p-1", "start_stage")
	require.NoError(t, err)
	err = store.Complete(ctx, "key-4", map[string]string{"status": "ok"})
	require.NoError(t, err)

	// Duplicate acquire returns the completed record.
	existing, err := store.Acquire(ctx, "key-4", "p-1", "start_stage")
	assert.ErrorIs(t, err, ErrDuplicateCommand)
	require.NotNil(t, existing)
	assert.Equal(t, IdempotencyCompleted, existing.Status)

	var parsed map[string]string
	json.Unmarshal(existing.ResultJSON, &parsed)
	assert.Equal(t, "ok", parsed["status"])
}

func TestIdempotencyStore_Get_NotFound(t *testing.T) {
	store, ctx := setupIdempotencyTest(t)
	_, err := store.Get(ctx, "nonexistent")
	assert.Error(t, err)
}
