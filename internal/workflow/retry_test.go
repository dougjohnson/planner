package workflow

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

func testRetryLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestRetryExecutor_SuccessOnFirstAttempt(t *testing.T) {
	re := NewRetryExecutor(DefaultRetryPolicy(), testRetryLogger())

	result := re.Execute(context.Background(), func(ctx context.Context) (*models.SessionResponse, error) {
		return &models.SessionResponse{Text: "success"}, nil
	})

	if result.Err != nil {
		t.Fatalf("expected success, got: %v", result.Err)
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempts)
	}
	if result.Response.Text != "success" {
		t.Errorf("expected 'success', got %q", result.Response.Text)
	}
}

func TestRetryExecutor_SuccessOnRetry(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	re := NewRetryExecutor(policy, testRetryLogger())

	callCount := 0
	result := re.Execute(context.Background(), func(ctx context.Context) (*models.SessionResponse, error) {
		callCount++
		if callCount < 3 {
			return nil, models.NewRetryableError(models.ProviderOpenAI, "rate limit", 0, nil)
		}
		return &models.SessionResponse{Text: "recovered"}, nil
	})

	if result.Err != nil {
		t.Fatalf("expected success after retry, got: %v", result.Err)
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", result.Attempts)
	}
	if result.Response.Text != "recovered" {
		t.Errorf("expected 'recovered', got %q", result.Response.Text)
	}
}

func TestRetryExecutor_ExhaustedAttempts(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
	re := NewRetryExecutor(policy, testRetryLogger())

	result := re.Execute(context.Background(), func(ctx context.Context) (*models.SessionResponse, error) {
		return nil, models.NewRetryableError(models.ProviderOpenAI, "always fails", 0, nil)
	})

	if result.Err == nil {
		t.Fatal("expected error after exhausting attempts")
	}
	if result.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", result.Attempts)
	}
	if result.LastRetryableErr == nil {
		t.Error("expected LastRetryableErr to be set")
	}
}

func TestRetryExecutor_NonRetryableError(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
	re := NewRetryExecutor(policy, testRetryLogger())

	result := re.Execute(context.Background(), func(ctx context.Context) (*models.SessionResponse, error) {
		return nil, models.NewProviderError(models.ProviderOpenAI, "invalid credentials", nil)
	})

	if result.Err == nil {
		t.Fatal("expected error for non-retryable failure")
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt (no retry for non-retryable), got %d", result.Attempts)
	}
}

func TestRetryExecutor_ContextCancelled(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 5, BaseDelay: time.Second, MaxDelay: 5 * time.Second}
	re := NewRetryExecutor(policy, testRetryLogger())

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	result := re.Execute(ctx, func(ctx context.Context) (*models.SessionResponse, error) {
		callCount++
		if callCount == 1 {
			// First attempt fails retryably, then cancel during backoff.
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
			return nil, models.NewRetryableError(models.ProviderOpenAI, "transient", 0, nil)
		}
		return &models.SessionResponse{Text: "should not reach"}, nil
	})

	if result.Err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestRetryExecutor_HonorsRetryAfter(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 2, BaseDelay: 10 * time.Second, MaxDelay: 30 * time.Second}
	re := NewRetryExecutor(policy, testRetryLogger())

	start := time.Now()
	callCount := 0
	re.Execute(context.Background(), func(ctx context.Context) (*models.SessionResponse, error) {
		callCount++
		if callCount == 1 {
			// Provider says retry after 50ms (much shorter than baseDelay of 10s).
			return nil, models.NewRetryableError(models.ProviderOpenAI, "rate limit", 50*time.Millisecond, nil)
		}
		return &models.SessionResponse{Text: "ok"}, nil
	})

	elapsed := time.Since(start)
	// Should have waited ~50ms (the retry-after), not 10s (the base delay).
	if elapsed > 2*time.Second {
		t.Errorf("expected fast retry with RetryAfter hint, took %v", elapsed)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"retryable provider error", models.NewRetryableError(models.ProviderOpenAI, "rate limit", 0, nil), true},
		{"non-retryable provider error", models.NewProviderError(models.ProviderOpenAI, "invalid creds", nil), false},
		{"context cancelled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"generic error", fmt.Errorf("unknown error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			if got != tt.expected {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestComputeBackoff(t *testing.T) {
	base := 100 * time.Millisecond
	maxD := 5 * time.Second

	// Attempt 1: 100ms.
	d1 := ComputeBackoff(1, base, maxD)
	if d1 != 100*time.Millisecond {
		t.Errorf("attempt 1: expected 100ms, got %v", d1)
	}

	// Attempt 2: 200ms.
	d2 := ComputeBackoff(2, base, maxD)
	if d2 != 200*time.Millisecond {
		t.Errorf("attempt 2: expected 200ms, got %v", d2)
	}

	// Attempt 3: 400ms.
	d3 := ComputeBackoff(3, base, maxD)
	if d3 != 400*time.Millisecond {
		t.Errorf("attempt 3: expected 400ms, got %v", d3)
	}

	// Very high attempt: capped at maxDelay.
	d100 := ComputeBackoff(100, base, maxD)
	if d100 != maxD {
		t.Errorf("attempt 100: expected max %v, got %v", maxD, d100)
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.MaxAttempts != DefaultMaxAttempts {
		t.Errorf("expected %d attempts, got %d", DefaultMaxAttempts, p.MaxAttempts)
	}
	if p.BaseDelay != DefaultBaseDelay {
		t.Errorf("expected base delay %v, got %v", DefaultBaseDelay, p.BaseDelay)
	}
	if p.MaxDelay != DefaultMaxDelay {
		t.Errorf("expected max delay %v, got %v", DefaultMaxDelay, p.MaxDelay)
	}
}

func TestRetryExecutor_DefaultsForZeroPolicy(t *testing.T) {
	re := NewRetryExecutor(RetryPolicy{}, testRetryLogger())
	if re.policy.MaxAttempts != DefaultMaxAttempts {
		t.Errorf("expected default max attempts")
	}
	if re.policy.BaseDelay != DefaultBaseDelay {
		t.Errorf("expected default base delay")
	}
}
