// Package export provides the export bundle assembly pipeline for
// flywheel-planner. Bundles are zip archives containing composed artifacts,
// foundation files, and optional intermediate/raw outputs.
package export

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"time"
)

// BundleOptions controls what is included in the export.
type BundleOptions struct {
	// CanonicalOnly includes only the final canonical artifacts (default behavior).
	CanonicalOnly bool
	// IncludeIntermediates includes each intermediate version composed from its fragment snapshot.
	IncludeIntermediates bool
	// IncludeRawOutputs includes raw model response payloads.
	IncludeRawOutputs bool
}

// DefaultBundleOptions returns the default export configuration (canonical only).
func DefaultBundleOptions() BundleOptions {
	return BundleOptions{CanonicalOnly: true}
}

// BundleFile represents a single file to include in the export bundle.
type BundleFile struct {
	// Path is the archive-relative path (e.g., "artifacts/prd/prd.v08.final.md").
	Path string
	// Content is the file data.
	Content []byte
}

// BundleManifest records what was included in the export for reproducibility.
type BundleManifest struct {
	ProjectID        string       `json:"project_id"`
	ProjectName      string       `json:"project_name"`
	ExportedAt       string       `json:"exported_at"`
	Options          BundleOptions `json:"options"`
	FileCount        int          `json:"file_count"`
	Files            []string     `json:"files"`
}

// Bundle assembles a zip archive from the given files and writes it to w.
// Returns a manifest describing the bundle contents.
func Bundle(w io.Writer, projectID, projectName string, files []BundleFile, opts BundleOptions) (_ *BundleManifest, err error) {
	zw := zip.NewWriter(w)
	// Close explicitly rather than deferring — zip.Writer.Close writes the
	// central directory, and we must surface that error to the caller.
	defer func() {
		if closeErr := zw.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("finalizing zip: %w", closeErr)
		}
	}()

	manifest := &BundleManifest{
		ProjectID:   projectID,
		ProjectName: projectName,
		ExportedAt:  time.Now().UTC().Format(time.RFC3339),
		Options:     opts,
	}

	for _, f := range files {
		header := &zip.FileHeader{
			Name:     f.Path,
			Method:   zip.Deflate,
			Modified: time.Now().UTC(),
		}

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("creating zip entry %s: %w", f.Path, err)
		}
		if _, err := writer.Write(f.Content); err != nil {
			return nil, fmt.Errorf("writing zip entry %s: %w", f.Path, err)
		}

		manifest.Files = append(manifest.Files, f.Path)
		manifest.FileCount++
	}

	return manifest, nil
}

// FoundationFiles returns BundleFile entries for the foundation artifacts
// (AGENTS.md, tech stack, architecture, guides) from the given directory contents.
func FoundationFiles(agentsMD, techStack, architecture string, guides map[string][]byte) []BundleFile {
	var files []BundleFile

	if agentsMD != "" {
		files = append(files, BundleFile{
			Path:    path.Join("foundations", "AGENTS.md"),
			Content: []byte(agentsMD),
		})
	}
	if techStack != "" {
		files = append(files, BundleFile{
			Path:    path.Join("foundations", "TECH_STACK.md"),
			Content: []byte(techStack),
		})
	}
	if architecture != "" {
		files = append(files, BundleFile{
			Path:    path.Join("foundations", "ARCHITECTURE.md"),
			Content: []byte(architecture),
		})
	}
	for name, content := range guides {
		files = append(files, BundleFile{
			Path:    path.Join("foundations", name),
			Content: content,
		})
	}

	return files
}

// ArtifactFile creates a BundleFile for a composed artifact.
func ArtifactFile(docType, versionLabel, content string) BundleFile {
	dir := "artifacts/" + docType
	filename := fmt.Sprintf("%s.%s.md", docType, versionLabel)
	return BundleFile{
		Path:    path.Join(dir, filename),
		Content: []byte(content),
	}
}

// RawOutputFile creates a BundleFile for a raw model response.
func RawOutputFile(runID string, content []byte) BundleFile {
	return BundleFile{
		Path:    path.Join("raw", runID+".json"),
		Content: content,
	}
}
