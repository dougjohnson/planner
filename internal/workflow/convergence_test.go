package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckConvergence_ZeroOps(t *testing.T) {
	result := CheckConvergence(&CommitResult{NoChanges: true}, 3, 4)
	assert.Equal(t, ConvergenceDetected, result.Status)
	assert.Equal(t, 3, result.IterationNumber)
	assert.Equal(t, 1, result.RemainingLoops)
	assert.Equal(t, 0, result.OperationCount)
	assert.Contains(t, result.Message, "no changes")
}

func TestCheckConvergence_WithOps(t *testing.T) {
	result := CheckConvergence(&CommitResult{UpdateCount: 2, AddCount: 1}, 2, 4)
	assert.Equal(t, ConvergenceNone, result.Status)
	assert.Equal(t, 3, result.OperationCount)
	assert.Equal(t, 2, result.RemainingLoops)
	assert.Contains(t, result.Message, "3 fragment operations")
}

func TestCheckConvergence_LastIteration(t *testing.T) {
	result := CheckConvergence(&CommitResult{NoChanges: true}, 4, 4)
	assert.Equal(t, ConvergenceDetected, result.Status)
	assert.Equal(t, 0, result.RemainingLoops)
}

func TestAcceptConvergence(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)

	cr := ConvergenceResult{
		Status:          ConvergenceDetected,
		IterationNumber: 3,
		RemainingLoops:  1,
	}

	err := AcceptConvergence(ctx, tdb.DB, "p-1", cr)
	require.NoError(t, err)

	// Verify event recorded.
	var count int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_events WHERE project_id = 'p-1' AND event_type = 'convergence_accepted'`).Scan(&count)
	assert.Equal(t, 1, count)
}

func TestDeclineConvergence_WithGuidance(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)

	cr := ConvergenceResult{
		Status:          ConvergenceDetected,
		IterationNumber: 2,
		RemainingLoops:  2,
	}

	err := DeclineConvergence(ctx, tdb.DB, "p-1", "prd_review", cr, "Focus on the testing section, it needs more detail")
	require.NoError(t, err)

	// Verify event recorded.
	var eventCount int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_events WHERE project_id = 'p-1' AND event_type = 'convergence_declined'`).Scan(&eventCount)
	assert.Equal(t, 1, eventCount)

	// Verify guidance injected.
	var guidanceCount int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM guidance_injections WHERE project_id = 'p-1'`).Scan(&guidanceCount)
	assert.Equal(t, 1, guidanceCount)
}

func TestDeclineConvergence_WithoutGuidance(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)

	cr := ConvergenceResult{IterationNumber: 2, RemainingLoops: 2}
	err := DeclineConvergence(ctx, tdb.DB, "p-1", "prd_review", cr, "")
	require.NoError(t, err)

	// No guidance should be injected.
	var guidanceCount int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM guidance_injections WHERE project_id = 'p-1'`).Scan(&guidanceCount)
	assert.Equal(t, 0, guidanceCount)
}

func TestGetConvergenceHistory(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)

	// Record some events.
	AcceptConvergence(ctx, tdb.DB, "p-1", ConvergenceResult{IterationNumber: 3})
	DeclineConvergence(ctx, tdb.DB, "p-1", "prd_review", ConvergenceResult{IterationNumber: 2}, "")

	history, err := GetConvergenceHistory(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.Len(t, history, 2)
}
