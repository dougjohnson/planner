package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/registry"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// orchMock is a test provider for orchestrator tests.
type orchMock struct {
	name   models.ProviderName
	execFn func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error)
}

func (m *orchMock) Name() models.ProviderName                         { return m.name }
func (m *orchMock) Models() []models.ModelInfo                        { return nil }
func (m *orchMock) Capabilities() models.Capabilities                 { return models.Capabilities{} }
func (m *orchMock) ValidateCredentials(_ context.Context) error        { return nil }
func (m *orchMock) ConcurrencyHints() models.ConcurrencyHints         { return models.ConcurrencyHints{} }
func (m *orchMock) Execute(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
	if m.execFn != nil {
		return m.execFn(ctx, req)
	}
	return &models.SessionResponse{
		Text:      "Generated content",
		ToolCalls: []models.ToolCall{{Name: "submit_document", Arguments: map[string]any{"content": "doc"}}},
		Usage:     models.UsageMetadata{TotalTokens: 100},
	}, nil
}

func newOrchSuccess(name models.ProviderName) *orchMock {
	return &orchMock{name: name}
}

func newOrchFail(name models.ProviderName, errMsg string) *orchMock {
	return &orchMock{
		name: name,
		execFn: func(_ context.Context, _ models.SessionRequest) (*models.SessionResponse, error) {
			return nil, fmt.Errorf("%s", errMsg)
		},
	}
}

func newOrchSlow(name models.ProviderName, delay time.Duration) *orchMock {
	return &orchMock{
		name: name,
		execFn: func(ctx context.Context, _ models.SessionRequest) (*models.SessionResponse, error) {
			select {
			case <-time.After(delay):
				return &models.SessionResponse{Usage: models.UsageMetadata{TotalTokens: 50}}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
}

func setupOrch(t *testing.T, quorum QuorumPolicy, providers ...models.Provider) *ParallelOrchestrator {
	t.Helper()
	logger := testLogger()
	reg := registry.New(logger)
	for _, p := range providers {
		reg.Register(p)
	}
	hub := sse.NewHub(logger)
	pool := NewPool(reg, hub, logger, 4)
	orch := NewParallelOrchestrator(pool, quorum, logger)
	orch.SetProviderTimeout(5 * time.Second)
	orch.SetOverallTimeout(10 * time.Second)
	return orch
}

func orchRequest(providers ...models.ProviderName) ParallelGenerationRequest {
	sessions := make(map[models.ProviderName]models.SessionRequest)
	for _, p := range providers {
		sessions[p] = models.SessionRequest{
			ModelID:  "test-model",
			Messages: []models.Message{{Role: "user", Content: "Generate"}},
		}
	}
	return ParallelGenerationRequest{
		ProjectID:          "proj-1",
		DocumentStream:     "prd",
		Providers:          providers,
		SessionsByProvider: sessions,
	}
}

func TestOrchestrator_BothProvidersSucceed(t *testing.T) {
	orch := setupOrch(t, DefaultQuorumPolicy(),
		newOrchSuccess("openai"),
		newOrchSuccess("anthropic"),
	)

	result, err := orch.Execute(context.Background(), orchRequest("openai", "anthropic"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !result.QuorumMet {
		t.Error("expected quorum met")
	}
	if len(result.Submissions) != 2 {
		t.Errorf("expected 2 submissions, got %d", len(result.Submissions))
	}
	if len(result.Failures) != 0 {
		t.Errorf("expected 0 failures, got %d", len(result.Failures))
	}
}

func TestOrchestrator_OneFailsQuorumStillMet(t *testing.T) {
	orch := setupOrch(t, SingleProviderQuorum(),
		newOrchSuccess("openai"),
		newOrchFail("anthropic", "timeout"),
	)

	result, err := orch.Execute(context.Background(), orchRequest("openai", "anthropic"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !result.QuorumMet {
		t.Error("expected quorum met with single-provider policy")
	}
	if len(result.Submissions) != 1 {
		t.Errorf("expected 1 submission, got %d", len(result.Submissions))
	}
	if len(result.Failures) != 1 {
		t.Errorf("expected 1 failure, got %d", len(result.Failures))
	}
}

func TestOrchestrator_AllFail_QuorumNotMet(t *testing.T) {
	orch := setupOrch(t, SingleProviderQuorum(),
		newOrchFail("openai", "api error"),
		newOrchFail("anthropic", "rate limited"),
	)

	result, err := orch.Execute(context.Background(), orchRequest("openai", "anthropic"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.QuorumMet {
		t.Error("expected quorum NOT met")
	}
	if len(result.Failures) != 2 {
		t.Errorf("expected 2 failures, got %d", len(result.Failures))
	}
}

func TestOrchestrator_DefaultQuorum_MissingOpus(t *testing.T) {
	orch := setupOrch(t, DefaultQuorumPolicy(),
		newOrchSuccess("openai"),
		newOrchFail("anthropic", "down"),
	)

	result, err := orch.Execute(context.Background(), orchRequest("openai", "anthropic"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.QuorumMet {
		t.Error("expected quorum NOT met — Opus required but failed")
	}
}

func TestOrchestrator_NoProviders(t *testing.T) {
	orch := setupOrch(t, SingleProviderQuorum())

	_, err := orch.Execute(context.Background(), ParallelGenerationRequest{ProjectID: "proj-1"})
	if err == nil {
		t.Error("expected error for no providers")
	}
}

func TestOrchestrator_ProviderTimeout(t *testing.T) {
	orch := setupOrch(t, SingleProviderQuorum(),
		newOrchSlow("openai", 10*time.Second),
		newOrchSuccess("anthropic"),
	)
	orch.SetProviderTimeout(100 * time.Millisecond)

	result, err := orch.Execute(context.Background(), orchRequest("openai", "anthropic"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !result.QuorumMet {
		t.Error("expected quorum met")
	}
	if len(result.Submissions) != 1 {
		t.Errorf("expected 1 submission (anthropic), got %d", len(result.Submissions))
	}
}
