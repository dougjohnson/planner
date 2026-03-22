package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupManualEditTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('f-1', 'p-1', 'prd', 'Introduction', 2, datetime('now'))`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, source_stage, checksum, created_at)
		VALUES ('fv-1', 'f-1', 'Original content here.', 'prd_integration', 'ck1', datetime('now'))`)

	return tdb, ctx
}

func TestManualEdit_Success(t *testing.T) {
	tdb, ctx := setupManualEditTest(t)

	result, err := ApplyManualFragmentEdit(ctx, tdb.DB, "f-1", "Corrected content here.", "Fixed terminology")
	require.NoError(t, err)

	assert.Equal(t, "f-1", result.FragmentID)
	assert.NotEqual(t, "fv-1", result.NewVersionID) // new version created
	assert.Equal(t, "Original content here.", result.PreviousContent)
	assert.Equal(t, "Corrected content here.", result.NewContent)

	// Verify the new version is tagged as manual edit.
	var sourceStage string
	tdb.DB.QueryRowContext(ctx,
		`SELECT source_stage FROM fragment_versions WHERE id = ?`, result.NewVersionID).Scan(&sourceStage)
	assert.Equal(t, "user_manual_edit", sourceStage)
}

func TestManualEdit_NoChange(t *testing.T) {
	tdb, ctx := setupManualEditTest(t)

	// Submit same content — should not create a new version.
	result, err := ApplyManualFragmentEdit(ctx, tdb.DB, "f-1", "Original content here.", "No real change")
	require.NoError(t, err)
	assert.Equal(t, "fv-1", result.NewVersionID) // same version returned
}

func TestManualEdit_FragmentNotFound(t *testing.T) {
	tdb, ctx := setupManualEditTest(t)

	_, err := ApplyManualFragmentEdit(ctx, tdb.DB, "nonexistent", "content", "rationale")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
