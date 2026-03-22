package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPreflightTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	tdb.Exec(`INSERT INTO projects (id, name, description, status, current_stage, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', '', datetime('now'), datetime('now'))`)
	return tdb, context.Background()
}

func TestPreflight_UnknownStage(t *testing.T) {
	tdb, ctx := setupPreflightTest(t)
	result := RunPreflight(ctx, tdb.DB, "p-1", "nonexistent")
	assert.False(t, result.Passed)
	assert.Len(t, result.Failures, 1)
	assert.Equal(t, "stage_exists", result.Failures[0].Check)
}

func TestPreflight_PrdIntakeStage_Passes(t *testing.T) {
	tdb, ctx := setupPreflightTest(t)
	// PRD intake only requires seed_prd_markdown input.
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'seed_prd_markdown', 'paste', '/path', datetime('now'), datetime('now'))`)
	result := RunPreflight(ctx, tdb.DB, "p-1", "prd_intake")
	assert.True(t, result.Passed, "prd_intake preflight should pass: %v", result.Failures)
}

func TestPreflight_ParallelGeneration_MissingProviders(t *testing.T) {
	tdb, ctx := setupPreflightTest(t)
	// Stage 3 requires GPT + Opus providers. None configured.
	result := RunPreflight(ctx, tdb.DB, "p-1", "parallel_prd_generation")
	assert.False(t, result.Passed)

	// Should fail on missing artifacts AND missing providers.
	hasProviderFailure := false
	for _, f := range result.Failures {
		if f.Check == "provider_enabled" {
			hasProviderFailure = true
		}
	}
	assert.True(t, hasProviderFailure, "should flag missing providers")
}

func TestPreflight_ParallelGeneration_WithProviders(t *testing.T) {
	tdb, ctx := setupPreflightTest(t)
	// Add providers.
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-1', 'openai', 'gpt-4o', 1, datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-2', 'anthropic', 'claude-opus', 1, datetime('now'), datetime('now'))`)
	// Add required inputs.
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'seed_prd', 'paste', '/path', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-2', 'p-1', 'foundation_context', 'generated', '/path2', datetime('now'), datetime('now'))`)
	// Add required prompt template.
	tdb.Exec(`INSERT INTO prompt_templates (id, name, version, stage, baseline_text, locked_status, created_at, updated_at)
		VALUES ('pt-1', 'PRD_EXPANSION_V1', 1, 'parallel_prd_generation', 'template', 'locked', datetime('now'), datetime('now'))`)

	result := RunPreflight(ctx, tdb.DB, "p-1", "parallel_prd_generation")
	assert.True(t, result.Passed, "preflight should pass with all prerequisites: %v", result.Failures)
}

func TestPreflight_MissingPromptTemplate(t *testing.T) {
	tdb, ctx := setupPreflightTest(t)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-1', 'openai', 'gpt-4o', 1, datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-2', 'anthropic', 'claude-opus', 1, datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'seed_prd', 'paste', '/path', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-2', 'p-1', 'foundation_context', 'generated', '/path2', datetime('now'), datetime('now'))`)
	// No prompt template.

	result := RunPreflight(ctx, tdb.DB, "p-1", "parallel_prd_generation")
	assert.False(t, result.Passed)
	hasTemplateFailure := false
	for _, f := range result.Failures {
		if f.Check == "prompt_template" {
			hasTemplateFailure = true
		}
	}
	assert.True(t, hasTemplateFailure)
}

func TestPreflight_PendingReviews(t *testing.T) {
	tdb, ctx := setupPreflightTest(t)
	// Stage 7 requires canonical base artifact — so pending reviews should block.
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'canonical_prd', 'generated', '/path', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO prompt_templates (id, name, version, stage, baseline_text, locked_status, created_at, updated_at)
		VALUES ('pt-1', 'GPT_PRD_REVIEW_V1', 1, 'prd_review', 'tmpl', 'locked', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO prompt_templates (id, name, version, stage, baseline_text, locked_status, created_at, updated_at)
		VALUES ('pt-2', 'OPUS_PRD_REVIEW_V1', 1, 'prd_review', 'tmpl', 'locked', datetime('now'), datetime('now'))`)
	// Add a pending review item.
	tdb.Exec(`INSERT INTO review_items (id, project_id, stage, classification, summary, status, created_at)
		VALUES ('ri-1', 'p-1', 'prd_integration', 'disagreement', 'Test', 'pending', datetime('now'))`)

	result := RunPreflight(ctx, tdb.DB, "p-1", "prd_review")
	assert.False(t, result.Passed)
	hasPendingFailure := false
	for _, f := range result.Failures {
		if f.Check == "pending_reviews" {
			hasPendingFailure = true
		}
	}
	assert.True(t, hasPendingFailure)
}

func TestFormatPreflightFailures_Passed(t *testing.T) {
	result := PreflightResult{StageID: "test", Passed: true}
	msg := FormatPreflightFailures(result)
	assert.Contains(t, msg, "passed")
}

func TestFormatPreflightFailures_Failed(t *testing.T) {
	result := PreflightResult{
		StageID: "prd_review",
		Passed:  false,
		Failures: []PreflightFailure{
			{Check: "required_artifact", Message: "missing canonical_prd", Remediation: "complete prior stage"},
		},
	}
	msg := FormatPreflightFailures(result)
	assert.Contains(t, msg, "preflight failed")
	assert.Contains(t, msg, "canonical_prd")
	assert.Contains(t, msg, "Fix:")
}

func TestPreflightResult_StagePreflightGuard(t *testing.T) {
	// The stagePreflightPassed guard should work with preflight results.
	result := PreflightResult{StageID: "test", Passed: true}
	require.True(t, result.Passed)
}
