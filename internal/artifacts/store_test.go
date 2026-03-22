package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestEnsureProjectDir(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, testLogger())

	projectDir, err := store.EnsureProjectDir("test-project-abc123")
	if err != nil {
		t.Fatalf("EnsureProjectDir: %v", err)
	}

	for _, sub := range projectSubdirs {
		subPath := filepath.Join(projectDir, sub)
		info, err := os.Stat(subPath)
		if err != nil {
			t.Errorf("subdir %s not created: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("subdir %s is not a directory", sub)
		}
	}
}

func TestWriteFile_AtomicAndChecksum(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, testLogger())

	data := []byte("hello, world")
	expectedHash := sha256.Sum256(data)
	expectedChecksum := hex.EncodeToString(expectedHash[:])

	checksum, err := store.WriteFile("test/file.txt", data)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if checksum != expectedChecksum {
		t.Errorf("checksum: expected %s, got %s", expectedChecksum, checksum)
	}

	// Verify file exists with correct content.
	got, err := os.ReadFile(filepath.Join(dir, "test", "file.txt"))
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content mismatch: expected %q, got %q", data, got)
	}
}

func TestReadFile_ValidChecksum(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, testLogger())

	data := []byte("test content")
	checksum, err := store.WriteFile("read-test.txt", data)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := store.ReadFile("read-test.txt", checksum)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content mismatch")
	}
}

func TestReadFile_InvalidChecksum(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, testLogger())

	data := []byte("test content")
	_, err := store.WriteFile("checksum-test.txt", data)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = store.ReadFile("checksum-test.txt", "wrong-checksum")
	if !errors.Is(err, ErrChecksumFailed) {
		t.Errorf("expected ErrChecksumFailed, got: %v", err)
	}
}

func TestReadFile_NoChecksumVerification(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, testLogger())

	data := []byte("no verify")
	_, err := store.WriteFile("no-verify.txt", data)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := store.ReadFile("no-verify.txt", "")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content mismatch")
	}
}

func TestWriteFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, testLogger())

	tests := []struct {
		name    string
		relPath string
	}{
		{"dot-dot prefix", "../escape.txt"},
		{"embedded dot-dot", "foo/../../escape.txt"},
		{"absolute path", "/etc/passwd"},
	}

	for _, tc := range tests {
		_, err := store.WriteFile(tc.relPath, []byte("evil"))
		if !errors.Is(err, ErrPathTraversal) {
			t.Errorf("%s: expected ErrPathTraversal, got: %v", tc.name, err)
		}
	}
}

func TestValidateUpload_ValidFile(t *testing.T) {
	store := NewStore(t.TempDir(), testLogger())
	err := store.ValidateUpload("readme.md", "text/markdown", 1024)
	if err != nil {
		t.Errorf("expected valid upload, got: %v", err)
	}
}

func TestValidateUpload_TextPlainMIME(t *testing.T) {
	store := NewStore(t.TempDir(), testLogger())
	err := store.ValidateUpload("doc.md", "text/plain; charset=utf-8", 512)
	if err != nil {
		t.Errorf("expected valid upload with text/plain, got: %v", err)
	}
}

func TestValidateUpload_InvalidExtension(t *testing.T) {
	store := NewStore(t.TempDir(), testLogger())
	err := store.ValidateUpload("readme.txt", "text/plain", 1024)
	if !errors.Is(err, ErrInvalidExt) {
		t.Errorf("expected ErrInvalidExt, got: %v", err)
	}
}

func TestValidateUpload_InvalidMIME(t *testing.T) {
	store := NewStore(t.TempDir(), testLogger())
	err := store.ValidateUpload("doc.md", "application/octet-stream", 1024)
	if !errors.Is(err, ErrInvalidMIME) {
		t.Errorf("expected ErrInvalidMIME, got: %v", err)
	}
}

func TestValidateUpload_TooLarge(t *testing.T) {
	store := NewStore(t.TempDir(), testLogger())
	err := store.ValidateUpload("big.md", "text/markdown", DefaultMaxUploadBytes+1)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("expected ErrFileTooLarge, got: %v", err)
	}
}

func TestValidateUpload_EmptyContent(t *testing.T) {
	store := NewStore(t.TempDir(), testLogger())
	err := store.ValidateUpload("empty.md", "text/markdown", 0)
	if !errors.Is(err, ErrEmptyContent) {
		t.Errorf("expected ErrEmptyContent, got: %v", err)
	}
}

func TestValidateUpload_CustomSizeLimit(t *testing.T) {
	store := NewStore(t.TempDir(), testLogger())
	store.SetMaxUploadSize(100)

	err := store.ValidateUpload("small.md", "text/markdown", 101)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("expected ErrFileTooLarge with custom limit, got: %v", err)
	}

	err = store.ValidateUpload("small.md", "text/markdown", 50)
	if err != nil {
		t.Errorf("expected valid upload under custom limit, got: %v", err)
	}
}

func TestChecksum(t *testing.T) {
	data := []byte("hello")
	expected := sha256.Sum256(data)
	got := Checksum(data)
	if got != hex.EncodeToString(expected[:]) {
		t.Errorf("Checksum mismatch: expected %s, got %s", hex.EncodeToString(expected[:]), got)
	}
}
