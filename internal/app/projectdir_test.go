package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateProjectDir(t *testing.T) {
	dataDir := t.TempDir()
	// Create the projects/ parent.
	os.MkdirAll(filepath.Join(dataDir, "projects"), 0700)

	path, err := CreateProjectDir(dataDir, "My Cool Project", "abc123")
	if err != nil {
		t.Fatalf("CreateProjectDir: %v", err)
	}

	// Verify the directory exists.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("project dir not found: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}

	// Verify subdirectories.
	for _, sub := range projectSubdirs {
		subPath := filepath.Join(path, sub)
		if _, err := os.Stat(subPath); err != nil {
			t.Errorf("subdirectory %s not found: %v", sub, err)
		}
	}

	// Verify naming convention.
	expected := filepath.Join(dataDir, "projects", "my-cool-project-abc123")
	if path != expected {
		t.Errorf("expected path %s, got %s", expected, path)
	}
}

func TestCreateProjectDir_EmptyDataDir(t *testing.T) {
	_, err := CreateProjectDir("", "Test", "id1")
	if err == nil {
		t.Fatal("expected error for empty data dir")
	}
}

func TestCreateProjectDir_EmptyProjectID(t *testing.T) {
	_, err := CreateProjectDir(t.TempDir(), "Test", "")
	if err == nil {
		t.Fatal("expected error for empty project ID")
	}
}

func TestCreateProjectDir_Idempotent(t *testing.T) {
	dataDir := t.TempDir()
	os.MkdirAll(filepath.Join(dataDir, "projects"), 0700)

	path1, _ := CreateProjectDir(dataDir, "Test", "id1")
	path2, err := CreateProjectDir(dataDir, "Test", "id1")
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if path1 != path2 {
		t.Errorf("expected same path, got %s and %s", path1, path2)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Project", "my-project"},
		{"Hello World 123", "hello-world-123"},
		{"  spaces  around  ", "spaces-around"},
		{"UPPERCASE", "uppercase"},
		{"special!@#$chars", "special-chars"},
		{"multiple---hyphens", "multiple-hyphens"},
		{"", "project"},
		{"   ", "project"},
		{"!!!!", "project"},
		{"a", "a"},
		{"a-b-c", "a-b-c"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSlugify_LongName(t *testing.T) {
	long := "this-is-a-very-long-project-name-that-should-be-truncated-at-the-limit"
	slug := Slugify(long)
	if len(slug) > maxSlugLen {
		t.Errorf("slug too long: %d chars (max %d)", len(slug), maxSlugLen)
	}
	// Should not end with hyphen.
	if slug[len(slug)-1] == '-' {
		t.Error("slug should not end with hyphen after truncation")
	}
}

func TestProjectDirPath(t *testing.T) {
	path := ProjectDirPath("/data", "My Project", "xyz789")
	expected := filepath.Join("/data", "projects", "my-project-xyz789")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestCreateProjectDir_Permissions(t *testing.T) {
	dataDir := t.TempDir()
	os.MkdirAll(filepath.Join(dataDir, "projects"), 0700)

	path, _ := CreateProjectDir(dataDir, "Secure Project", "sec1")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		t.Errorf("expected restrictive permissions, got %o", perm)
	}
}
