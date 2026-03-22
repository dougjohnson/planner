package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockFoundations_Success(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', 'foundations', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'foundation', 'upload', '/path', datetime('now'), datetime('now'))`)

	result, err := LockFoundations(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.True(t, result.Locked)
	assert.Equal(t, "foundations", result.PreviousStage)
	assert.Equal(t, "prd_intake", result.CurrentStage)
	assert.Equal(t, 1, result.FoundationCount)

	// Verify project stage updated in DB.
	var stage string
	tdb.DB.QueryRowContext(ctx, `SELECT current_stage FROM projects WHERE id = 'p-1'`).Scan(&stage)
	assert.Equal(t, "prd_intake", stage)
}

func TestLockFoundations_NoFoundations(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', 'foundations', datetime('now'), datetime('now'))`)

	_, err := LockFoundations(ctx, tdb.DB, "p-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot lock foundations")
}

func TestLockFoundations_Idempotent(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', 'prd_intake', datetime('now'), datetime('now'))`)

	// Already past foundations — should succeed idempotently.
	result, err := LockFoundations(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.True(t, result.Locked)
	assert.Equal(t, "prd_intake", result.CurrentStage)
}

func TestLockFoundations_ProjectNotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	_, err := LockFoundations(ctx, tdb.DB, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
