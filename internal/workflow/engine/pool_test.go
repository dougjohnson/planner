package engine

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/registry"
)

// testLogger is defined in orchestrator_test.go

// mockProvider implements models.Provider for testing.
type mockProvider struct {
	name        models.ProviderName
	execFunc    func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error)
}

func newMockProvider(name models.ProviderName) *mockProvider {
	return &mockProvider{
		name: name,
		execFunc: func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
			return &models.SessionResponse{
				Text:  "mock response",
				Usage: models.UsageMetadata{TotalTokens: 100},
			}, nil
		},
	}
}

func (m *mockProvider) Name() models.ProviderName                          { return m.name }
func (m *mockProvider) Models() []models.ModelInfo                         { return nil }
func (m *mockProvider) Capabilities() models.Capabilities                  { return models.Capabilities{} }
func (m *mockProvider) ValidateCredentials(ctx context.Context) error       { return nil }
func (m *mockProvider) ConcurrencyHints() models.ConcurrencyHints          { return models.ConcurrencyHints{} }
func (m *mockProvider) Execute(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
	return m.execFunc(ctx, req)
}

func setupPool(t *testing.T, maxConcurrency int) (*Pool, *registry.Registry) {
	t.Helper()
	logger := testLogger()
	reg := registry.New(logger)
	reg.Register(newMockProvider(models.ProviderOpenAI))

	hub := sse.NewHub(logger)
	pool := NewPool(reg, hub, logger, maxConcurrency)
	return pool, reg
}

func TestSubmit_Success(t *testing.T) {
	pool, _ := setupPool(t, 4)

	resp, err := pool.Submit(context.Background(), RunRequest{
		ProjectID:     "proj-1",
		WorkflowRunID: "run-1",
		Provider:      models.ProviderOpenAI,
		Session:       models.SessionRequest{ModelID: "gpt-4o"},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("run error: %v", resp.Error)
	}
	if resp.Response == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Response.Text != "mock response" {
		t.Errorf("expected 'mock response', got %q", resp.Response.Text)
	}
}

func TestSubmit_ProviderError(t *testing.T) {
	logger := testLogger()
	reg := registry.New(logger)
	mock := newMockProvider(models.ProviderOpenAI)
	mock.execFunc = func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
		return nil, fmt.Errorf("provider error")
	}
	reg.Register(mock)

	pool := NewPool(reg, sse.NewHub(logger), logger, 4)

	resp, err := pool.Submit(context.Background(), RunRequest{
		ProjectID: "proj-1", WorkflowRunID: "run-1", Provider: models.ProviderOpenAI,
	})
	if err != nil {
		t.Fatalf("Submit should not return pool error: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected run error from provider")
	}
}

func TestSubmit_ContextCancelled(t *testing.T) {
	logger := testLogger()
	reg := registry.New(logger)
	reg.Register(newMockProvider(models.ProviderOpenAI))

	// Pool with 1 slot.
	pool := NewPool(reg, sse.NewHub(logger), logger, 1)

	// Fill the slot with a slow request.
	slowCtx, slowCancel := context.WithCancel(context.Background())
	defer slowCancel()

	slow := newMockProvider(models.ProviderOpenAI)
	slow.execFunc = func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	reg.Register(slow) // Replace with slow provider.

	go pool.Submit(slowCtx, RunRequest{
		ProjectID: "proj-1", WorkflowRunID: "run-slow", Provider: models.ProviderOpenAI,
	})
	time.Sleep(10 * time.Millisecond) // Let the slow request acquire the slot.

	// Try to submit with a cancelled context.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := pool.Submit(cancelCtx, RunRequest{
		ProjectID: "proj-1", WorkflowRunID: "run-2", Provider: models.ProviderOpenAI,
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	slowCancel() // Clean up slow goroutine.
}

func TestSubmitAll(t *testing.T) {
	pool, _ := setupPool(t, 4)

	requests := []RunRequest{
		{ProjectID: "proj-1", WorkflowRunID: "run-1", Provider: models.ProviderOpenAI},
		{ProjectID: "proj-1", WorkflowRunID: "run-2", Provider: models.ProviderOpenAI},
		{ProjectID: "proj-1", WorkflowRunID: "run-3", Provider: models.ProviderOpenAI},
	}

	results, err := pool.SubmitAll(context.Background(), requests)
	if err != nil {
		t.Fatalf("SubmitAll: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Error != nil {
			t.Errorf("result %d error: %v", i, r.Error)
		}
		if r.Response == nil {
			t.Errorf("result %d has nil response", i)
		}
	}
}

func TestSubmitAll_Empty(t *testing.T) {
	pool, _ := setupPool(t, 4)

	results, err := pool.SubmitAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("SubmitAll empty: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}

func TestBoundedConcurrency(t *testing.T) {
	logger := testLogger()
	reg := registry.New(logger)

	var active atomic.Int32
	var maxActive atomic.Int32

	slow := newMockProvider(models.ProviderOpenAI)
	slow.execFunc = func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
		cur := active.Add(1)
		// Track the max active workers.
		for {
			old := maxActive.Load()
			if cur <= old || maxActive.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond) // Simulate work.
		active.Add(-1)
		return &models.SessionResponse{Text: "ok"}, nil
	}
	reg.Register(slow)

	const maxConcurrency = 2
	pool := NewPool(reg, sse.NewHub(logger), logger, maxConcurrency)

	// Submit more requests than the pool size.
	requests := make([]RunRequest, 6)
	for i := range requests {
		requests[i] = RunRequest{
			ProjectID:     "proj-1",
			WorkflowRunID: fmt.Sprintf("run-%d", i),
			Provider:      models.ProviderOpenAI,
		}
	}

	pool.SubmitAll(context.Background(), requests)

	observed := maxActive.Load()
	if observed > int32(maxConcurrency) {
		t.Errorf("max active workers (%d) exceeded pool size (%d)", observed, maxConcurrency)
	}
}

func TestMaxConcurrency(t *testing.T) {
	pool, _ := setupPool(t, 8)
	if pool.MaxConcurrency() != 8 {
		t.Errorf("expected max concurrency 8, got %d", pool.MaxConcurrency())
	}
}

func TestDefaultConcurrency(t *testing.T) {
	pool, _ := setupPool(t, 0)
	if pool.MaxConcurrency() != 4 {
		t.Errorf("expected default concurrency 4, got %d", pool.MaxConcurrency())
	}
}

func TestSSEEvents(t *testing.T) {
	logger := testLogger()
	reg := registry.New(logger)
	reg.Register(newMockProvider(models.ProviderOpenAI))

	hub := sse.NewHub(logger)
	pool := NewPool(reg, hub, logger, 4)

	// Subscribe to SSE events.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, unsub := hub.Subscribe(ctx, "proj-1")
	defer unsub()

	pool.Submit(context.Background(), RunRequest{
		ProjectID:     "proj-1",
		WorkflowRunID: "run-1",
		Provider:      models.ProviderOpenAI,
	})

	// Should receive at least 2 events: run_started and run_completed.
	received := 0
	timeout := time.After(2 * time.Second)
	for received < 2 {
		select {
		case <-events:
			received++
		case <-timeout:
			t.Fatalf("timed out waiting for SSE events, got %d", received)
		}
	}
}

func TestNilSSEHub(t *testing.T) {
	logger := testLogger()
	reg := registry.New(logger)
	reg.Register(newMockProvider(models.ProviderOpenAI))

	// Pool with nil SSE hub should not panic.
	pool := NewPool(reg, nil, logger, 4)

	resp, err := pool.Submit(context.Background(), RunRequest{
		ProjectID:     "proj-1",
		WorkflowRunID: "run-1",
		Provider:      models.ProviderOpenAI,
	})
	if err != nil {
		t.Fatalf("Submit with nil hub: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("run error: %v", resp.Error)
	}
}
