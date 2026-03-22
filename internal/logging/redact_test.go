package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// captureLog runs a logging function and returns the JSON output as a map.
func captureLog(t *testing.T, fn func(logger *slog.Logger)) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(NewRedactingHandler(inner))
	fn(logger)

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse log output: %v\nraw: %s", err, buf.String())
	}
	return result
}

func TestRedactsSensitiveFieldNames(t *testing.T) {
	sensitiveKeys := []string{
		"api_key", "apikey", "secret", "token", "password",
		"authorization", "auth", "credential", "credentials",
		"secret_key", "access_key", "private_key",
	}

	for _, key := range sensitiveKeys {
		t.Run(key, func(t *testing.T) {
			result := captureLog(t, func(logger *slog.Logger) {
				logger.Info("test", key, "super-secret-value-123")
			})
			val, ok := result[key].(string)
			if !ok {
				t.Fatalf("expected string for key %q, got %T", key, result[key])
			}
			if val != redactedValue {
				t.Errorf("expected %q for key %q, got %q", redactedValue, key, val)
			}
		})
	}
}

func TestRedactsCaseInsensitive(t *testing.T) {
	cases := []string{"API_KEY", "Api_Key", "TOKEN", "Password", "AUTHORIZATION"}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			result := captureLog(t, func(logger *slog.Logger) {
				logger.Info("test", key, "secret-value")
			})
			val, ok := result[key].(string)
			if !ok {
				t.Fatalf("expected string for key %q, got %T", key, result[key])
			}
			if val != redactedValue {
				t.Errorf("expected %q for key %q, got %q", redactedValue, key, val)
			}
		})
	}
}

func TestPassesThroughNonSensitiveFields(t *testing.T) {
	result := captureLog(t, func(logger *slog.Logger) {
		logger.Info("test",
			"user", "alice",
			"path", "/api/health",
			"status", 200,
		)
	})

	if result["user"] != "alice" {
		t.Errorf("expected user=alice, got %v", result["user"])
	}
	if result["path"] != "/api/health" {
		t.Errorf("expected path=/api/health, got %v", result["path"])
	}
	// JSON numbers are float64.
	if result["status"] != float64(200) {
		t.Errorf("expected status=200, got %v", result["status"])
	}
}

func TestRedactsOpenAIKeyPattern(t *testing.T) {
	result := captureLog(t, func(logger *slog.Logger) {
		logger.Info("provider loaded",
			"provider", "openai",
			"key_preview", "sk-proj-abc123def456ghi789jkl012mno",
		)
	})

	if result["key_preview"] != redactedValue {
		t.Errorf("expected OpenAI key pattern to be redacted, got %q", result["key_preview"])
	}
	if result["provider"] != "openai" {
		t.Errorf("provider field should not be redacted, got %q", result["provider"])
	}
}

func TestRedactsAnthropicKeyPattern(t *testing.T) {
	result := captureLog(t, func(logger *slog.Logger) {
		logger.Info("provider loaded",
			"key_preview", "sk-ant-api03-abc123def456ghi789jkl012mnopqrs",
		)
	})

	if result["key_preview"] != redactedValue {
		t.Errorf("expected Anthropic key pattern to be redacted, got %q", result["key_preview"])
	}
}

func TestRedactsBearerToken(t *testing.T) {
	result := captureLog(t, func(logger *slog.Logger) {
		logger.Info("auth header",
			"header_value", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature",
		)
	})

	if result["header_value"] != redactedValue {
		t.Errorf("expected Bearer token to be redacted, got %q", result["header_value"])
	}
}

func TestRedactsLongHexStrings(t *testing.T) {
	result := captureLog(t, func(logger *slog.Logger) {
		logger.Info("config",
			"hex_key", "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
		)
	})

	if result["hex_key"] != redactedValue {
		t.Errorf("expected long hex string to be redacted, got %q", result["hex_key"])
	}
}

func TestDoesNotRedactShortHexStrings(t *testing.T) {
	result := captureLog(t, func(logger *slog.Logger) {
		logger.Info("request",
			"request_id", "a1b2c3d4",
		)
	})

	if result["request_id"] != "a1b2c3d4" {
		t.Errorf("short hex string should not be redacted, got %q", result["request_id"])
	}
}

func TestRedactsNestedGroupAttributes(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(NewRedactingHandler(inner))

	logger.Info("nested test",
		slog.Group("config",
			slog.String("api_key", "sk-secret-key-value"),
			slog.String("endpoint", "https://api.openai.com"),
		),
	)

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse log output: %v\nraw: %s", err, buf.String())
	}

	group, ok := result["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected config group, got %T: %v", result["config"], result["config"])
	}

	if group["api_key"] != redactedValue {
		t.Errorf("expected nested api_key to be redacted, got %q", group["api_key"])
	}
	if group["endpoint"] != "https://api.openai.com" {
		t.Errorf("expected endpoint to pass through, got %q", group["endpoint"])
	}
}

func TestWithAttrsRedacts(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := NewRedactingHandler(inner)

	// Pre-attach a sensitive attribute via WithAttrs.
	handler2 := handler.WithAttrs([]slog.Attr{
		slog.String("api_key", "should-be-redacted"),
		slog.String("component", "auth"),
	})

	logger := slog.New(handler2)
	logger.Info("with attrs test")

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse log output: %v\nraw: %s", err, buf.String())
	}

	if result["api_key"] != redactedValue {
		t.Errorf("expected api_key from WithAttrs to be redacted, got %q", result["api_key"])
	}
	if result["component"] != "auth" {
		t.Errorf("expected component to pass through, got %q", result["component"])
	}
}

func TestWithGroupPreservesRedaction(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := NewRedactingHandler(inner)

	// Open a group and log with a sensitive field.
	groupHandler := handler.WithGroup("provider")
	logger := slog.New(groupHandler)
	logger.Info("group test", "token", "secret-token-value", "name", "openai")

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse log output: %v\nraw: %s", err, buf.String())
	}

	group, ok := result["provider"].(map[string]any)
	if !ok {
		t.Fatalf("expected provider group, got %T: %v", result["provider"], result["provider"])
	}

	if group["token"] != redactedValue {
		t.Errorf("expected token in group to be redacted, got %q", group["token"])
	}
	if group["name"] != "openai" {
		t.Errorf("expected name in group to pass through, got %q", group["name"])
	}
}

func TestEnabledDelegates(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	handler := NewRedactingHandler(inner)

	if handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Info to be disabled when inner handler level is Warn")
	}
	if !handler.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected Warn to be enabled when inner handler level is Warn")
	}
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Error("expected Error to be enabled when inner handler level is Warn")
	}
}

func TestMessageIsPreserved(t *testing.T) {
	result := captureLog(t, func(logger *slog.Logger) {
		logger.Info("flywheel-planner starting")
	})

	if result["msg"] != "flywheel-planner starting" {
		t.Errorf("expected message to be preserved, got %q", result["msg"])
	}
}

func TestDoesNotRedactNonStringFieldsWithSensitiveLikeNames(t *testing.T) {
	// Ensure integer/bool fields with sensitive names are still redacted.
	result := captureLog(t, func(logger *slog.Logger) {
		logger.Info("test", "token", 42)
	})

	// Even non-string values should be redacted when the key is sensitive.
	if result["token"] != redactedValue {
		t.Errorf("expected token (even with int value) to be redacted, got %v", result["token"])
	}
}

func TestNewLoggerWithHandler(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := NewLoggerWithHandler(inner)

	logger.Info("test", "api_key", "secret123", "user", "alice")

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if result["api_key"] != redactedValue {
		t.Errorf("expected redaction via NewLoggerWithHandler")
	}
	if result["user"] != "alice" {
		t.Errorf("expected user pass-through")
	}
}

func TestIsSensitiveKey(t *testing.T) {
	if !isSensitiveKey("api_key") {
		t.Error("api_key should be sensitive")
	}
	if !isSensitiveKey("API_KEY") {
		t.Error("API_KEY should be sensitive (case-insensitive)")
	}
	if isSensitiveKey("username") {
		t.Error("username should not be sensitive")
	}
	if isSensitiveKey("") {
		t.Error("empty string should not be sensitive")
	}
}

func TestRedactsMultipleSensitiveFieldsInOneRecord(t *testing.T) {
	result := captureLog(t, func(logger *slog.Logger) {
		logger.Info("multi-field test",
			"api_key", "key1",
			"password", "pass1",
			"user", "alice",
			"token", "tok1",
		)
	})

	for _, key := range []string{"api_key", "password", "token"} {
		if result[key] != redactedValue {
			t.Errorf("expected %s to be redacted, got %q", key, result[key])
		}
	}
	if result["user"] != "alice" {
		t.Errorf("expected user to pass through, got %q", result["user"])
	}
}

func TestDoesNotFalsePositiveOnShortStrings(t *testing.T) {
	safeValues := []string{
		"hello",
		"info",
		"GET",
		"/api/health",
		"200 OK",
		"sk-short",         // too short for key pattern
		"not-a-hex-string", // has dashes, not hex
	}

	for _, val := range safeValues {
		t.Run(val, func(t *testing.T) {
			result := captureLog(t, func(logger *slog.Logger) {
				logger.Info("test", "field", val)
			})
			got, _ := result["field"].(string)
			if strings.Contains(got, "REDACTED") {
				t.Errorf("value %q should not be redacted, got %q", val, got)
			}
		})
	}
}
