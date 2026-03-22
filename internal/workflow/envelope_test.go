package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordAndLoadEnvelope(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO model_configs (id, provider, model_name, created_at, updated_at)
		VALUES ('mc-1', 'openai', 'gpt-4o', datetime('now'), datetime('now'))`)

	repo := NewRunRepository(tdb.DB)
	run, err := repo.Create(ctx, "p-1", "prd_synthesis", "mc-1")
	require.NoError(t, err)

	env := ExecutionEnvelope{
		RunID:             run.ID,
		AttemptNumber:     1,
		ProviderRequestID: "req-abc",
		ContinuityMode:    "replayed",
		TimeoutMs:         30000,
		LoopIteration:     2,
		ModelFamily:       "gpt",
		PromptTemplateID:  "GPT_PRD_SYNTHESIS_V1",
		PromptTemplateVer: 1,
		ToolDefinitions:   []string{"submit_document", "submit_change_rationale"},
		ToolCallResults: []models.NormalizedToolCallResult{
			{ToolCall: models.ToolCall{ID: "tc-1", Name: "submit_document"}, Valid: true},
		},
		SourceArtifacts: []ArtifactRef{
			{ArtifactID: "art-1", Checksum: "abc123", Position: 0},
			{ArtifactID: "art-2", Checksum: "def456", Position: 1},
		},
		SubmissionOutcome: "success",
	}

	err = RecordExecutionEnvelope(ctx, tdb.DB, "p-1", env)
	require.NoError(t, err)

	// Load back.
	loaded, err := LoadExecutionEnvelopes(ctx, tdb.DB, run.ID)
	require.NoError(t, err)
	require.Len(t, loaded, 1)

	assert.Equal(t, run.ID, loaded[0].RunID)
	assert.Equal(t, 1, loaded[0].AttemptNumber)
	assert.Equal(t, "replayed", loaded[0].ContinuityMode)
	assert.Equal(t, "gpt", loaded[0].ModelFamily)
	assert.Len(t, loaded[0].ToolCallResults, 1)
	assert.Len(t, loaded[0].SourceArtifacts, 2)
	assert.NotEmpty(t, loaded[0].InputFingerprint)
}

func TestEnvelopeFingerprint_Deterministic(t *testing.T) {
	env1 := ExecutionEnvelope{
		PromptTemplateID:  "GPT_PRD_REVIEW_V1",
		PromptTemplateVer: 1,
		ToolDefinitions:   []string{"update_fragment", "submit_review_summary"},
		ContinuityMode:    "fresh",
		SourceArtifacts: []ArtifactRef{
			{ArtifactID: "a-1", Checksum: "c1", Position: 0},
		},
	}
	env2 := env1 // same inputs

	fp1 := computeFingerprint(env1)
	fp2 := computeFingerprint(env2)
	assert.Equal(t, fp1, fp2, "same inputs should produce same fingerprint")
}

func TestEnvelopeFingerprint_DifferentInputs(t *testing.T) {
	env1 := ExecutionEnvelope{
		PromptTemplateID:  "GPT_PRD_REVIEW_V1",
		PromptTemplateVer: 1,
		ContinuityMode:    "fresh",
	}
	env2 := ExecutionEnvelope{
		PromptTemplateID:  "GPT_PRD_REVIEW_V1",
		PromptTemplateVer: 2, // different version
		ContinuityMode:    "fresh",
	}

	fp1 := computeFingerprint(env1)
	fp2 := computeFingerprint(env2)
	assert.NotEqual(t, fp1, fp2, "different inputs should produce different fingerprints")
}

func TestLoadEnvelopes_Empty(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	envs, err := LoadExecutionEnvelopes(ctx, tdb.DB, "nonexistent-run")
	require.NoError(t, err)
	assert.Empty(t, envs)
}
