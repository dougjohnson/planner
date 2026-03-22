package models

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// ValidationStatus represents the outcome of credential validation.
type ValidationStatus string

const (
	ValidationUnchecked ValidationStatus = "unchecked"
	ValidationValid     ValidationStatus = "valid"
	ValidationInvalid   ValidationStatus = "invalid"
	ValidationError     ValidationStatus = "error"
)

// ValidationResult contains the outcome of validating a provider's credentials.
type ValidationResult struct {
	Provider         ProviderName     `json:"provider"`
	Status           ValidationStatus `json:"status"`
	Message          string           `json:"message,omitempty"`
	ValidatedAt      time.Time        `json:"validated_at"`
}

// CredentialValidator validates provider credentials via minimal API calls.
// It never logs or persists the credential value itself — only the outcome.
type CredentialValidator struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewCredentialValidator creates a new validator.
func NewCredentialValidator(db *sql.DB, logger *slog.Logger) *CredentialValidator {
	return &CredentialValidator{db: db, logger: logger}
}

// Validate checks if a provider's credentials are valid by calling
// Provider.ValidateCredentials. Updates the model_config's validation_status.
func (v *CredentialValidator) Validate(ctx context.Context, provider Provider) *ValidationResult {
	result := &ValidationResult{
		Provider:    provider.Name(),
		ValidatedAt: time.Now().UTC(),
	}

	err := provider.ValidateCredentials(ctx)
	if err == nil {
		result.Status = ValidationValid
		result.Message = "credentials validated successfully"
		v.logger.Info("credential validation passed",
			"provider", provider.Name(),
		)
	} else {
		// Classify the error.
		var provErr *ProviderError
		if ok := isProviderError(err, &provErr); ok && !provErr.Retryable {
			result.Status = ValidationInvalid
			result.Message = "invalid credentials: " + provErr.Message
		} else {
			result.Status = ValidationError
			result.Message = "validation failed: " + err.Error()
		}
		v.logger.Warn("credential validation failed",
			"provider", provider.Name(),
			"status", result.Status,
			// Never log the actual credential or detailed error that might contain it.
		)
	}

	// Update model_config validation_status if DB is available.
	if v.db != nil {
		v.updateModelConfigStatus(ctx, string(provider.Name()), result.Status)
	}

	return result
}

// ValidateAll validates credentials for all provided providers.
func (v *CredentialValidator) ValidateAll(ctx context.Context, providers []Provider) []*ValidationResult {
	results := make([]*ValidationResult, len(providers))
	for i, p := range providers {
		results[i] = v.Validate(ctx, p)
	}
	return results
}

// updateModelConfigStatus updates the validation_status on model_configs.
func (v *CredentialValidator) updateModelConfigStatus(ctx context.Context, provider string, status ValidationStatus) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := v.db.ExecContext(ctx,
		"UPDATE model_configs SET validation_status = ?, updated_at = ? WHERE provider = ?",
		string(status), now, provider,
	)
	if err != nil {
		v.logger.Warn("failed to update validation status",
			"provider", provider,
			"error", err,
		)
	}
}

// isProviderError checks if err wraps a ProviderError.
func isProviderError(err error, target **ProviderError) bool {
	if err == nil {
		return false
	}
	type unwrapper interface {
		Unwrap() error
	}
	// Simple type assertion — errors.As would be better but we avoid import cycle risk.
	if pe, ok := err.(*ProviderError); ok {
		*target = pe
		return true
	}
	if u, ok := err.(unwrapper); ok {
		return isProviderError(u.Unwrap(), target)
	}
	return false
}

// GetValidationStatus returns the current validation status for a provider from the DB.
func (v *CredentialValidator) GetValidationStatus(ctx context.Context, provider string) (ValidationStatus, error) {
	var status string
	err := v.db.QueryRowContext(ctx,
		"SELECT validation_status FROM model_configs WHERE provider = ? LIMIT 1",
		provider,
	).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return ValidationUnchecked, nil
		}
		return "", fmt.Errorf("querying validation status: %w", err)
	}
	return ValidationStatus(status), nil
}
