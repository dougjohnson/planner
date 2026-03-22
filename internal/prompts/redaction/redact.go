// Package redaction provides credential scrubbing for prompt renders,
// raw payload snapshots, and log output. Two layers: exact-value redaction
// using known credentials, and regex-pattern redaction matching common
// API key formats. Both must run before any content is persisted.
package redaction

import (
	"regexp"
	"strings"
)

const (
	// RedactedPlaceholder replaces sensitive values in output.
	RedactedPlaceholder = "[REDACTED]"
)

// apiKeyPatterns matches common API key formats in text.
var apiKeyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[a-zA-Z0-9_-]{20,}`),            // OpenAI keys
	regexp.MustCompile(`sk-ant-[a-zA-Z0-9_-]{20,}`),        // Anthropic keys
	regexp.MustCompile(`sk-proj-[a-zA-Z0-9_-]{20,}`),       // OpenAI project keys
	regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9._\-/+=]{20,}`), // Bearer tokens
	regexp.MustCompile(`[a-fA-F0-9]{32,}`),                 // Long hex strings (32+ chars)
}

// Redactor scrubs credentials from text content using both exact-value
// matching and regex-pattern detection.
type Redactor struct {
	knownSecrets []string
}

// NewRedactor creates a redactor with the given set of known credential values.
// Pass the actual API key strings that are currently loaded so they can be
// matched exactly in addition to pattern-based detection.
func NewRedactor(knownSecrets []string) *Redactor {
	// Filter empty strings.
	secrets := make([]string, 0, len(knownSecrets))
	for _, s := range knownSecrets {
		s = strings.TrimSpace(s)
		if s != "" {
			secrets = append(secrets, s)
		}
	}
	return &Redactor{knownSecrets: secrets}
}

// Redact scrubs all known secrets and API key patterns from the input text.
// Returns the redacted text and whether any redaction occurred.
func (r *Redactor) Redact(text string) (string, bool) {
	if text == "" {
		return text, false
	}

	original := text
	redacted := false

	// Layer 1: Exact-value redaction (highest priority — known loaded credentials).
	for _, secret := range r.knownSecrets {
		if strings.Contains(text, secret) {
			text = strings.ReplaceAll(text, secret, RedactedPlaceholder)
			redacted = true
		}
	}

	// Layer 2: Regex-pattern redaction (catches keys not in the known set).
	for _, pat := range apiKeyPatterns {
		if pat.MatchString(text) {
			text = pat.ReplaceAllString(text, RedactedPlaceholder)
			redacted = true
		}
	}

	if !redacted {
		return original, false
	}
	return text, true
}

// RedactBytes is a convenience wrapper for byte slices.
func (r *Redactor) RedactBytes(data []byte) ([]byte, bool) {
	result, changed := r.Redact(string(data))
	if !changed {
		return data, false
	}
	return []byte(result), true
}

// QuickRedact performs pattern-only redaction without known secrets.
// Useful when the credential service is not available.
func QuickRedact(text string) string {
	for _, pat := range apiKeyPatterns {
		text = pat.ReplaceAllString(text, RedactedPlaceholder)
	}
	return text
}

// ContainsSecret checks if text contains any known secret or API key pattern.
// Useful for validation before persisting.
func (r *Redactor) ContainsSecret(text string) bool {
	for _, secret := range r.knownSecrets {
		if strings.Contains(text, secret) {
			return true
		}
	}
	for _, pat := range apiKeyPatterns {
		if pat.MatchString(text) {
			return true
		}
	}
	return false
}
