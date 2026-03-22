package workflow

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validResult(name string) models.NormalizedToolCallResult {
	return models.NormalizedToolCallResult{
		ToolCall: models.ToolCall{ID: "tc-1", Name: name, Arguments: map[string]any{"content": "test"}},
		Valid:    true,
	}
}

func invalidResult(name string, errs ...string) models.NormalizedToolCallResult {
	return models.NormalizedToolCallResult{
		ToolCall:         models.ToolCall{ID: "tc-2", Name: name, Arguments: map[string]any{}},
		Valid:            false,
		ValidationErrors: errs,
	}
}

func TestEvaluateRecovery_AllValid(t *testing.T) {
	results := []models.NormalizedToolCallResult{
		validResult("submit_document"),
	}
	decision := EvaluateRecovery(results, DefaultRecoveryConfig(), 1, nil)
	assert.Equal(t, RecoveryProceed, decision.Action)
	assert.Len(t, decision.ValidResults, 1)
	assert.Empty(t, decision.InvalidResults)
}

func TestEvaluateRecovery_InvalidFirstAttempt_Retry(t *testing.T) {
	results := []models.NormalizedToolCallResult{
		validResult("submit_document"),
		invalidResult("update_fragment", "missing required argument: fragment_id"),
	}
	decision := EvaluateRecovery(results, DefaultRecoveryConfig(), 1, nil)
	assert.Equal(t, RecoveryRetry, decision.Action)
	assert.NotEmpty(t, decision.RetryMessage)
	assert.Contains(t, decision.RetryMessage, "fragment_id")
}

func TestEvaluateRecovery_InvalidExhausted_Partial(t *testing.T) {
	config := RecoveryConfig{MaxRetries: 2, AllowPartialAccept: true}
	results := []models.NormalizedToolCallResult{
		validResult("submit_document"),
		invalidResult("update_fragment", "bad arg"),
	}
	// Attempt 2 = at max retries.
	decision := EvaluateRecovery(results, config, 2, nil)
	assert.Equal(t, RecoveryPartial, decision.Action)
	assert.Len(t, decision.ValidResults, 1)
	assert.Len(t, decision.InvalidResults, 1)
}

func TestEvaluateRecovery_InvalidExhausted_NoPartial_UserRequired(t *testing.T) {
	config := RecoveryConfig{MaxRetries: 1, AllowPartialAccept: false}
	results := []models.NormalizedToolCallResult{
		invalidResult("submit_document", "missing content"),
	}
	decision := EvaluateRecovery(results, config, 1, nil)
	assert.Equal(t, RecoveryUserRequired, decision.Action)
}

func TestEvaluateRecovery_MissingRequiredTool(t *testing.T) {
	config := RecoveryConfig{
		MaxRetries:       2,
		RequiredToolNames: []string{"submit_document"},
	}
	// Valid call but not the required one.
	results := []models.NormalizedToolCallResult{
		validResult("submit_change_rationale"),
	}
	decision := EvaluateRecovery(results, config, 1, nil)
	assert.Equal(t, RecoveryRetry, decision.Action)
	assert.Contains(t, decision.RetryMessage, "submit_document")
}

func TestEvaluateRecovery_AllAttemptsPreserved(t *testing.T) {
	attempt1 := []models.NormalizedToolCallResult{invalidResult("bad", "err")}
	attempt2 := []models.NormalizedToolCallResult{validResult("submit_document")}

	decision := EvaluateRecovery(attempt2, DefaultRecoveryConfig(), 2, [][]models.NormalizedToolCallResult{attempt1})
	assert.Equal(t, RecoveryProceed, decision.Action)
	require.Len(t, decision.AllAttempts, 2)
}

func TestValidateFragmentIDs_ValidIDs(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, created_at)
		VALUES ('frag_001', 'p-1', 'prd', 'Intro', datetime('now'))`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, created_at)
		VALUES ('frag_002', 'p-1', 'prd', 'Arch', datetime('now'))`)

	results := []models.NormalizedToolCallResult{{
		ToolCall: models.ToolCall{
			Name:      "update_fragment",
			Arguments: map[string]any{"fragment_id": "frag_001", "new_content": "Updated"},
		},
		Valid: true,
	}}

	enriched := ValidateFragmentIDs(ctx, tdb.DB, "p-1", results)
	assert.True(t, enriched[0].Valid)
}

func TestValidateFragmentIDs_InvalidID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, created_at)
		VALUES ('frag_001', 'p-1', 'prd', 'Intro', datetime('now'))`)

	results := []models.NormalizedToolCallResult{{
		ToolCall: models.ToolCall{
			Name:      "update_fragment",
			Arguments: map[string]any{"fragment_id": "frag_999", "new_content": "Bad"},
		},
		Valid: true,
	}}

	enriched := ValidateFragmentIDs(ctx, tdb.DB, "p-1", results)
	assert.False(t, enriched[0].Valid)
	assert.Contains(t, enriched[0].ValidationErrors[0], "frag_999")
	assert.Contains(t, enriched[0].ValidationErrors[0], "does not exist")
	assert.Contains(t, enriched[0].ValidationErrors[0], "frag_001") // lists available IDs
}
