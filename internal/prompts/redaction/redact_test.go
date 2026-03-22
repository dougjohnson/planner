package redaction

import (
	"strings"
	"testing"
)

func TestRedact_ExactValue(t *testing.T) {
	r := NewRedactor([]string{"my-secret-api-key-123"})

	text := "Using API key: my-secret-api-key-123 for auth"
	result, changed := r.Redact(text)
	if !changed {
		t.Fatal("expected redaction")
	}
	if strings.Contains(result, "my-secret-api-key-123") {
		t.Error("secret should be redacted")
	}
	if !strings.Contains(result, RedactedPlaceholder) {
		t.Error("should contain placeholder")
	}
}

func TestRedact_MultipleExactValues(t *testing.T) {
	r := NewRedactor([]string{"secret-one", "secret-two"})

	text := "Keys: secret-one and secret-two"
	result, changed := r.Redact(text)
	if !changed {
		t.Fatal("expected redaction")
	}
	if strings.Contains(result, "secret-one") || strings.Contains(result, "secret-two") {
		t.Error("all secrets should be redacted")
	}
}

func TestRedact_OpenAIKeyPattern(t *testing.T) {
	r := NewRedactor(nil)

	text := "Key is sk-proj-abc123def456ghi789jkl012mno345"
	result, changed := r.Redact(text)
	if !changed {
		t.Fatal("expected pattern redaction")
	}
	if strings.Contains(result, "sk-proj-") {
		t.Error("OpenAI key pattern should be redacted")
	}
}

func TestRedact_AnthropicKeyPattern(t *testing.T) {
	r := NewRedactor(nil)

	text := "Using sk-ant-api03-abc123def456ghi789jkl012mnopqrs"
	result, changed := r.Redact(text)
	if !changed {
		t.Fatal("expected pattern redaction")
	}
	if strings.Contains(result, "sk-ant-") {
		t.Error("Anthropic key pattern should be redacted")
	}
}

func TestRedact_BearerToken(t *testing.T) {
	r := NewRedactor(nil)

	text := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig"
	result, changed := r.Redact(text)
	if !changed {
		t.Fatal("expected bearer token redaction")
	}
	if strings.Contains(result, "eyJh") {
		t.Error("bearer token should be redacted")
	}
}

func TestRedact_NoSecrets(t *testing.T) {
	r := NewRedactor([]string{"known-secret"})

	text := "This is a normal prompt with no secrets."
	result, changed := r.Redact(text)
	if changed {
		t.Error("no redaction expected for clean text")
	}
	if result != text {
		t.Error("text should be unchanged")
	}
}

func TestRedact_EmptyText(t *testing.T) {
	r := NewRedactor([]string{"secret"})

	result, changed := r.Redact("")
	if changed {
		t.Error("no redaction for empty text")
	}
	if result != "" {
		t.Error("should return empty")
	}
}

func TestRedact_EmptySecrets(t *testing.T) {
	r := NewRedactor([]string{"", "  ", ""})

	text := "Normal text"
	result, changed := r.Redact(text)
	if changed {
		t.Error("empty secrets should not cause redaction")
	}
	if result != text {
		t.Error("text should be unchanged")
	}
}

func TestRedact_BothLayers(t *testing.T) {
	exactKey := "my-custom-key-value"
	r := NewRedactor([]string{exactKey})

	// Text with both an exact-match key and a pattern-match key.
	text := "Keys: my-custom-key-value and sk-proj-abc123def456ghi789jkl012mno345"
	result, changed := r.Redact(text)
	if !changed {
		t.Fatal("expected redaction")
	}
	if strings.Contains(result, exactKey) {
		t.Error("exact key should be redacted")
	}
	if strings.Contains(result, "sk-proj-") {
		t.Error("pattern key should be redacted")
	}
}

func TestRedactBytes(t *testing.T) {
	r := NewRedactor([]string{"secret-key"})

	data := []byte("payload with secret-key inside")
	result, changed := r.RedactBytes(data)
	if !changed {
		t.Fatal("expected redaction")
	}
	if strings.Contains(string(result), "secret-key") {
		t.Error("secret should be redacted in bytes")
	}
}

func TestQuickRedact(t *testing.T) {
	text := "Key: sk-proj-abc123def456ghi789jkl012mno345"
	result := QuickRedact(text)
	if strings.Contains(result, "sk-proj-") {
		t.Error("pattern should be redacted")
	}
}

func TestContainsSecret_ExactMatch(t *testing.T) {
	r := NewRedactor([]string{"my-secret"})

	if !r.ContainsSecret("text with my-secret here") {
		t.Error("should detect exact secret")
	}
	if r.ContainsSecret("clean text") {
		t.Error("should not detect in clean text")
	}
}

func TestContainsSecret_PatternMatch(t *testing.T) {
	r := NewRedactor(nil)

	if !r.ContainsSecret("has sk-proj-abc123def456ghi789jkl012mno345") {
		t.Error("should detect API key pattern")
	}
	if r.ContainsSecret("normal text with short sk-x") {
		t.Error("short strings should not match")
	}
}

func TestRedact_DoesNotFalsePositive(t *testing.T) {
	r := NewRedactor(nil)

	safe := []string{
		"Hello, world!",
		"GET /api/health",
		"status: 200",
		"short-hex: a1b2c3",
		"request_id: abc123",
	}

	for _, text := range safe {
		_, changed := r.Redact(text)
		if changed {
			t.Errorf("false positive redaction on: %q", text)
		}
	}
}
