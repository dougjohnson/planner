// Package credentials provides API key resolution for flywheel-planner.
//
// The credential service resolves API keys through a tiered lookup:
//  1. Environment variables (highest priority): FLYWHEEL_OPENAI_API_KEY, FLYWHEEL_ANTHROPIC_API_KEY
//  2. Config file (fallback): ~/.flywheel-planner/credentials.json (mode 0600)
//
// Keys are held in memory only and never written to the database, logs, or artifacts.
// The redacting logger (internal/logging) must be active before this service is used (§6.5).
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

const (
	// credentialsFileName is the name of the credentials file under the data directory.
	credentialsFileName = "credentials.json"

	// credentialsFilePerm is the required permission for the credentials file (owner read/write only).
	credentialsFilePerm os.FileMode = 0600
)

// envVarNames maps each provider to its environment variable name.
var envVarNames = map[models.ProviderName]string{
	models.ProviderOpenAI:    "FLYWHEEL_OPENAI_API_KEY",
	models.ProviderAnthropic: "FLYWHEEL_ANTHROPIC_API_KEY",
}

// ErrNoCredentials is returned when no API key is found for a provider.
var ErrNoCredentials = errors.New("no credentials found")

// Service resolves API keys for model providers. It checks environment variables
// first, then falls back to a credentials config file. Keys are held in memory
// only and never persisted to the database or logs.
type Service struct {
	mu       sync.RWMutex
	dataDir  string
	envCache map[models.ProviderName]string // cached env var lookups
}

// NewService creates a credential service that will look for a credentials.json
// file under the given data directory. Pass "" if no file-based fallback is desired.
func NewService(dataDir string) *Service {
	return &Service{
		dataDir:  dataDir,
		envCache: make(map[models.ProviderName]string),
	}
}

// Get returns the API key for the given provider. It checks:
//  1. Environment variables (highest priority)
//  2. Credentials config file at <dataDir>/credentials.json
//
// Returns ErrNoCredentials if no key is found through any tier.
func (s *Service) Get(provider models.ProviderName) (string, error) {
	// Tier 1: Environment variables.
	if key := s.getFromEnv(provider); key != "" {
		return key, nil
	}

	// Tier 2: Config file fallback.
	if s.dataDir != "" {
		key, err := s.getFromFile(provider)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("reading credentials file: %w", err)
		}
		if key != "" {
			return key, nil
		}
	}

	return "", fmt.Errorf("%w for provider %s", ErrNoCredentials, provider)
}

// Has reports whether a credential exists for the given provider without
// returning the sensitive value. Useful for health checks and status display.
func (s *Service) Has(provider models.ProviderName) bool {
	key, err := s.Get(provider)
	return err == nil && key != ""
}

// Providers returns the list of all known provider names.
func (s *Service) Providers() []models.ProviderName {
	return []models.ProviderName{
		models.ProviderOpenAI,
		models.ProviderAnthropic,
	}
}

// EnvVarName returns the environment variable name for a given provider.
// Returns "" if the provider is not recognized.
func EnvVarName(provider models.ProviderName) string {
	return envVarNames[provider]
}

// getFromEnv checks the environment variable for the given provider.
func (s *Service) getFromEnv(provider models.ProviderName) string {
	envVar, ok := envVarNames[provider]
	if !ok {
		return ""
	}

	s.mu.RLock()
	cached, hasCached := s.envCache[provider]
	s.mu.RUnlock()
	if hasCached {
		return cached
	}

	val := strings.TrimSpace(os.Getenv(envVar))
	if val != "" {
		s.mu.Lock()
		s.envCache[provider] = val
		s.mu.Unlock()
	}
	return val
}

// fileCredentials is the schema for credentials.json.
type fileCredentials struct {
	OpenAI    string `json:"openai_api_key"`
	Anthropic string `json:"anthropic_api_key"`
}

// getFromFile reads the credentials file and extracts the key for the given provider.
func (s *Service) getFromFile(provider models.ProviderName) (string, error) {
	path := filepath.Join(s.dataDir, credentialsFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Validate file permissions.
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat credentials file: %w", err)
	}
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		return "", fmt.Errorf("credentials file %s has unsafe permissions %o (must be %o)", path, perm, credentialsFilePerm)
	}

	var creds fileCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", fmt.Errorf("parsing credentials file: %w", err)
	}

	switch provider {
	case models.ProviderOpenAI:
		return strings.TrimSpace(creds.OpenAI), nil
	case models.ProviderAnthropic:
		return strings.TrimSpace(creds.Anthropic), nil
	default:
		return "", nil
	}
}

// Set writes an API key for the given provider to the credentials config file.
// The file is created with 0600 permissions if it doesn't exist.
// Existing keys for other providers are preserved.
func (s *Service) Set(provider models.ProviderName, key string) error {
	if s.dataDir == "" {
		return fmt.Errorf("no data directory configured for credential storage")
	}
	if err := ValidateKeyFormat(provider, key); err != nil {
		return err
	}

	path := filepath.Join(s.dataDir, credentialsFileName)

	// Read existing credentials if file exists.
	var creds fileCredentials
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &creds) // ignore parse errors, overwrite
	}

	// Update the key for this provider.
	switch provider {
	case models.ProviderOpenAI:
		creds.OpenAI = key
	case models.ProviderAnthropic:
		creds.Anthropic = key
	default:
		return fmt.Errorf("unsupported provider %q for credential storage", provider)
	}

	// Marshal and write atomically.
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}
	data = append(data, '\n')

	// Write to temp file then rename for atomicity.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, credentialsFilePerm); err != nil {
		return fmt.Errorf("writing credentials temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming credentials file: %w", err)
	}

	return nil
}

// Delete removes the API key for the given provider from the credentials config file.
func (s *Service) Delete(provider models.ProviderName) error {
	return s.Set(provider, "")
}

// ValidateKeyFormat performs basic sanity checks on an API key format.
// This is not cryptographic validation — just catches obviously wrong inputs.
func ValidateKeyFormat(provider models.ProviderName, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil // empty key means delete
	}
	if len(key) < 10 {
		return fmt.Errorf("API key for %s is too short (minimum 10 characters)", provider)
	}
	if strings.ContainsAny(key, " \t\n\r") {
		return fmt.Errorf("API key for %s contains whitespace", provider)
	}

	switch provider {
	case models.ProviderOpenAI:
		if !strings.HasPrefix(key, "sk-") {
			return fmt.Errorf("OpenAI API key should start with 'sk-'")
		}
	case models.ProviderAnthropic:
		if !strings.HasPrefix(key, "sk-ant-") {
			return fmt.Errorf("Anthropic API key should start with 'sk-ant-'")
		}
	}
	return nil
}

// ClearCache clears any cached environment variable lookups. Useful for testing
// when env vars change mid-process.
func (s *Service) ClearCache() {
	s.mu.Lock()
	s.envCache = make(map[models.ProviderName]string)
	s.mu.Unlock()
}
