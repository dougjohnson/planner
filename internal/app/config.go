// Package app provides application configuration and bootstrap for flywheel-planner.
package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultDataDirName is the directory name under the user's home.
	DefaultDataDirName = ".flywheel-planner"

	// DefaultListenAddr is the default HTTP server address (loopback only, §15.7).
	DefaultListenAddr = "127.0.0.1:7432"

	// DefaultLogLevel is the default structured logging level.
	DefaultLogLevel = slog.LevelInfo

	// dataDirPerm is the permission for the data directory and subdirectories.
	dataDirPerm = 0700
)

// subdirectories that must exist under the data root.
var requiredSubdirs = []string{
	"projects",
}

// Config holds the runtime configuration for flywheel-planner.
// All fields are resolved and validated by LoadConfig.
type Config struct {
	// DataDir is the absolute, resolved path to the local data root
	// (default: ~/.flywheel-planner).
	DataDir string

	// ListenAddr is the HTTP server listen address (default: 127.0.0.1:7432).
	ListenAddr string

	// LogLevel controls the structured logging threshold.
	LogLevel slog.Level

	// DBPath is the resolved path to the SQLite database file.
	DBPath string
}

// LoadConfig builds a Config from environment variables with sensible defaults.
//
// Supported environment variables:
//
//	FLYWHEEL_DATA_DIR    — override the data root (default: ~/.flywheel-planner)
//	FLYWHEEL_LISTEN_ADDR — override the listen address (default: 127.0.0.1:7432)
//	FLYWHEEL_LOG_LEVEL   — override log level: debug, info, warn, error (default: info)
func LoadConfig() (*Config, error) {
	dataDir, err := resolveDataDir(os.Getenv("FLYWHEEL_DATA_DIR"))
	if err != nil {
		return nil, fmt.Errorf("resolving data directory: %w", err)
	}

	listenAddr := DefaultListenAddr
	if v := os.Getenv("FLYWHEEL_LISTEN_ADDR"); v != "" {
		listenAddr = v
	}

	logLevel := DefaultLogLevel
	if v := os.Getenv("FLYWHEEL_LOG_LEVEL"); v != "" {
		parsed, err := parseLogLevel(v)
		if err != nil {
			return nil, err
		}
		logLevel = parsed
	}

	return &Config{
		DataDir:    dataDir,
		ListenAddr: listenAddr,
		LogLevel:   logLevel,
		DBPath:     filepath.Join(dataDir, "app.db"),
	}, nil
}

// EnsureDataDir creates the data directory and all required subdirectories
// with restricted permissions (0700). It validates that the resolved path
// does not escape via symlinks.
func (c *Config) EnsureDataDir() error {
	// Create the root data directory.
	if err := os.MkdirAll(c.DataDir, dataDirPerm); err != nil {
		return fmt.Errorf("creating data directory %s: %w", c.DataDir, err)
	}

	// Verify the data directory itself is not a symlink (§15.2).
	// We check with Lstat to detect if the final path component is a symlink.
	// Parent path components may be symlinks (e.g., /tmp → /private/tmp on macOS)
	// — that's the OS's business, not ours. We only care that OUR directory
	// isn't a symlink pointing somewhere unexpected.
	info, err := os.Lstat(c.DataDir)
	if err != nil {
		return fmt.Errorf("checking data directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("data directory %s is a symlink — refusing to use a symlinked data root", c.DataDir)
	}

	// Create required subdirectories.
	for _, sub := range requiredSubdirs {
		subPath := filepath.Join(c.DataDir, sub)
		if err := os.MkdirAll(subPath, dataDirPerm); err != nil {
			return fmt.Errorf("creating subdirectory %s: %w", subPath, err)
		}
	}

	return nil
}

// resolveDataDir determines the absolute data root path.
// If override is non-empty, it is cleaned and made absolute.
// Otherwise, ~/.flywheel-planner is used.
func resolveDataDir(override string) (string, error) {
	if override != "" {
		abs, err := filepath.Abs(filepath.Clean(override))
		if err != nil {
			return "", fmt.Errorf("resolving override path %q: %w", override, err)
		}
		return abs, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, DefaultDataDirName), nil
}

// parseLogLevel converts a string log level name to slog.Level.
func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q: valid values are debug, info, warn, error", s)
	}
}
