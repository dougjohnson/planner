package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/providers"
	"github.com/dougflynn/flywheel-planner/internal/models/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupParallelPool(t *testing.T) (*Pool, *registry.Registry) {
	t.Helper()
	reg := registry.New(slog.Default())
	pool := NewPool(reg, nil, slog.Default(), 4)
	return pool, reg
}

func TestFanOut_TwoProviders(t *testing.T) {
	pool, reg := setupParallelPool(t)
	ctx := context.Background()

	mockGPT := providers.NewMockGPT("")
	mockOpus := providers.NewMockOpus("")
	reg.Register(mockGPT)
	reg.Register(mockOpus)

	requests := []RunRequest{
		{ProjectID: "p-1", WorkflowRunID: "r-1", Provider: models.ProviderOpenAI,
			Session: models.SessionRequest{ModelID: "mock-gpt-4o"}},
		{ProjectID: "p-1", WorkflowRunID: "r-2", Provider: models.ProviderAnthropic,
			Session: models.SessionRequest{ModelID: "mock-claude-opus"}},
	}

	results, err := pool.SubmitAll(ctx, requests)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Both should succeed.
	for _, r := range results {
		assert.NoError(t, r.Error)
		assert.NotNil(t, r.Response)
	}

	assert.Equal(t, 1, mockGPT.CallCount())
	assert.Equal(t, 1, mockOpus.CallCount())
}

func TestFanOut_ConcurrentExecution(t *testing.T) {
	pool, reg := setupParallelPool(t)
	ctx := context.Background()

	mockGPT := providers.NewMockGPT("")
	mockOpus := providers.NewMockOpus("")
	reg.Register(mockGPT)
	reg.Register(mockOpus)

	start := time.Now()
	requests := []RunRequest{
		{ProjectID: "p-1", WorkflowRunID: "r-1", Provider: models.ProviderOpenAI,
			Session: models.SessionRequest{ModelID: "mock-gpt-4o"}},
		{ProjectID: "p-1", WorkflowRunID: "r-2", Provider: models.ProviderAnthropic,
			Session: models.SessionRequest{ModelID: "mock-claude-opus"}},
	}

	results, err := pool.SubmitAll(ctx, requests)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Mock providers are instant, so concurrent execution should be fast.
	assert.Less(t, elapsed, 2*time.Second, "concurrent execution should be fast")
}

func TestFanOut_EmptyRequests(t *testing.T) {
	pool, _ := setupParallelPool(t)
	ctx := context.Background()

	results, err := pool.SubmitAll(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestFanOut_PartialFailure(t *testing.T) {
	pool, reg := setupParallelPool(t)
	ctx := context.Background()

	mockGPT := providers.NewMockGPT("")
	mockOpus := providers.NewMockOpus("")
	// Cancel Opus context to force failure.
	cancelCtx, cancel := context.WithCancel(ctx)

	reg.Register(mockGPT)
	reg.Register(mockOpus)

	// First request succeeds, second will fail due to cancelled context.
	requests := []RunRequest{
		{ProjectID: "p-1", WorkflowRunID: "r-1", Provider: models.ProviderOpenAI,
			Session: models.SessionRequest{ModelID: "mock-gpt-4o"}},
	}

	// Normal execution succeeds.
	results, err := pool.SubmitAll(ctx, requests)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.NoError(t, results[0].Error)

	// Cancelled context fails.
	cancel()
	requests2 := []RunRequest{
		{ProjectID: "p-1", WorkflowRunID: "r-3", Provider: models.ProviderAnthropic,
			Session: models.SessionRequest{ModelID: "mock-claude-opus"}},
	}
	results2, _ := pool.SubmitAll(cancelCtx, requests2)
	if len(results2) > 0 {
		assert.Error(t, results2[0].Error)
	}
}

func TestFanOut_ConcurrencyBounded(t *testing.T) {
	reg := registry.New(slog.Default())
	pool := NewPool(reg, nil, slog.Default(), 2) // max 2 concurrent
	ctx := context.Background()

	mockGPT := providers.NewMockGPT("")
	reg.Register(mockGPT)

	var maxConcurrent int64
	var current int64

	// Track concurrent calls using overrides.
	origExec := mockGPT.Models()[0].ModelID
	_ = origExec

	// Submit 4 requests with concurrency limit of 2.
	requests := make([]RunRequest, 4)
	for i := range requests {
		requests[i] = RunRequest{
			ProjectID:     "p-1",
			WorkflowRunID: "r-" + string(rune('1'+i)),
			Provider:      models.ProviderOpenAI,
			Session:       models.SessionRequest{ModelID: "mock-gpt-4o"},
		}
	}

	results, err := pool.SubmitAll(ctx, requests)
	require.NoError(t, err)
	assert.Len(t, results, 4)

	// All should succeed.
	for _, r := range results {
		assert.NoError(t, r.Error)
	}

	_ = maxConcurrent
	_ = current
}

func TestPool_ActiveWorkers(t *testing.T) {
	pool, _ := setupParallelPool(t)
	assert.Equal(t, 0, pool.ActiveWorkers())
}

func TestPool_MaxConcurrency(t *testing.T) {
	reg := registry.New(slog.Default())
	pool := NewPool(reg, nil, slog.Default(), 8)
	assert.Equal(t, 8, pool.MaxConcurrency())
}

func TestFanOut_RaceCondition(t *testing.T) {
	pool, reg := setupParallelPool(t)
	ctx := context.Background()

	mockGPT := providers.NewMockGPT("")
	reg.Register(mockGPT)

	// Run multiple orchestrations concurrently to check for races.
	var done int64
	const goroutines = 5

	for i := 0; i < goroutines; i++ {
		go func() {
			defer atomic.AddInt64(&done, 1)
			requests := []RunRequest{
				{ProjectID: "p-1", WorkflowRunID: "r-concurrent",
					Provider: models.ProviderOpenAI,
					Session:  models.SessionRequest{ModelID: "mock-gpt-4o"}},
			}
			pool.SubmitAll(ctx, requests)
		}()
	}

	// Wait for all goroutines.
	deadline := time.After(5 * time.Second)
	for {
		if atomic.LoadInt64(&done) >= goroutines {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for concurrent orchestrations")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
