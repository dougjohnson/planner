package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

const (
	// DefaultMaxAttempts is the default maximum number of attempts per run.
	DefaultMaxAttempts = 3

	// DefaultBaseDelay is the base delay for exponential backoff.
	DefaultBaseDelay = 1 * time.Second

	// DefaultMaxDelay caps the backoff to prevent unreasonable waits.
	DefaultMaxDelay = 30 * time.Second

	// BackoffMultiplier is the exponential factor between retries.
	BackoffMultiplier = 2.0
)

// RetryPolicy configures retry behavior for a stage execution.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of attempts (including the initial try).
	MaxAttempts int

	// BaseDelay is the initial backoff delay.
	BaseDelay time.Duration

	// MaxDelay caps the exponential backoff.
	MaxDelay time.Duration
}

// DefaultRetryPolicy returns the default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: DefaultMaxAttempts,
		BaseDelay:   DefaultBaseDelay,
		MaxDelay:    DefaultMaxDelay,
	}
}

// RetryResult captures the outcome of a retry-managed execution.
type RetryResult struct {
	// Response is the successful response (nil if all attempts failed).
	Response *models.SessionResponse
	// Err is the final error if all attempts were exhausted.
	Err error
	// Attempts is the total number of attempts made.
	Attempts int
	// LastRetryableErr is the last retryable error encountered (for diagnostics).
	LastRetryableErr error
}

// RetryExecutor manages retries for model execution calls.
type RetryExecutor struct {
	policy RetryPolicy
	logger *slog.Logger
}

// NewRetryExecutor creates a new retry executor with the given policy.
func NewRetryExecutor(policy RetryPolicy, logger *slog.Logger) *RetryExecutor {
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = DefaultMaxAttempts
	}
	if policy.BaseDelay <= 0 {
		policy.BaseDelay = DefaultBaseDelay
	}
	if policy.MaxDelay <= 0 {
		policy.MaxDelay = DefaultMaxDelay
	}
	return &RetryExecutor{policy: policy, logger: logger}
}

// Execute runs the given function with retry logic. The function is retried
// only for retryable errors (as determined by IsRetryable). Non-retryable
// errors cause immediate return.
//
// If the error includes a RetryAfter hint, that duration is used instead
// of the exponential backoff.
func (re *RetryExecutor) Execute(ctx context.Context, fn func(ctx context.Context) (*models.SessionResponse, error)) *RetryResult {
	result := &RetryResult{}

	for attempt := 1; attempt <= re.policy.MaxAttempts; attempt++ {
		result.Attempts = attempt

		resp, err := fn(ctx)
		if err == nil {
			result.Response = resp
			return result
		}

		// Check if the error is retryable.
		if !IsRetryable(err) {
			result.Err = fmt.Errorf("non-retryable error on attempt %d: %w", attempt, err)
			re.logger.Warn("non-retryable error, stopping",
				"attempt", attempt,
				"error", err,
			)
			return result
		}

		result.LastRetryableErr = err

		// If this was the last attempt, don't wait.
		if attempt == re.policy.MaxAttempts {
			result.Err = fmt.Errorf("exhausted %d attempts: %w", re.policy.MaxAttempts, err)
			re.logger.Warn("retry budget exhausted",
				"attempts", attempt,
				"last_error", err,
			)
			return result
		}

		// Compute backoff delay.
		delay := re.computeDelay(attempt, err)

		re.logger.Info("retrying after error",
			"attempt", attempt,
			"max_attempts", re.policy.MaxAttempts,
			"delay", delay,
			"error", err,
		)

		// Wait for the delay or context cancellation.
		select {
		case <-time.After(delay):
			// Continue to next attempt.
		case <-ctx.Done():
			result.Err = fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			return result
		}
	}

	return result
}

// computeDelay calculates the backoff delay for a given attempt.
// If the error includes a RetryAfter hint, that value is used instead.
func (re *RetryExecutor) computeDelay(attempt int, err error) time.Duration {
	// Check for provider-specified retry-after.
	var provErr *models.ProviderError
	if errors.As(err, &provErr) && provErr.RetryAfter > 0 {
		if provErr.RetryAfter > re.policy.MaxDelay {
			return re.policy.MaxDelay
		}
		return provErr.RetryAfter
	}

	// Exponential backoff: base * multiplier^(attempt-1)
	return ComputeBackoff(attempt, re.policy.BaseDelay, re.policy.MaxDelay)
}

// IsRetryable determines whether an error should trigger a retry.
// Retryable: rate limits, timeouts, transient upstream failures.
// Non-retryable: invalid credentials, invalid requests, user cancellation.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for ProviderError with retryable flag.
	var provErr *models.ProviderError
	if errors.As(err, &provErr) {
		return provErr.Retryable
	}

	// Context cancellation is never retryable.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Default: not retryable (fail-safe).
	return false
}

// ComputeBackoff returns the backoff duration for a given attempt number
// using exponential backoff. Exported for testing and external use.
func ComputeBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	multiplied := float64(baseDelay) * math.Pow(BackoffMultiplier, float64(attempt-1))
	if multiplied > float64(maxDelay) || multiplied < 0 {
		return maxDelay
	}
	return time.Duration(multiplied)
}
