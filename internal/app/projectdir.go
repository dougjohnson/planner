package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const (
	// maxSlugLen caps the slug portion of the directory name.
	maxSlugLen = 40
)

// projectSubdirs are created inside each project directory.
var projectSubdirs = []string{
	"inputs",
	"foundations",
	"raw",
	"prompts",
	"exports",
	"manifests",
}

// slugRe matches characters allowed in a slug.
var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

// CreateProjectDir initializes the directory tree for a new project under
// the data root's projects/ subdirectory. The directory name is
// <slug>-<id>/ where slug is derived from the project name.
//
// Returns the absolute path to the created project directory.
func CreateProjectDir(dataDir, projectName, projectID string) (string, error) {
	if dataDir == "" {
		return "", fmt.Errorf("data directory is required")
	}
	if projectID == "" {
		return "", fmt.Errorf("project ID is required")
	}

	slug := Slugify(projectName)
	dirName := slug + "-" + projectID
	projectPath := filepath.Join(dataDir, "projects", dirName)

	// Create the project root directory.
	if err := os.MkdirAll(projectPath, dataDirPerm); err != nil {
		return "", fmt.Errorf("creating project directory %s: %w", projectPath, err)
	}

	// Create all required subdirectories.
	for _, sub := range projectSubdirs {
		subPath := filepath.Join(projectPath, sub)
		if err := os.MkdirAll(subPath, dataDirPerm); err != nil {
			return "", fmt.Errorf("creating subdirectory %s: %w", subPath, err)
		}
	}

	return projectPath, nil
}

// Slugify converts a project name to a filesystem-safe slug.
// Rules:
//   - Lowercase all characters
//   - Replace spaces and non-alphanumeric chars with hyphens
//   - Collapse consecutive hyphens
//   - Trim leading/trailing hyphens
//   - Truncate to maxSlugLen characters
//   - If empty after sanitization, use "project"
func Slugify(name string) string {
	// Lowercase.
	s := strings.ToLower(strings.TrimSpace(name))

	// Replace non-ASCII with closest ASCII or drop.
	var b strings.Builder
	for _, r := range s {
		if r <= unicode.MaxASCII {
			b.WriteRune(r)
		}
	}
	s = b.String()

	// Replace non-alphanumeric with hyphens.
	s = slugRe.ReplaceAllString(s, "-")

	// Collapse consecutive hyphens.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Trim leading/trailing hyphens.
	s = strings.Trim(s, "-")

	// Truncate.
	if len(s) > maxSlugLen {
		s = s[:maxSlugLen]
		s = strings.TrimRight(s, "-") // Don't end with hyphen after truncation.
	}

	// Fallback for empty slug.
	if s == "" {
		s = "project"
	}

	return s
}

// ProjectDirPath returns the expected path for a project directory without
// creating it. Useful for lookups.
func ProjectDirPath(dataDir, projectName, projectID string) string {
	slug := Slugify(projectName)
	return filepath.Join(dataDir, "projects", slug+"-"+projectID)
}
