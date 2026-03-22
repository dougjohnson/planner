package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBootstrapSequence_DataDirCreation verifies the boot sequence creates
// the data directory and all required subdirectories.
func TestBootstrapSequence_DataDirCreation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FLYWHEEL_DATA_DIR", filepath.Join(tmpDir, "flywheel-test"))
	t.Setenv("FLYWHEEL_LISTEN_ADDR", "")
	t.Setenv("FLYWHEEL_LOG_LEVEL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir: %v", err)
	}

	// Verify root + subdirectories exist.
	info, err := os.Stat(cfg.DataDir)
	if err != nil {
		t.Fatalf("data dir missing: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("data dir is not a directory")
	}

	projectsDir := filepath.Join(cfg.DataDir, "projects")
	if _, err := os.Stat(projectsDir); err != nil {
		t.Fatalf("projects/ subdir missing: %v", err)
	}
}

// TestBootstrapSequence_ConfigDefaults verifies default configuration.
func TestBootstrapSequence_ConfigDefaults(t *testing.T) {
	t.Setenv("FLYWHEEL_DATA_DIR", "")
	t.Setenv("FLYWHEEL_LISTEN_ADDR", "")
	t.Setenv("FLYWHEEL_LOG_LEVEL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.ListenAddr != DefaultListenAddr {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, DefaultListenAddr)
	}
	if cfg.DBPath == "" {
		t.Error("DBPath should not be empty")
	}
}

// TestBootstrapSequence_FailsOnBadLogLevel verifies the app fails to start
// with a clear error if the log level is invalid.
func TestBootstrapSequence_FailsOnBadLogLevel(t *testing.T) {
	t.Setenv("FLYWHEEL_LOG_LEVEL", "INVALID")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

// TestBootstrapSequence_DBPathDerived verifies DBPath is correctly derived.
func TestBootstrapSequence_DBPathDerived(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FLYWHEEL_DATA_DIR", tmpDir)
	t.Setenv("FLYWHEEL_LISTEN_ADDR", "")
	t.Setenv("FLYWHEEL_LOG_LEVEL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	expected := filepath.Join(tmpDir, "app.db")
	if cfg.DBPath != expected {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, expected)
	}
}

// TestBootstrapSequence_PermissionsSafe verifies directories have 0700.
func TestBootstrapSequence_PermissionsSafe(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "flywheel-test")

	cfg := &Config{DataDir: dataDir}
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir: %v", err)
	}

	info, err := os.Stat(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("data dir perm = %o, want 0700", perm)
	}
}

// TestBootstrapSequence_IdempotentDataDir verifies calling EnsureDataDir
// multiple times is safe.
func TestBootstrapSequence_IdempotentDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "flywheel-test")

	cfg := &Config{DataDir: dataDir}
	for i := 0; i < 3; i++ {
		if err := cfg.EnsureDataDir(); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
	}
}
