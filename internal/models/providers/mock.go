// Package providers contains provider adapter implementations.
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

// MockProvider implements the Provider interface with deterministic,
// fixture-backed responses for testing without live API keys.
type MockProvider struct {
	name         models.ProviderName
	family       string // "gpt" or "opus"
	scenario     string // e.g., "happy-path-prd"
	fixturesDir  string
	capabilities models.Capabilities

	mu       sync.Mutex
	callLog  []mockCall
	overrides map[string]models.SessionResponse // keyed by stage
}

type mockCall struct {
	RequestedAt time.Time
	Stage       string
	ModelID     string
}

// MockConfig configures a mock provider instance.
type MockConfig struct {
	Name        models.ProviderName
	Family      string // "gpt" or "opus"
	Scenario    string
	FixturesDir string
}

// NewMockProvider creates a mock provider that loads responses from fixtures.
func NewMockProvider(cfg MockConfig) *MockProvider {
	scenario := cfg.Scenario
	if scenario == "" {
		scenario = os.Getenv("FLYWHEEL_MOCK_SCENARIO")
	}
	if scenario == "" {
		scenario = "happy-path-prd"
	}

	fixturesDir := cfg.FixturesDir
	if fixturesDir == "" {
		fixturesDir = "tests/fixtures/mock-responses"
	}

	maxCtx := 128000
	if cfg.Family == "opus" {
		maxCtx = 200000
	}

	return &MockProvider{
		name:     cfg.Name,
		family:   cfg.Family,
		scenario: scenario,
		fixturesDir: fixturesDir,
		capabilities: models.Capabilities{
			SupportsFreshSessions:          true,
			SupportsSessionContinuity:      cfg.Family == "gpt",
			SupportsFileAttachments:        true,
			SupportsNativeToolCalling:      true,
			SupportsStructuredOutputHints:  true,
			SupportsReasoningModeSelection: true,
			MaxContextTokens:               maxCtx,
		},
		overrides: make(map[string]models.SessionResponse),
	}
}

// Name returns the provider name.
func (m *MockProvider) Name() models.ProviderName {
	return m.name
}

// Models returns mock model metadata.
func (m *MockProvider) Models() []models.ModelInfo {
	modelID := "mock-gpt-4o"
	displayName := "Mock GPT-4o"
	if m.family == "opus" {
		modelID = "mock-claude-opus"
		displayName = "Mock Claude Opus"
	}
	return []models.ModelInfo{{
		Provider:         m.name,
		ModelID:          modelID,
		DisplayName:      displayName,
		MaxContextTokens: m.capabilities.MaxContextTokens,
		SupportsReasoning: true,
	}}
}

// Capabilities returns the mock provider's capability flags.
func (m *MockProvider) Capabilities() models.Capabilities {
	return m.capabilities
}

// ValidateCredentials always succeeds for mock providers.
func (m *MockProvider) ValidateCredentials(_ context.Context) error {
	return nil
}

// Execute returns a deterministic response based on the fixture scenario.
func (m *MockProvider) Execute(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
	m.mu.Lock()
	m.callLog = append(m.callLog, mockCall{
		RequestedAt: time.Now(),
		ModelID:     req.ModelID,
	})

	// Check for per-stage overrides (used by individual tests).
	if override, ok := m.overrides[req.ModelID]; ok {
		m.mu.Unlock()
		return &override, nil
	}
	m.mu.Unlock()

	// Check context cancellation.
	if err := ctx.Err(); err != nil {
		return nil, models.NormalizeCancellationError(m.name, err)
	}

	// Try to load fixture response.
	resp, err := m.loadFixtureResponse(req)
	if err != nil {
		// If no fixture found, return a default successful response.
		return m.defaultResponse(req), nil
	}

	return resp, nil
}

// ConcurrencyHints returns mock concurrency limits.
func (m *MockProvider) ConcurrencyHints() models.ConcurrencyHints {
	return models.ConcurrencyHints{
		MaxConcurrentRequests: 10,
		RecommendedBackoff:    100 * time.Millisecond,
	}
}

// --- Test Helpers ---

// SetOverride sets a fixed response for a specific model ID.
func (m *MockProvider) SetOverride(modelID string, resp models.SessionResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overrides[modelID] = resp
}

// ClearOverrides removes all test overrides.
func (m *MockProvider) ClearOverrides() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overrides = make(map[string]models.SessionResponse)
}

// CallCount returns the number of Execute calls made.
func (m *MockProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.callLog)
}

// CallLog returns a copy of all recorded calls.
func (m *MockProvider) CallLog() []mockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	log := make([]mockCall, len(m.callLog))
	copy(log, m.callLog)
	return log
}

// --- Internal ---

func (m *MockProvider) loadFixtureResponse(req models.SessionRequest) (*models.SessionResponse, error) {
	// Try scenario-specific fixture first.
	patterns := []string{
		filepath.Join(m.fixturesDir, m.scenario, fmt.Sprintf("%s-%s.json", req.ModelID, m.family)),
		filepath.Join(m.fixturesDir, m.scenario, fmt.Sprintf("%s.json", m.family)),
	}

	for _, path := range patterns {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var resp models.SessionResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("parsing fixture %s: %w", path, err)
		}
		return &resp, nil
	}

	return nil, fmt.Errorf("no fixture found for scenario=%s family=%s model=%s", m.scenario, m.family, req.ModelID)
}

func (m *MockProvider) defaultResponse(req models.SessionRequest) *models.SessionResponse {
	// Generate a default submit_document tool call.
	toolCall := models.ToolCall{
		ID:   fmt.Sprintf("mock_call_%d", time.Now().UnixNano()),
		Name: "submit_document",
		Arguments: map[string]any{
			"content":        "## Mock Section\n\nThis is mock-generated content from the " + m.family + " provider.\n",
			"change_summary": "Mock " + m.family + " response",
		},
	}

	rawPayload, _ := json.Marshal(map[string]any{
		"mock": true, "family": m.family, "model": req.ModelID,
	})

	return &models.SessionResponse{
		ProviderID: fmt.Sprintf("mock-%s-%d", m.family, time.Now().UnixNano()),
		Text:       "Mock response from " + m.family + " provider",
		ToolCalls:  []models.ToolCall{toolCall},
		RawPayload: rawPayload,
		Usage: models.UsageMetadata{
			PromptTokens:     1500,
			CompletionTokens: 800,
			TotalTokens:      2300,
		},
	}
}

// --- Factory ---

// NewMockGPT creates a mock GPT-family provider.
func NewMockGPT(fixturesDir string) *MockProvider {
	return NewMockProvider(MockConfig{
		Name:        models.ProviderOpenAI,
		Family:      "gpt",
		FixturesDir: fixturesDir,
	})
}

// NewMockOpus creates a mock Opus-family provider.
func NewMockOpus(fixturesDir string) *MockProvider {
	return NewMockProvider(MockConfig{
		Name:        models.ProviderAnthropic,
		Family:      "opus",
		FixturesDir: fixturesDir,
	})
}

// IsMockMode returns true if mock providers should be used.
func IsMockMode() bool {
	return os.Getenv("FLYWHEEL_MOCK_PROVIDERS") == "true"
}
