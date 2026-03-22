package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCommitTest(t *testing.T) (*testutil.TestDB, context.Context, string) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	now := "2026-01-01T00:00:00Z"

	// Project.
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', '` + now + `', '` + now + `')`)

	// Fragments with versions.
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('f-1', 'p-1', 'prd', 'Introduction', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('f-2', 'p-1', 'prd', 'Architecture', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('f-3', 'p-1', 'prd', 'Conclusion', 2, '` + now + `')`)

	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('fv-1', 'f-1', 'Intro content', 'ck1', '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('fv-2', 'f-2', 'Arch content', 'ck2', '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('fv-3', 'f-3', 'Conclusion content', 'ck3', '` + now + `')`)

	// Canonical artifact.
	tdb.Exec(`INSERT INTO artifacts (id, project_id, artifact_type, version_label, created_at)
		VALUES ('art-1', 'p-1', 'prd', 'prd.v01', '` + now + `')`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
		VALUES ('art-1', 'fv-1', 0)`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
		VALUES ('art-1', 'fv-2', 1)`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
		VALUES ('art-1', 'fv-3', 2)`)

	return tdb, ctx, "art-1"
}

func TestCommitFragmentOperations_Update(t *testing.T) {
	tdb, ctx, canonicalID := setupCommitTest(t)

	ops := []FragmentOperation{
		{Type: "update", FragmentID: "f-2", NewContent: "Improved architecture content", Rationale: "Better clarity"},
	}

	result, err := CommitFragmentOperations(ctx, tdb.DB, "p-1", canonicalID, ops, "prd_commit", "run-1", "prd")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.ArtifactID)
	assert.Equal(t, "prd.v02", result.VersionLabel)
	assert.Equal(t, 1, result.UpdateCount)
	assert.Equal(t, 2, result.UnchangedCount)
	assert.Equal(t, 0, result.AddCount)
	assert.Equal(t, 0, result.RemoveCount)
	assert.False(t, result.NoChanges)

	// Verify new artifact has 3 fragments.
	var count int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifact_fragments WHERE artifact_id = ?`, result.ArtifactID).Scan(&count)
	assert.Equal(t, 3, count)

	// Verify lineage recorded.
	var relCount int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifact_relations WHERE artifact_id = ? AND related_artifact_id = ?`,
		result.ArtifactID, canonicalID).Scan(&relCount)
	assert.Equal(t, 1, relCount)
}

func TestCommitFragmentOperations_Add(t *testing.T) {
	tdb, ctx, canonicalID := setupCommitTest(t)

	ops := []FragmentOperation{
		{Type: "add", AfterFragmentID: "f-1", Heading: "Requirements", NewContent: "New requirements section", Rationale: "Missing section"},
	}

	result, err := CommitFragmentOperations(ctx, tdb.DB, "p-1", canonicalID, ops, "prd_commit", "run-2", "prd")
	require.NoError(t, err)

	assert.Equal(t, 1, result.AddCount)
	assert.Equal(t, 3, result.UnchangedCount) // original 3 unchanged

	// New artifact should have 4 fragments (3 original + 1 added).
	var count int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifact_fragments WHERE artifact_id = ?`, result.ArtifactID).Scan(&count)
	assert.Equal(t, 4, count)
}

func TestCommitFragmentOperations_Remove(t *testing.T) {
	tdb, ctx, canonicalID := setupCommitTest(t)

	ops := []FragmentOperation{
		{Type: "remove", FragmentID: "f-3", Rationale: "Section no longer relevant"},
	}

	result, err := CommitFragmentOperations(ctx, tdb.DB, "p-1", canonicalID, ops, "prd_commit", "run-3", "prd")
	require.NoError(t, err)

	assert.Equal(t, 1, result.RemoveCount)
	assert.Equal(t, 2, result.UnchangedCount)

	// New artifact should have 2 fragments.
	var count int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifact_fragments WHERE artifact_id = ?`, result.ArtifactID).Scan(&count)
	assert.Equal(t, 2, count)
}

func TestCommitFragmentOperations_MixedOperations(t *testing.T) {
	tdb, ctx, canonicalID := setupCommitTest(t)

	ops := []FragmentOperation{
		{Type: "update", FragmentID: "f-1", NewContent: "Updated intro", Rationale: "Better"},
		{Type: "add", AfterFragmentID: "f-2", Heading: "Testing", NewContent: "Testing section", Rationale: "Missing"},
		{Type: "remove", FragmentID: "f-3", Rationale: "Redundant"},
	}

	result, err := CommitFragmentOperations(ctx, tdb.DB, "p-1", canonicalID, ops, "prd_commit", "run-4", "prd")
	require.NoError(t, err)

	assert.Equal(t, 1, result.UpdateCount)
	assert.Equal(t, 1, result.AddCount)
	assert.Equal(t, 1, result.RemoveCount)
	assert.Equal(t, 1, result.UnchangedCount) // only f-2 unchanged

	// 3 original - 1 removed + 1 added = 3
	var count int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifact_fragments WHERE artifact_id = ?`, result.ArtifactID).Scan(&count)
	assert.Equal(t, 3, count)
}

func TestCommitFragmentOperations_ZeroOps(t *testing.T) {
	tdb, ctx, canonicalID := setupCommitTest(t)

	result, err := CommitFragmentOperations(ctx, tdb.DB, "p-1", canonicalID, nil, "prd_commit", "run-5", "prd")
	require.NoError(t, err)

	assert.True(t, result.NoChanges)
	assert.Equal(t, canonicalID, result.ArtifactID, "should return existing canonical ID")
}

func TestCommitFragmentOperations_VersionLabelIncrement(t *testing.T) {
	tdb, ctx, canonicalID := setupCommitTest(t)

	ops1 := []FragmentOperation{
		{Type: "update", FragmentID: "f-1", NewContent: "v2 content", Rationale: "Update"},
	}
	r1, err := CommitFragmentOperations(ctx, tdb.DB, "p-1", canonicalID, ops1, "prd_commit", "run-6", "prd")
	require.NoError(t, err)
	assert.Equal(t, "prd.v02", r1.VersionLabel)

	ops2 := []FragmentOperation{
		{Type: "update", FragmentID: "f-1", NewContent: "v3 content", Rationale: "Another update"},
	}
	r2, err := CommitFragmentOperations(ctx, tdb.DB, "p-1", r1.ArtifactID, ops2, "prd_commit", "run-7", "prd")
	require.NoError(t, err)
	assert.Equal(t, "prd.v03", r2.VersionLabel)
}
