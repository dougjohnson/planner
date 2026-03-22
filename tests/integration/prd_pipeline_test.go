package integration

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/providers"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/dougflynn/flywheel-planner/internal/workflow/stages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPipelineTest creates a project with foundations and seed PRD ready for the pipeline.
func setupPipelineTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Pipeline Test', '', 'active', 'parallel_prd_generation', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-gpt', 'openai', 'gpt-4o', 1, datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-opus', 'anthropic', 'claude-opus', 1, datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'foundation', 'upload', '/path/agents.md', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-2', 'p-1', 'seed_prd', 'paste', '/path/seed.md', datetime('now'), datetime('now'))`)
	// Seed prompt templates.
	tdb.Exec(`INSERT INTO prompt_templates (id, name, version, stage, baseline_text, locked_status, created_at, updated_at)
		VALUES ('pt-1', 'PRD_EXPANSION_V1', 1, 'parallel_prd_generation', 'Expand this PRD', 'locked', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO prompt_templates (id, name, version, stage, baseline_text, locked_status, created_at, updated_at)
		VALUES ('pt-2', 'GPT_PRD_SYNTHESIS_V1', 1, 'prd_synthesis', 'Synthesize', 'locked', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO prompt_templates (id, name, version, stage, baseline_text, locked_status, created_at, updated_at)
		VALUES ('pt-3', 'OPUS_PRD_INTEGRATION_V1', 1, 'prd_integration', 'Integrate', 'locked', datetime('now'), datetime('now'))`)

	return tdb, ctx
}

func TestPRDPipeline_SynthesisCreatesArtifact(t *testing.T) {
	tdb, ctx := setupPipelineTest(t)
	logger := testutil.NewTestLogger(t)

	mockGPT := providers.NewMockGPT("")

	result, err := stages.ExecuteSynthesis(
		ctx, tdb.DB, mockGPT,
		stages.PRDSynthesisConfig(), "p-1", "mc-gpt", logger,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ArtifactID)
	assert.Contains(t, result.VersionLabel, "prd")

	// Verify artifact in DB.
	var artCount int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifacts WHERE project_id = 'p-1' AND artifact_type = 'prd'`).Scan(&artCount)
	assert.GreaterOrEqual(t, artCount, 1)
}

func TestPRDPipeline_IntegrationCreatesReviewItems(t *testing.T) {
	tdb, ctx := setupPipelineTest(t)
	logger := testutil.NewTestLogger(t)

	// Add a fragment for disagreement references.
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('frag-intro', 'p-1', 'prd', 'Introduction', 2, datetime('now'))`)

	mockOpus := providers.NewMockOpus("")
	mockOpus.SetOverride(mockOpus.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "integ-1",
		ToolCalls: []models.ToolCall{
			{ID: "tc-1", Name: "submit_document", Arguments: map[string]any{
				"content": "## Introduction\n\nIntegrated content.\n", "change_summary": "Integrated",
			}},
			{ID: "tc-2", Name: "report_disagreement", Arguments: map[string]any{
				"fragment_id": "frag-intro", "severity": "moderate",
				"summary": "Missing context", "rationale": "Needs background",
				"suggested_change": "Add background section",
			}},
		},
		Usage: models.UsageMetadata{TotalTokens: 5000},
	})

	result, err := stages.ExecuteIntegration(
		ctx, tdb.DB, mockOpus,
		stages.PRDIntegrationConfig(), "p-1", "mc-opus", logger,
	)
	require.NoError(t, err)
	assert.True(t, result.HasDisagreements)
	assert.Equal(t, 1, result.DisagreementCount)

	// Verify review_item exists.
	var reviewCount int
	tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM review_items WHERE project_id = 'p-1' AND classification = 'disagreement'`).Scan(&reviewCount)
	assert.Equal(t, 1, reviewCount)
}

func TestPRDPipeline_ReviewProducesOperations(t *testing.T) {
	tdb, ctx := setupPipelineTest(t)
	logger := testutil.NewTestLogger(t)

	mockGPT := providers.NewMockGPT("")
	mockGPT.SetOverride(mockGPT.Models()[0].ModelID, models.SessionResponse{
		ProviderID: "review-1",
		ToolCalls: []models.ToolCall{
			{ID: "tc-1", Name: "update_fragment", Arguments: map[string]any{
				"fragment_id": "frag-1", "new_content": "Better content", "rationale": "Improved clarity",
			}},
			{ID: "tc-2", Name: "submit_review_summary", Arguments: map[string]any{
				"summary": "One section improved",
			}},
		},
		Usage: models.UsageMetadata{TotalTokens: 4000},
	})

	result, err := stages.ExecuteReview(
		ctx, tdb.DB, mockGPT,
		stages.PRDReviewConfig("gpt"), "p-1", "mc-gpt", logger,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, result.OperationCount)
	assert.Equal(t, "update", result.Operations[0].Type)
	assert.False(t, result.NoChanges)
}

func TestPRDPipeline_QuorumGuard(t *testing.T) {
	tdb, ctx := setupPipelineTest(t)

	// No completed runs → quorum not satisfied.
	guardResult, err := workflow.EvaluateGuard(ctx, tdb.DB, "parallelQuorumSatisfied", "p-1")
	require.NoError(t, err)
	assert.False(t, guardResult.Passed)

	// Add GPT run only → still no quorum.
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-gpt', 'p-1', 'parallel_prd_generation', 'mc-gpt', 'completed', datetime('now'))`)

	guardResult, _ = workflow.EvaluateGuard(ctx, tdb.DB, "parallelQuorumSatisfied", "p-1")
	assert.False(t, guardResult.Passed)
	assert.Contains(t, guardResult.Reason, "Opus")

	// Add Opus run → quorum satisfied.
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, created_at)
		VALUES ('r-opus', 'p-1', 'parallel_prd_generation', 'mc-opus', 'completed', datetime('now'))`)

	guardResult, _ = workflow.EvaluateGuard(ctx, tdb.DB, "parallelQuorumSatisfied", "p-1")
	assert.True(t, guardResult.Passed)
}

func TestPRDPipeline_FoundationsLockTransition(t *testing.T) {
	tdb, ctx := setupPipelineTest(t)

	// Set project back to foundations stage.
	tdb.Exec(`UPDATE projects SET current_stage = 'foundations' WHERE id = 'p-1'`)

	result, err := workflow.LockFoundations(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.True(t, result.Locked)
	assert.Equal(t, "prd_intake", result.CurrentStage)
}

func TestPRDPipeline_CommitWithConvergence(t *testing.T) {
	tdb, ctx := setupPipelineTest(t)

	// Setup canonical artifact with fragments.
	now := "2026-01-01T00:00:00Z"
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('f-1', 'p-1', 'prd', 'Intro', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('fv-1', 'f-1', 'Content', 'ck', '` + now + `')`)
	tdb.Exec(`INSERT INTO artifacts (id, project_id, artifact_type, version_label, created_at)
		VALUES ('art-canon', 'p-1', 'prd', 'prd.v01', '` + now + `')`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
		VALUES ('art-canon', 'fv-1', 0)`)

	// Commit with zero operations → convergence.
	commitResult, err := workflow.CommitFragmentOperations(
		ctx, tdb.DB, "p-1", "art-canon", nil, "prd_commit", "run-1", "prd",
	)
	require.NoError(t, err)
	assert.True(t, commitResult.NoChanges)

	// Check convergence detection.
	conv := workflow.CheckConvergence(commitResult, 3, 4)
	assert.Equal(t, workflow.ConvergenceDetected, conv.Status)
}
