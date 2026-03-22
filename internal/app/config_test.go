package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear any env overrides.
	t.Setenv("FLYWHEEL_DATA_DIR", "")
	t.Setenv("FLYWHEEL_LISTEN_ADDR", "")
	t.Setenv("FLYWHEEL_LOG_LEVEL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	wantDir := filepath.Join(home, DefaultDataDirName)

	if cfg.DataDir != wantDir {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, wantDir)
	}
	if cfg.ListenAddr != DefaultListenAddr {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, DefaultListenAddr)
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, DefaultLogLevel)
	}
	if cfg.DBPath != filepath.Join(wantDir, "app.db") {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, filepath.Join(wantDir, "app.db"))
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FLYWHEEL_DATA_DIR", tmpDir)
	t.Setenv("FLYWHEEL_LISTEN_ADDR", "127.0.0.1:9999")
	t.Setenv("FLYWHEEL_LOG_LEVEL", "debug")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.DataDir != tmpDir {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, tmpDir)
	}
	if cfg.ListenAddr != "127.0.0.1:9999" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, "127.0.0.1:9999")
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
}

func TestLoadConfig_InvalidLogLevel(t *testing.T) {
	t.Setenv("FLYWHEEL_DATA_DIR", "")
	t.Setenv("FLYWHEEL_LISTEN_ADDR", "")
	t.Setenv("FLYWHEEL_LOG_LEVEL", "invalid")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid log level, got nil")
	}
}

func TestEnsureDataDir_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "flywheel-test")

	cfg := &Config{DataDir: dataDir}
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir() error: %v", err)
	}

	// Verify root exists.
	info, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf("data dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("data dir is not a directory")
	}
	if info.Mode().Perm() != dataDirPerm {
		t.Errorf("data dir permissions = %o, want %o", info.Mode().Perm(), dataDirPerm)
	}

	// Verify projects/ subdirectory exists.
	for _, sub := range requiredSubdirs {
		subPath := filepath.Join(dataDir, sub)
		subInfo, err := os.Stat(subPath)
		if err != nil {
			t.Fatalf("subdirectory %q does not exist: %v", sub, err)
		}
		if !subInfo.IsDir() {
			t.Errorf("subdirectory %q is not a directory", sub)
		}
	}
}

func TestEnsureDataDir_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "flywheel-test")

	cfg := &Config{DataDir: dataDir}

	// Call twice — second call must not fail.
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("first EnsureDataDir() error: %v", err)
	}
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("second EnsureDataDir() error: %v", err)
	}
}

func TestEnsureDataDir_RejectsSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real")
	linkDir := filepath.Join(tmpDir, "link")

	if err := os.MkdirAll(realDir, 0700); err != nil {
		t.Fatalf("creating real dir: %v", err)
	}
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	cfg := &Config{DataDir: linkDir}
	err := cfg.EnsureDataDir()
	if err == nil {
		t.Fatal("expected error for symlinked data directory, got nil")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"  Info  ", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseLogLevel(tt.input)
			if err != nil {
				t.Fatalf("parseLogLevel(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveDataDir_Relative(t *testing.T) {
	// A relative override should be made absolute.
	dir, err := resolveDataDir("relative/path")
	if err != nil {
		t.Fatalf("resolveDataDir() error: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("expected absolute path, got %q", dir)
	}
}
