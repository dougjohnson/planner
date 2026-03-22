package credentials

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

func TestGetFromEnv_OpenAI(t *testing.T) {
	t.Setenv("FLYWHEEL_OPENAI_API_KEY", "sk-test-openai-key")

	svc := NewService("")
	key, err := svc.Get(models.ProviderOpenAI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "sk-test-openai-key" {
		t.Errorf("expected sk-test-openai-key, got %q", key)
	}
}

func TestGetFromEnv_Anthropic(t *testing.T) {
	t.Setenv("FLYWHEEL_ANTHROPIC_API_KEY", "sk-ant-test-key")

	svc := NewService("")
	key, err := svc.Get(models.ProviderAnthropic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "sk-ant-test-key" {
		t.Errorf("expected sk-ant-test-key, got %q", key)
	}
}

func TestGetFromEnv_TrimsWhitespace(t *testing.T) {
	t.Setenv("FLYWHEEL_OPENAI_API_KEY", "  sk-test-key  \n")

	svc := NewService("")
	key, err := svc.Get(models.ProviderOpenAI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "sk-test-key" {
		t.Errorf("expected trimmed key, got %q", key)
	}
}

func TestGetFromEnv_EmptyReturnsError(t *testing.T) {
	t.Setenv("FLYWHEEL_OPENAI_API_KEY", "")

	svc := NewService("")
	_, err := svc.Get(models.ProviderOpenAI)
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials, got %v", err)
	}
}

func TestGetFromEnv_NotSetReturnsError(t *testing.T) {
	// Don't set the env var at all.
	svc := NewService("")
	_, err := svc.Get(models.ProviderOpenAI)
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials, got %v", err)
	}
}

func TestGetFromFile_OpenAI(t *testing.T) {
	dir := t.TempDir()
	creds := fileCredentials{
		OpenAI:    "sk-file-openai-key",
		Anthropic: "sk-ant-file-key",
	}
	writeCredentialsFile(t, dir, creds)

	svc := NewService(dir)
	key, err := svc.Get(models.ProviderOpenAI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "sk-file-openai-key" {
		t.Errorf("expected sk-file-openai-key, got %q", key)
	}
}

func TestGetFromFile_Anthropic(t *testing.T) {
	dir := t.TempDir()
	creds := fileCredentials{
		OpenAI:    "sk-file-openai-key",
		Anthropic: "sk-ant-file-key",
	}
	writeCredentialsFile(t, dir, creds)

	svc := NewService(dir)
	key, err := svc.Get(models.ProviderAnthropic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "sk-ant-file-key" {
		t.Errorf("expected sk-ant-file-key, got %q", key)
	}
}

func TestEnvTakesPriorityOverFile(t *testing.T) {
	dir := t.TempDir()
	creds := fileCredentials{OpenAI: "sk-file-key"}
	writeCredentialsFile(t, dir, creds)

	t.Setenv("FLYWHEEL_OPENAI_API_KEY", "sk-env-key")

	svc := NewService(dir)
	key, err := svc.Get(models.ProviderOpenAI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "sk-env-key" {
		t.Errorf("expected env key to take priority, got %q", key)
	}
}

func TestGetFromFile_UnsafePermissions(t *testing.T) {
	dir := t.TempDir()
	creds := fileCredentials{OpenAI: "sk-test-key"}
	data, err := json.Marshal(creds)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, credentialsFileName)
	// Write with world-readable permissions.
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(dir)
	_, err = svc.Get(models.ProviderOpenAI)
	if err == nil {
		t.Fatal("expected error for unsafe permissions")
	}
	if !containsString(err.Error(), "unsafe permissions") {
		t.Errorf("expected unsafe permissions error, got: %v", err)
	}
}

func TestGetFromFile_MissingFileReturnsNoCredentials(t *testing.T) {
	dir := t.TempDir()
	// No credentials file created.

	svc := NewService(dir)
	_, err := svc.Get(models.ProviderOpenAI)
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials for missing file, got %v", err)
	}
}

func TestGetFromFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialsFileName)
	if err := os.WriteFile(path, []byte("not-json"), credentialsFilePerm); err != nil {
		t.Fatal(err)
	}

	svc := NewService(dir)
	_, err := svc.Get(models.ProviderOpenAI)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHas(t *testing.T) {
	t.Setenv("FLYWHEEL_OPENAI_API_KEY", "sk-test-key")

	svc := NewService("")
	if !svc.Has(models.ProviderOpenAI) {
		t.Error("expected Has to return true for set env var")
	}
	if svc.Has(models.ProviderAnthropic) {
		t.Error("expected Has to return false for unset provider")
	}
}

func TestProviders(t *testing.T) {
	svc := NewService("")
	providers := svc.Providers()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	found := make(map[models.ProviderName]bool)
	for _, p := range providers {
		found[p] = true
	}
	if !found[models.ProviderOpenAI] {
		t.Error("expected ProviderOpenAI in list")
	}
	if !found[models.ProviderAnthropic] {
		t.Error("expected ProviderAnthropic in list")
	}
}

func TestEnvVarName(t *testing.T) {
	if name := EnvVarName(models.ProviderOpenAI); name != "FLYWHEEL_OPENAI_API_KEY" {
		t.Errorf("expected FLYWHEEL_OPENAI_API_KEY, got %q", name)
	}
	if name := EnvVarName(models.ProviderAnthropic); name != "FLYWHEEL_ANTHROPIC_API_KEY" {
		t.Errorf("expected FLYWHEEL_ANTHROPIC_API_KEY, got %q", name)
	}
	if name := EnvVarName("unknown"); name != "" {
		t.Errorf("expected empty for unknown provider, got %q", name)
	}
}

func TestClearCache(t *testing.T) {
	t.Setenv("FLYWHEEL_OPENAI_API_KEY", "sk-original")

	svc := NewService("")
	// Populate cache.
	key, _ := svc.Get(models.ProviderOpenAI)
	if key != "sk-original" {
		t.Fatalf("expected sk-original, got %q", key)
	}

	// Change env var and verify cache still returns old value.
	t.Setenv("FLYWHEEL_OPENAI_API_KEY", "sk-updated")
	key, _ = svc.Get(models.ProviderOpenAI)
	if key != "sk-original" {
		t.Fatalf("expected cached sk-original, got %q", key)
	}

	// Clear cache and verify new value is returned.
	svc.ClearCache()
	key, _ = svc.Get(models.ProviderOpenAI)
	if key != "sk-updated" {
		t.Errorf("expected sk-updated after cache clear, got %q", key)
	}
}

func TestUnknownProviderReturnsError(t *testing.T) {
	svc := NewService("")
	_, err := svc.Get("unknown-provider")
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials for unknown provider, got %v", err)
	}
}

// writeCredentialsFile writes a credentials.json file with proper permissions.
func writeCredentialsFile(t *testing.T, dir string, creds fileCredentials) {
	t.Helper()
	data, err := json.Marshal(creds)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, credentialsFileName)
	if err := os.WriteFile(path, data, credentialsFilePerm); err != nil {
		t.Fatal(err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
