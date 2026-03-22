package providers

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMockGPT(t *testing.T) {
	m := NewMockGPT("")
	assert.Equal(t, models.ProviderOpenAI, m.Name())
	assert.True(t, m.Capabilities().SupportsNativeToolCalling)
	assert.True(t, m.Capabilities().SupportsSessionContinuity)

	mods := m.Models()
	require.Len(t, mods, 1)
	assert.Equal(t, "mock-gpt-4o", mods[0].ModelID)
}

func TestNewMockOpus(t *testing.T) {
	m := NewMockOpus("")
	assert.Equal(t, models.ProviderAnthropic, m.Name())
	assert.True(t, m.Capabilities().SupportsNativeToolCalling)
	assert.False(t, m.Capabilities().SupportsSessionContinuity)

	mods := m.Models()
	require.Len(t, mods, 1)
	assert.Equal(t, "mock-claude-opus", mods[0].ModelID)
	assert.Equal(t, 200000, mods[0].MaxContextTokens)
}

func TestMockProvider_ValidateCredentials(t *testing.T) {
	m := NewMockGPT("")
	err := m.ValidateCredentials(context.Background())
	assert.NoError(t, err)
}

func TestMockProvider_Execute_DefaultResponse(t *testing.T) {
	m := NewMockGPT("")
	req := models.SessionRequest{
		ModelID: "mock-gpt-4o",
		Messages: []models.Message{
			{Role: "user", Content: "Generate a PRD"},
		},
	}

	resp, err := m.Execute(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.NotEmpty(t, resp.ProviderID)
	assert.NotEmpty(t, resp.Text)
	assert.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "submit_document", resp.ToolCalls[0].Name)
	assert.NotEmpty(t, resp.ToolCalls[0].Arguments["content"])
	assert.NotEmpty(t, resp.ToolCalls[0].Arguments["change_summary"])
	assert.Greater(t, resp.Usage.TotalTokens, 0)
	assert.NotEmpty(t, resp.RawPayload)
}

func TestMockProvider_Execute_WithOverride(t *testing.T) {
	m := NewMockGPT("")

	customResp := models.SessionResponse{
		ProviderID: "override-123",
		Text:       "Custom response",
		ToolCalls: []models.ToolCall{{
			ID:   "tc-1",
			Name: "update_fragment",
			Arguments: map[string]any{
				"fragment_id": "frag_001",
				"new_content": "Updated content",
				"rationale":   "Test override",
			},
		}},
		Usage: models.UsageMetadata{TotalTokens: 100},
	}
	m.SetOverride("mock-gpt-4o", customResp)

	resp, err := m.Execute(context.Background(), models.SessionRequest{ModelID: "mock-gpt-4o"})
	require.NoError(t, err)
	assert.Equal(t, "override-123", resp.ProviderID)
	assert.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "update_fragment", resp.ToolCalls[0].Name)
}

func TestMockProvider_Execute_CancelledContext(t *testing.T) {
	m := NewMockGPT("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := m.Execute(ctx, models.SessionRequest{ModelID: "mock-gpt-4o"})
	require.Error(t, err)

	var provErr *models.ProviderError
	require.ErrorAs(t, err, &provErr)
	assert.False(t, provErr.Retryable)
	assert.Equal(t, "cancelled", provErr.Code)
}

func TestMockProvider_CallCount(t *testing.T) {
	m := NewMockOpus("")
	ctx := context.Background()

	assert.Equal(t, 0, m.CallCount())

	_, _ = m.Execute(ctx, models.SessionRequest{ModelID: "mock-claude-opus"})
	assert.Equal(t, 1, m.CallCount())

	_, _ = m.Execute(ctx, models.SessionRequest{ModelID: "mock-claude-opus"})
	assert.Equal(t, 2, m.CallCount())
}

func TestMockProvider_ClearOverrides(t *testing.T) {
	m := NewMockGPT("")
	m.SetOverride("mock-gpt-4o", models.SessionResponse{ProviderID: "custom"})
	m.ClearOverrides()

	resp, err := m.Execute(context.Background(), models.SessionRequest{ModelID: "mock-gpt-4o"})
	require.NoError(t, err)
	assert.NotEqual(t, "custom", resp.ProviderID, "override should have been cleared")
}

func TestMockProvider_ConcurrencyHints(t *testing.T) {
	m := NewMockGPT("")
	hints := m.ConcurrencyHints()
	assert.Equal(t, 10, hints.MaxConcurrentRequests)
	assert.Greater(t, hints.RecommendedBackoff.Milliseconds(), int64(0))
}

func TestMockProvider_DeterministicToolCalls(t *testing.T) {
	m := NewMockGPT("")
	ctx := context.Background()
	req := models.SessionRequest{ModelID: "mock-gpt-4o"}

	resp, err := m.Execute(ctx, req)
	require.NoError(t, err)

	// Tool calls should be valid and normalizable.
	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	// submit_document should have required fields.
	require.Len(t, resp.ToolCalls, 1)
	tc := resp.ToolCalls[0]
	assert.Equal(t, "submit_document", tc.Name)
	_, hasContent := tc.Arguments["content"]
	assert.True(t, hasContent, "submit_document must have content arg")
	_, hasSummary := tc.Arguments["change_summary"]
	assert.True(t, hasSummary, "submit_document must have change_summary arg")
}

func TestMockProvider_FixtureLoading(t *testing.T) {
	// Create a temporary fixture file.
	dir := t.TempDir()
	scenarioDir := dir + "/test-scenario"
	mustMkdir(t, scenarioDir)

	fixture := models.SessionResponse{
		ProviderID: "fixture-response",
		Text:       "From fixture",
		ToolCalls: []models.ToolCall{{
			ID:   "fix-1",
			Name: "submit_document",
			Arguments: map[string]any{
				"content":        "Fixture content",
				"change_summary": "From fixture file",
			},
		}},
		Usage: models.UsageMetadata{TotalTokens: 500},
	}
	data, _ := json.Marshal(fixture)
	mustWriteFile(t, scenarioDir+"/gpt.json", data)

	m := NewMockProvider(MockConfig{
		Name:        models.ProviderOpenAI,
		Family:      "gpt",
		Scenario:    "test-scenario",
		FixturesDir: dir,
	})

	resp, err := m.Execute(context.Background(), models.SessionRequest{ModelID: "mock-gpt-4o"})
	require.NoError(t, err)
	assert.Equal(t, "fixture-response", resp.ProviderID)
	assert.Equal(t, "From fixture", resp.Text)
}

func TestIsMockMode(t *testing.T) {
	// Default should be false (env var not set in test).
	// We just verify the function doesn't panic.
	_ = IsMockMode()
}

// --- helpers ---

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
