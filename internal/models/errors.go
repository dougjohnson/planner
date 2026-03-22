package models

import (
	"fmt"
	"time"
)

// ProviderError is the normalized error type returned by all provider adapters.
// The workflow engine, retry logic, UI messaging, and telemetry all consume
// this type rather than raw provider-specific failures.
type ProviderError struct {
	// Provider identifies which provider produced the error.
	Provider ProviderName `json:"provider"`
	// Retryable indicates whether the caller should attempt the request again.
	Retryable bool `json:"retryable"`
	// RetryAfter suggests how long to wait before retrying. Zero means no hint.
	RetryAfter time.Duration `json:"retry_after"`
	// Message is a human-readable description suitable for display.
	Message string `json:"message"`
	// Code is an optional provider-specific error code for diagnostics.
	Code string `json:"code,omitempty"`
	// Cause is the underlying error from the provider SDK, if any.
	Cause error `json:"-"`
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Provider, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Provider, e.Message)
}

// Unwrap returns the underlying cause for use with errors.Is/As.
func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// NewProviderError creates a non-retryable provider error.
func NewProviderError(provider ProviderName, message string, cause error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Retryable: false,
		Message:   message,
		Cause:     cause,
	}
}

// NewRetryableError creates a retryable provider error with an optional backoff hint.
func NewRetryableError(provider ProviderName, message string, retryAfter time.Duration, cause error) *ProviderError {
	return &ProviderError{
		Provider:   provider,
		Retryable:  true,
		RetryAfter: retryAfter,
		Message:    message,
		Cause:      cause,
	}
}
