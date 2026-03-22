package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// --- Error Normalization ---

// NormalizeHTTPError converts an HTTP status code and body from a provider
// into a normalized ProviderError. This handles the common error patterns
// across OpenAI and Anthropic APIs.
func NormalizeHTTPError(provider ProviderName, statusCode int, body []byte, headers http.Header) *ProviderError {
	retryAfter := parseRetryAfter(headers)

	switch {
	case statusCode == 429:
		return &ProviderError{
			Provider:   provider,
			Retryable:  true,
			RetryAfter: retryAfter,
			Message:    "rate limit exceeded",
			Code:       "rate_limit",
		}

	case statusCode == 401 || statusCode == 403:
		return &ProviderError{
			Provider:  provider,
			Retryable: false,
			Message:   "authentication failed — check your API key",
			Code:      "auth_error",
		}

	case statusCode == 400:
		msg := extractErrorMessage(body)
		if msg == "" {
			msg = "invalid request"
		}
		return &ProviderError{
			Provider:  provider,
			Retryable: false,
			Message:   msg,
			Code:      "invalid_request",
		}

	case statusCode == 500 || statusCode == 502 || statusCode == 503:
		return &ProviderError{
			Provider:   provider,
			Retryable:  true,
			RetryAfter: retryAfter,
			Message:    fmt.Sprintf("provider server error (HTTP %d)", statusCode),
			Code:       "server_error",
		}

	case statusCode == 408 || statusCode == 504:
		return &ProviderError{
			Provider:   provider,
			Retryable:  true,
			RetryAfter: retryAfter,
			Message:    "request timed out",
			Code:       "timeout",
		}

	default:
		return &ProviderError{
			Provider:  provider,
			Retryable: false,
			Message:   fmt.Sprintf("unexpected HTTP %d", statusCode),
			Code:      fmt.Sprintf("http_%d", statusCode),
		}
	}
}

// NormalizeTimeoutError wraps a context timeout or deadline exceeded error.
func NormalizeTimeoutError(provider ProviderName, cause error) *ProviderError {
	return &ProviderError{
		Provider:   provider,
		Retryable:  true,
		RetryAfter: 5 * time.Second,
		Message:    "request timed out",
		Code:       "timeout",
		Cause:      cause,
	}
}

// NormalizeCancellationError wraps a context cancellation.
func NormalizeCancellationError(provider ProviderName, cause error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Retryable: false,
		Message:   "request cancelled by user",
		Code:      "cancelled",
		Cause:     cause,
	}
}

// NormalizeUnknownError wraps any unrecognized error as non-retryable.
func NormalizeUnknownError(provider ProviderName, cause error) *ProviderError {
	return &ProviderError{
		Provider:  provider,
		Retryable: false,
		Message:   cause.Error(),
		Code:      "unknown",
		Cause:     cause,
	}
}

// --- Tool-Call Result Normalization ---

// NormalizedToolCallResult is a validated, normalized record of a tool call
// extracted from a provider response.
type NormalizedToolCallResult struct {
	// ToolCall is the normalized tool invocation.
	ToolCall ToolCall `json:"tool_call"`
	// Valid indicates whether the tool call passed schema validation.
	Valid bool `json:"valid"`
	// ValidationErrors lists specific issues found during validation.
	ValidationErrors []string `json:"validation_errors,omitempty"`
	// RawRepresentation preserves the original provider format for lineage.
	RawRepresentation json.RawMessage `json:"raw_representation,omitempty"`
}

// NormalizeToolCalls extracts and validates tool calls from a raw provider
// response using the appropriate translator and validates against schemas.
func NormalizeToolCalls(provider ProviderName, rawResponse json.RawMessage, expectedTools []ToolSchema) ([]NormalizedToolCallResult, error) {
	translator := TranslatorForProvider(provider)
	calls, err := translator.ParseToolCalls(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("parsing tool calls from %s response: %w", provider, err)
	}

	results := make([]NormalizedToolCallResult, 0, len(calls))
	for _, call := range calls {
		result := NormalizedToolCallResult{
			ToolCall: call,
			Valid:    true,
		}

		// Preserve raw representation.
		raw, _ := json.Marshal(call)
		result.RawRepresentation = raw

		// Find the matching tool schema.
		schema := findToolSchema(call.Name, expectedTools)
		if schema == nil {
			result.Valid = false
			result.ValidationErrors = append(result.ValidationErrors,
				fmt.Sprintf("unknown tool: %q", call.Name))
		} else {
			// Validate required arguments.
			errors := validateToolArguments(call.Arguments, schema.Parameters)
			if len(errors) > 0 {
				result.Valid = false
				result.ValidationErrors = errors
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// --- Helpers ---

func parseRetryAfter(headers http.Header) time.Duration {
	val := headers.Get("Retry-After")
	if val == "" {
		return 0
	}
	// Try seconds first.
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Duration(secs) * time.Second
	}
	// Try HTTP date format.
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

func extractErrorMessage(body []byte) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	// Anthropic format.
	var alt struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &alt) == nil {
		if alt.Error.Message != "" {
			return alt.Error.Message
		}
		if alt.Message != "" {
			return alt.Message
		}
	}
	return ""
}

func findToolSchema(name string, tools []ToolSchema) *ToolSchema {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}

func validateToolArguments(args map[string]any, schema JSONSchema) []string {
	var errors []string

	// Check required fields.
	for _, req := range schema.Required {
		val, exists := args[req]
		if !exists {
			errors = append(errors, fmt.Sprintf("missing required argument: %q", req))
			continue
		}
		// Check type for string fields.
		if prop, ok := schema.Properties[req]; ok && prop.Type == "string" {
			if _, isStr := val.(string); !isStr {
				errors = append(errors, fmt.Sprintf("argument %q must be a string", req))
			}
		}
	}

	// Check enum constraints.
	for name, prop := range schema.Properties {
		if len(prop.Enum) == 0 {
			continue
		}
		val, exists := args[name]
		if !exists {
			continue
		}
		strVal, ok := val.(string)
		if !ok {
			continue
		}
		found := false
		for _, allowed := range prop.Enum {
			if strVal == allowed {
				found = true
				break
			}
		}
		if !found {
			errors = append(errors, fmt.Sprintf("argument %q value %q not in allowed values: [%s]",
				name, strVal, strings.Join(prop.Enum, ", ")))
		}
	}

	return errors
}
