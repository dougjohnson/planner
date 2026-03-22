package foundations

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupProvenanceTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	return tdb, ctx
}

func TestRecordFoundationInput(t *testing.T) {
	tdb, ctx := setupProvenanceTest(t)

	fi, err := RecordFoundationInput(ctx, tdb.DB, "p-1", "foundation", ProvenanceBuiltIn, "/guides/go.md", "GOLANG_BEST_PRACTICES.md")
	require.NoError(t, err)
	assert.NotEmpty(t, fi.ID)
	assert.Equal(t, ProvenanceBuiltIn, fi.SourceType)
	assert.Equal(t, "GOLANG_BEST_PRACTICES.md", fi.OriginalName)
}

func TestListFoundationInputs(t *testing.T) {
	tdb, ctx := setupProvenanceTest(t)

	RecordFoundationInput(ctx, tdb.DB, "p-1", "foundation", ProvenanceBuiltIn, "/guides/go.md", "go_guide.md")
	RecordFoundationInput(ctx, tdb.DB, "p-1", "foundation", ProvenanceUserUpload, "/uploads/custom.md", "custom_guide.md")
	RecordFoundationInput(ctx, tdb.DB, "p-1", "foundation", ProvenanceGenerated, "/gen/tech_stack.md", "tech_stack.md")

	inputs, err := ListFoundationInputs(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.Len(t, inputs, 3)

	assert.Equal(t, ProvenanceBuiltIn, inputs[0].SourceType)
	assert.Equal(t, ProvenanceUserUpload, inputs[1].SourceType)
	assert.Equal(t, ProvenanceGenerated, inputs[2].SourceType)
}

func TestGetProvenanceSummary(t *testing.T) {
	tdb, ctx := setupProvenanceTest(t)

	RecordFoundationInput(ctx, tdb.DB, "p-1", "foundation", ProvenanceBuiltIn, "/a", "a")
	RecordFoundationInput(ctx, tdb.DB, "p-1", "foundation", ProvenanceBuiltIn, "/b", "b")
	RecordFoundationInput(ctx, tdb.DB, "p-1", "foundation", ProvenanceUserUpload, "/c", "c")

	summary, err := GetProvenanceSummary(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.Equal(t, 2, summary[ProvenanceBuiltIn])
	assert.Equal(t, 1, summary[ProvenanceUserUpload])
}

func TestListFoundationInputs_Empty(t *testing.T) {
	tdb, ctx := setupProvenanceTest(t)

	inputs, err := ListFoundationInputs(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.Empty(t, inputs)
}
