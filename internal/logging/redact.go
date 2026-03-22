// Package logging provides structured logging infrastructure for flywheel-planner,
// including credential redaction that must be active before any provider interaction.
package logging

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

const redactedValue = "[REDACTED]"

// sensitiveFields contains exact field names (lowercased) whose values must be redacted.
var sensitiveFields = map[string]bool{
	"api_key":       true,
	"apikey":        true,
	"secret":        true,
	"token":         true,
	"password":      true,
	"authorization": true,
	"auth":          true,
	"credential":    true,
	"credentials":   true,
	"secret_key":    true,
	"access_key":    true,
	"private_key":   true,
}

// apiKeyPattern matches common API key formats in string values:
// - sk-... (OpenAI keys)
// - sk-ant-... (Anthropic keys)
// - Bearer ... tokens
// - Long hex strings (32+ chars)
var apiKeyPattern = regexp.MustCompile(
	`(?i)` +
		`(sk-[a-zA-Z0-9_-]{20,})` + // OpenAI-style keys
		`|(sk-ant-[a-zA-Z0-9_-]{20,})` + // Anthropic-style keys
		`|(bearer\s+[a-zA-Z0-9._\-/+=]{20,})` + // Bearer tokens
		`|([a-fA-F0-9]{32,})`, // Long hex strings
)

// RedactingHandler wraps a slog.Handler and scrubs sensitive attributes
// before passing log records to the underlying handler. It strips:
//   - Attributes whose keys match known sensitive field names (case-insensitive)
//   - String values that match common API key patterns
//
// This handler must be installed as the outermost wrapper to guarantee
// that no credential data reaches any log sink.
type RedactingHandler struct {
	inner slog.Handler
}

// NewRedactingHandler wraps the given handler with credential redaction.
func NewRedactingHandler(inner slog.Handler) *RedactingHandler {
	return &RedactingHandler{inner: inner}
}

// Enabled delegates to the inner handler.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle redacts sensitive attributes in the record before passing to the inner handler.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	var redacted []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		redacted = append(redacted, redactAttr(a))
		return true
	})

	// Build a new record without the original attrs.
	clean := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	clean.AddAttrs(redacted...)

	return h.inner.Handle(ctx, clean)
}

// WithAttrs returns a new handler with the given pre-redacted attributes.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = redactAttr(a)
	}
	return &RedactingHandler{inner: h.inner.WithAttrs(redacted)}
}

// WithGroup returns a new handler that opens a group. Group names themselves
// are never sensitive, so we pass through to the inner handler.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{inner: h.inner.WithGroup(name)}
}

// redactAttr returns a copy of the attribute with sensitive data replaced.
func redactAttr(a slog.Attr) slog.Attr {
	// Check if the field name itself is sensitive.
	if isSensitiveKey(a.Key) {
		return slog.String(a.Key, redactedValue)
	}

	// Handle groups recursively.
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		redacted := make([]slog.Attr, len(attrs))
		for i, ga := range attrs {
			redacted[i] = redactAttr(ga)
		}
		return slog.Group(a.Key, attrsToAny(redacted)...)
	}

	// Check string values for API key patterns.
	if a.Value.Kind() == slog.KindString {
		s := a.Value.String()
		if apiKeyPattern.MatchString(s) {
			return slog.String(a.Key, redactedValue)
		}
	}

	return a
}

// isSensitiveKey checks if a key name matches a known sensitive field.
func isSensitiveKey(key string) bool {
	return sensitiveFields[strings.ToLower(key)]
}

// attrsToAny converts a slice of slog.Attr to []any for slog.Group.
func attrsToAny(attrs []slog.Attr) []any {
	result := make([]any, len(attrs))
	for i, a := range attrs {
		result[i] = a
	}
	return result
}

// NewLogger creates a new slog.Logger with JSON output and credential redaction.
// This is the standard way to create a logger for flywheel-planner.
func NewLogger(level slog.Level) *slog.Logger {
	return NewLoggerWithHandler(slog.NewJSONHandler(nil, &slog.HandlerOptions{
		Level: level,
	}))
}

// NewLoggerWithHandler creates a new slog.Logger that wraps the given handler
// with credential redaction.
func NewLoggerWithHandler(inner slog.Handler) *slog.Logger {
	return slog.New(NewRedactingHandler(inner))
}
