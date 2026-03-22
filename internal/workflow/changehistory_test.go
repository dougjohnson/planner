package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHistoryTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-gpt', 'openai', 'gpt-4o', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-opus', 'anthropic', 'claude-opus', datetime('now'), datetime('now'))`)

	return tdb, ctx
}

func TestBuildChangeHistory_NoRuns(t *testing.T) {
	tdb, ctx := setupHistoryTest(t)

	history, err := BuildChangeHistory(ctx, tdb.DB, "p-1", "prd")
	require.NoError(t, err)
	assert.Equal(t, 0, history.TotalIters)
	assert.Empty(t, history.Entries)
}

func TestBuildChangeHistory_WithReviewRuns(t *testing.T) {
	tdb, ctx := setupHistoryTest(t)

	// Create completed review runs.
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-1', 'p-1', 'prd_review', 'mc-gpt', 'completed', '2026-01-01T01:00:00Z')`)
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-2', 'p-1', 'prd_review', 'mc-opus', 'completed', '2026-01-01T02:00:00Z')`)

	history, err := BuildChangeHistory(ctx, tdb.DB, "p-1", "prd")
	require.NoError(t, err)
	assert.Equal(t, 2, history.TotalIters)
	require.Len(t, history.Entries, 2)
	assert.Equal(t, "gpt", history.Entries[0].ModelFamily)
	assert.Equal(t, "opus", history.Entries[1].ModelFamily)
}

func TestBuildChangeHistory_PlanDocType(t *testing.T) {
	tdb, ctx := setupHistoryTest(t)

	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-1', 'p-1', 'plan_review', 'mc-gpt', 'completed', '2026-01-01T01:00:00Z')`)

	history, err := BuildChangeHistory(ctx, tdb.DB, "p-1", "plan")
	require.NoError(t, err)
	assert.Equal(t, "plan", history.DocumentType)
	assert.Equal(t, 1, history.TotalIters)
}

func TestRenderChangeHistoryMarkdown_Empty(t *testing.T) {
	result := RenderChangeHistoryMarkdown(nil)
	assert.Equal(t, "", result)

	result = RenderChangeHistoryMarkdown(&ChangeHistory{})
	assert.Equal(t, "", result)
}

func TestRenderChangeHistoryMarkdown_WithEntries(t *testing.T) {
	history := &ChangeHistory{
		DocumentType: "prd",
		TotalIters:   2,
		Entries: []ChangeHistoryEntry{
			{
				Iteration:    1,
				ModelFamily:  "gpt",
				UpdatedFrags: []string{"frag_001", "frag_002"},
				AddedFrags:   []string{"frag_010"},
			},
			{
				Iteration:   2,
				ModelFamily: "opus",
				Guidance:    "Focus more on testing strategy",
			},
		},
	}

	md := RenderChangeHistoryMarkdown(history)
	assert.Contains(t, md, "Prior Review History")
	assert.Contains(t, md, "2 review iteration")
	assert.Contains(t, md, "Iteration 1 (gpt)")
	assert.Contains(t, md, "Updated 2 fragment")
	assert.Contains(t, md, "Added 1 fragment")
	assert.Contains(t, md, "Iteration 2 (opus)")
	assert.Contains(t, md, "Focus more on testing strategy")
	assert.Contains(t, md, "re-proposing changes")
}

func TestRenderChangeHistoryMarkdown_Convergence(t *testing.T) {
	history := &ChangeHistory{
		DocumentType: "prd",
		TotalIters:   1,
		Entries: []ChangeHistoryEntry{
			{Iteration: 1, ModelFamily: "gpt"},
		},
	}

	md := RenderChangeHistoryMarkdown(history)
	assert.Contains(t, md, "convergence candidate")
}
