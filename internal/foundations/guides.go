package foundations

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

//go:embed builtin
var builtinFS embed.FS

// BuiltInGuide represents a built-in best-practice guide for a known technology.
type BuiltInGuide struct {
	Technology string // e.g., "Go", "React"
	Name       string // e.g., "Go Best Practices"
	Filename   string // e.g., "BEST_PRACTICE_GO.md"
	Content    []byte
}

// ListBuiltInGuides returns all available built-in guides.
func ListBuiltInGuides() ([]BuiltInGuide, error) {
	var guides []BuiltInGuide

	entries, err := fs.ReadDir(builtinFS, "builtin")
	if err != nil {
		return nil, fmt.Errorf("reading builtin guides: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := builtinFS.ReadFile(filepath.Join("builtin", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading guide %s: %w", entry.Name(), err)
		}

		tech := guessGuideIechnology(entry.Name())
		name := guideDisplayName(entry.Name())

		guides = append(guides, BuiltInGuide{
			Technology: tech,
			Name:       name,
			Filename:   entry.Name(),
			Content:    content,
		})
	}

	return guides, nil
}

// GuidesForStack returns built-in guides that match the given tech stack.
func GuidesForStack(stack []string) ([]BuiltInGuide, error) {
	all, err := ListBuiltInGuides()
	if err != nil {
		return nil, err
	}

	stackSet := make(map[string]bool)
	for _, s := range stack {
		stackSet[strings.ToLower(strings.TrimSpace(s))] = true
	}

	var matched []BuiltInGuide
	for _, g := range all {
		if stackSet[strings.ToLower(g.Technology)] {
			matched = append(matched, g)
		}
	}
	return matched, nil
}

// ValidateCustomGuide performs basic validation on an uploaded custom guide.
func ValidateCustomGuide(filename string, content []byte, maxSize int64) error {
	if len(content) == 0 {
		return fmt.Errorf("guide %q is empty", filename)
	}
	if int64(len(content)) > maxSize {
		return fmt.Errorf("guide %q exceeds maximum size of %d bytes", filename, maxSize)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".md" {
		return fmt.Errorf("guide %q must have .md extension, got %q", filename, ext)
	}
	return nil
}

// guessGuideIechnology extracts the technology name from a guide filename.
func guessGuideIechnology(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	name = strings.TrimPrefix(name, "BEST_PRACTICE_")
	name = strings.TrimPrefix(name, "best_practice_")
	switch strings.ToLower(name) {
	case "go", "golang":
		return "Go"
	case "react":
		return "React"
	case "typescript", "ts":
		return "TypeScript"
	case "python":
		return "Python"
	default:
		return name
	}
}

// guideDisplayName creates a human-readable name from a guide filename.
func guideDisplayName(filename string) string {
	tech := guessGuideIechnology(filename)
	return tech + " Best Practices"
}
