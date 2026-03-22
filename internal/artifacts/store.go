// Package artifacts provides filesystem storage for file-backed artifacts
// in the flywheel-planner application.
package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Known subdirectories under each project root.
var projectSubdirs = []string{
	"inputs",
	"foundations",
	"raw",
	"prompts",
	"exports",
	"manifests",
}

// Validation defaults.
const (
	DefaultMaxUploadBytes = 5 * 1024 * 1024 // 5 MB
	dirPerm               = 0700
	filePerm              = 0600
)

// Allowed MIME types for markdown uploads.
var allowedMIMETypes = map[string]bool{
	"text/markdown": true,
	"text/plain":    true,
}

// Errors returned by the store.
var (
	ErrPathTraversal  = errors.New("path traversal detected")
	ErrSymlinkEscape  = errors.New("symlink escapes managed directory")
	ErrInvalidExt     = errors.New("invalid file extension")
	ErrInvalidMIME    = errors.New("invalid MIME type")
	ErrFileTooLarge   = errors.New("file exceeds maximum size")
	ErrEmptyContent   = errors.New("empty content")
	ErrChecksumFailed = errors.New("checksum verification failed")
)

// Store manages file-backed artifacts under a data root.
type Store struct {
	dataDir       string // e.g. ~/.flywheel-planner
	maxUploadSize int64
	logger        *slog.Logger
}

// NewStore creates a new artifact Store rooted at dataDir.
func NewStore(dataDir string, logger *slog.Logger) *Store {
	return &Store{
		dataDir:       dataDir,
		maxUploadSize: DefaultMaxUploadBytes,
		logger:        logger,
	}
}

// SetMaxUploadSize overrides the default upload size limit.
func (s *Store) SetMaxUploadSize(n int64) {
	s.maxUploadSize = n
}

// EnsureProjectDir creates the project directory and all required subdirectories.
// The slug should be sanitized before calling this.
func (s *Store) EnsureProjectDir(projectSlug string) (string, error) {
	projectDir := filepath.Join(s.dataDir, "projects", projectSlug)
	for _, sub := range projectSubdirs {
		subDir := filepath.Join(projectDir, sub)
		if err := os.MkdirAll(subDir, dirPerm); err != nil {
			return "", fmt.Errorf("creating %s: %w", subDir, err)
		}
	}
	s.logger.Debug("project directory ensured", "path", projectDir)
	return projectDir, nil
}

// WriteFile atomically writes data to the given path under the managed data directory.
// It uses a temp file + rename strategy. Returns the SHA-256 checksum of the written data.
func (s *Store) WriteFile(relPath string, data []byte) (checksum string, err error) {
	absPath, err := s.safePath(relPath)
	if err != nil {
		return "", err
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(absPath), dirPerm); err != nil {
		return "", fmt.Errorf("creating parent dir: %w", err)
	}

	// Compute checksum.
	hash := sha256.Sum256(data)
	checksum = hex.EncodeToString(hash[:])

	// Atomic write via temp file + rename.
	tmpFile, err := os.CreateTemp(filepath.Dir(absPath), ".tmp-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, filePerm); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("setting file permissions: %w", err)
	}
	if err := os.Rename(tmpPath, absPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("renaming temp file: %w", err)
	}

	s.logger.Debug("file written", "path", absPath, "checksum", checksum, "size", len(data))
	return checksum, nil
}

// ReadFile reads the file at the given relative path and verifies its checksum.
// If expectedChecksum is empty, checksum verification is skipped.
func (s *Store) ReadFile(relPath string, expectedChecksum string) ([]byte, error) {
	absPath, err := s.safePath(relPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	if expectedChecksum != "" {
		hash := sha256.Sum256(data)
		actual := hex.EncodeToString(hash[:])
		if actual != expectedChecksum {
			return nil, fmt.Errorf("%w: expected %s, got %s", ErrChecksumFailed, expectedChecksum, actual)
		}
	}

	return data, nil
}

// ValidateUpload checks that the upload meets all requirements before storage.
// Returns nil if valid.
func (s *Store) ValidateUpload(filename string, mimeType string, size int64) error {
	// Check empty content.
	if size <= 0 {
		return ErrEmptyContent
	}

	// Check size limit.
	if size > s.maxUploadSize {
		return fmt.Errorf("%w: %d bytes exceeds limit of %d", ErrFileTooLarge, size, s.maxUploadSize)
	}

	// Check file extension.
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".md" {
		return fmt.Errorf("%w: expected .md, got %q", ErrInvalidExt, ext)
	}

	// Check MIME type.
	// Strip parameters like charset from the MIME type.
	baseMIME := strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	if !allowedMIMETypes[strings.ToLower(baseMIME)] {
		return fmt.Errorf("%w: %q not in allowed types", ErrInvalidMIME, mimeType)
	}

	return nil
}

// safePath resolves a relative path to an absolute path under the data directory,
// checking for path traversal and symlink escape.
func (s *Store) safePath(relPath string) (string, error) {
	// Reject absolute paths and explicit traversal.
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%w: absolute path not allowed", ErrPathTraversal)
	}
	cleaned := filepath.Clean(relPath)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("%w: '..' component not allowed", ErrPathTraversal)
	}

	absPath := filepath.Join(s.dataDir, cleaned)

	// Verify the resolved path is still under the data directory.
	// For existing files, resolve symlinks and verify.
	if _, err := os.Lstat(absPath); err == nil {
		resolved, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", fmt.Errorf("resolving symlinks: %w", err)
		}
		resolvedDataDir, err := filepath.EvalSymlinks(s.dataDir)
		if err != nil {
			return "", fmt.Errorf("resolving data dir symlinks: %w", err)
		}
		if !strings.HasPrefix(resolved, resolvedDataDir+string(filepath.Separator)) && resolved != resolvedDataDir {
			return "", fmt.Errorf("%w: resolved path %s escapes %s", ErrSymlinkEscape, resolved, resolvedDataDir)
		}
	}

	return absPath, nil
}

// Checksum computes the SHA-256 checksum of the given data.
func Checksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ChecksumReader computes the SHA-256 checksum from a reader.
func ChecksumReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
