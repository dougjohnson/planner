package registry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// mockProvider is a simple Provider implementation for testing.
type mockProvider struct {
	name         models.ProviderName
	executeFunc  func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error)
}

func newMockProvider(name models.ProviderName) *mockProvider {
	return &mockProvider{
		name: name,
		executeFunc: func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
			return &models.SessionResponse{
				ProviderID: "mock-response-id",
				Text:       "mock response from " + string(name),
				Usage: models.UsageMetadata{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
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
	return m.executeFunc(ctx, req)
}

func TestRegisterAndGet(t *testing.T) {
	reg := New(testLogger())
	mock := newMockProvider(models.ProviderOpenAI)

	reg.Register(mock)

	got := reg.Get(models.ProviderOpenAI)
	if got == nil {
		t.Fatal("expected registered provider")
	}
	if got.Name() != models.ProviderOpenAI {
		t.Errorf("expected openai, got %s", got.Name())
	}
}

func TestGet_NotRegistered(t *testing.T) {
	reg := New(testLogger())

	got := reg.Get(models.ProviderOpenAI)
	if got != nil {
		t.Error("expected nil for unregistered provider")
	}
}

func TestIsRegistered(t *testing.T) {
	reg := New(testLogger())
	reg.Register(newMockProvider(models.ProviderOpenAI))

	if !reg.IsRegistered(models.ProviderOpenAI) {
		t.Error("expected IsRegistered=true")
	}
	if reg.IsRegistered(models.ProviderAnthropic) {
		t.Error("expected IsRegistered=false for unregistered")
	}
}

func TestUnregister(t *testing.T) {
	reg := New(testLogger())
	reg.Register(newMockProvider(models.ProviderOpenAI))

	reg.Unregister(models.ProviderOpenAI)

	if reg.IsRegistered(models.ProviderOpenAI) {
		t.Error("expected provider to be unregistered")
	}
}

func TestRegisteredProviders(t *testing.T) {
	reg := New(testLogger())
	reg.Register(newMockProvider(models.ProviderOpenAI))
	reg.Register(newMockProvider(models.ProviderAnthropic))

	names := reg.RegisteredProviders()
	if len(names) != 2 {
		t.Errorf("expected 2 providers, got %d", len(names))
	}
}

func TestDispatch_Success(t *testing.T) {
	reg := New(testLogger())
	reg.Register(newMockProvider(models.ProviderOpenAI))

	resp, err := reg.Dispatch(context.Background(), models.ProviderOpenAI, models.SessionRequest{
		ModelID: "gpt-4o",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Text == "" {
		t.Error("expected non-empty response text")
	}
}

func TestDispatch_NotRegistered(t *testing.T) {
	reg := New(testLogger())

	_, err := reg.Dispatch(context.Background(), models.ProviderOpenAI, models.SessionRequest{})
	if err == nil {
		t.Fatal("expected error for unregistered provider")
	}
}

func TestDispatch_ProviderError(t *testing.T) {
	reg := New(testLogger())
	mock := newMockProvider(models.ProviderOpenAI)
	mock.executeFunc = func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
		return nil, fmt.Errorf("provider error")
	}
	reg.Register(mock)

	_, err := reg.Dispatch(context.Background(), models.ProviderOpenAI, models.SessionRequest{})
	if err == nil {
		t.Fatal("expected error from failing provider")
	}
}

func TestDispatchAll(t *testing.T) {
	reg := New(testLogger())
	reg.Register(newMockProvider(models.ProviderOpenAI))
	reg.Register(newMockProvider(models.ProviderAnthropic))

	results, err := reg.DispatchAll(context.Background(), models.SessionRequest{})
	if err != nil {
		t.Fatalf("DispatchAll: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Error != nil {
			t.Errorf("provider %s failed: %v", r.Provider, r.Error)
		}
		if r.Response == nil {
			t.Errorf("provider %s returned nil response", r.Provider)
		}
	}
}

func TestDispatchAll_NoProviders(t *testing.T) {
	reg := New(testLogger())

	_, err := reg.DispatchAll(context.Background(), models.SessionRequest{})
	if err == nil {
		t.Fatal("expected error when no providers registered")
	}
}

func TestDispatchAll_PartialFailure(t *testing.T) {
	reg := New(testLogger())

	good := newMockProvider(models.ProviderOpenAI)
	bad := newMockProvider(models.ProviderAnthropic)
	bad.executeFunc = func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
		return nil, fmt.Errorf("anthropic failure")
	}

	reg.Register(good)
	reg.Register(bad)

	results, err := reg.DispatchAll(context.Background(), models.SessionRequest{})
	if err != nil {
		t.Fatalf("DispatchAll should not fail on partial failure: %v", err)
	}

	successes := 0
	failures := 0
	for _, r := range results {
		if r.Error != nil {
			failures++
		} else {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected 1 success, got %d", successes)
	}
	if failures != 1 {
		t.Errorf("expected 1 failure, got %d", failures)
	}
}

func TestHealth_SuccessTracking(t *testing.T) {
	reg := New(testLogger())
	reg.Register(newMockProvider(models.ProviderOpenAI))

	reg.Dispatch(context.Background(), models.ProviderOpenAI, models.SessionRequest{})

	health, ok := reg.Health(models.ProviderOpenAI)
	if !ok {
		t.Fatal("expected health info")
	}
	if health.LastSuccessAt == nil {
		t.Error("expected LastSuccessAt to be set")
	}
	if health.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures, got %d", health.ConsecutiveFailures)
	}
	if health.Degraded {
		t.Error("expected not degraded")
	}
}

func TestHealth_DegradedAfterFailures(t *testing.T) {
	reg := New(testLogger())
	mock := newMockProvider(models.ProviderOpenAI)
	mock.executeFunc = func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
		return nil, fmt.Errorf("error")
	}
	reg.Register(mock)

	for range degradedThreshold {
		reg.Dispatch(context.Background(), models.ProviderOpenAI, models.SessionRequest{})
	}

	health, _ := reg.Health(models.ProviderOpenAI)
	if !health.Degraded {
		t.Error("expected provider to be degraded after threshold failures")
	}
	if health.ConsecutiveFailures != degradedThreshold {
		t.Errorf("expected %d failures, got %d", degradedThreshold, health.ConsecutiveFailures)
	}
}

func TestHealth_RecoveryAfterSuccess(t *testing.T) {
	reg := New(testLogger())
	failCount := 0
	mock := newMockProvider(models.ProviderOpenAI)
	mock.executeFunc = func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
		failCount++
		if failCount <= degradedThreshold {
			return nil, fmt.Errorf("error")
		}
		return &models.SessionResponse{Text: "recovered"}, nil
	}
	reg.Register(mock)

	// Fail enough times to degrade.
	for range degradedThreshold {
		reg.Dispatch(context.Background(), models.ProviderOpenAI, models.SessionRequest{})
	}

	health, _ := reg.Health(models.ProviderOpenAI)
	if !health.Degraded {
		t.Fatal("expected degraded")
	}

	// One success should recover.
	reg.Dispatch(context.Background(), models.ProviderOpenAI, models.SessionRequest{})

	health, _ = reg.Health(models.ProviderOpenAI)
	if health.Degraded {
		t.Error("expected recovery after success")
	}
	if health.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 failures after recovery, got %d", health.ConsecutiveFailures)
	}
}

func TestHealth_NotRegistered(t *testing.T) {
	reg := New(testLogger())

	_, ok := reg.Health(models.ProviderOpenAI)
	if ok {
		t.Error("expected ok=false for unregistered provider")
	}
}

func TestAllHealth(t *testing.T) {
	reg := New(testLogger())
	reg.Register(newMockProvider(models.ProviderOpenAI))
	reg.Register(newMockProvider(models.ProviderAnthropic))

	health := reg.AllHealth()
	if len(health) != 2 {
		t.Errorf("expected 2 health entries, got %d", len(health))
	}
}

func TestConcurrentDispatch(t *testing.T) {
	reg := New(testLogger())
	mock := newMockProvider(models.ProviderOpenAI)
	mock.executeFunc = func(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
		time.Sleep(time.Millisecond) // Simulate work.
		return &models.SessionResponse{Text: "ok"}, nil
	}
	reg.Register(mock)

	const n = 10
	var wg sync.WaitGroup
	errors := make(chan error, n)

	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := reg.Dispatch(context.Background(), models.ProviderOpenAI, models.SessionRequest{})
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent dispatch error: %v", err)
	}
}
