package export

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Manifest contains reproducibility metadata for an export bundle (§12.7).
type Manifest struct {
	// Version of the manifest format.
	FormatVersion string `json:"format_version"`
	// ProjectID is the source project.
	ProjectID string `json:"project_id"`
	// ProjectName for display.
	ProjectName string `json:"project_name"`
	// ExportedAt timestamp.
	ExportedAt string `json:"exported_at"`
	// WorkflowVersion is the workflow definition version used.
	WorkflowDefinitionVersion string `json:"workflow_definition_version"`
	// Artifacts lists canonical artifacts with checksums.
	Artifacts []ManifestArtifact `json:"artifacts"`
	// PromptVersions records which prompt template versions were used.
	PromptVersions []PromptVersionRef `json:"prompt_versions"`
	// Runs summarizes execution history.
	Runs []RunSummary `json:"runs"`
	// LoopCounts records review loop iteration counts.
	LoopCounts map[string]int `json:"loop_counts"`
	// Options records the export configuration.
	Options ManifestOptions `json:"options"`
	// Files lists all files in the bundle.
	Files []ManifestFile `json:"files"`
}

// ManifestArtifact records a canonical artifact.
type ManifestArtifact struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	VersionLabel string `json:"version_label"`
	Checksum     string `json:"checksum"`
	IsCanonical  bool   `json:"is_canonical"`
}

// PromptVersionRef records a prompt template version used.
type PromptVersionRef struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Stage   string `json:"stage"`
}

// RunSummary records key facts about a workflow run.
type RunSummary struct {
	ID             string `json:"id"`
	Stage          string `json:"stage"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	Status         string `json:"status"`
	Attempt        int    `json:"attempt"`
	ContinuityMode string `json:"continuity_mode,omitempty"`
}

// ManifestOptions records which export options were used.
type ManifestOptions struct {
	CanonicalOnly        bool `json:"canonical_only"`
	IncludeIntermediates bool `json:"include_intermediates"`
	IncludeRaw           bool `json:"include_raw"`
}

// ManifestFile records a file in the bundle.
type ManifestFile struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Type     string `json:"type"` // "foundation", "artifact", "raw", "prompt", "manifest"
}

// NewManifest creates a manifest with required fields populated.
func NewManifest(projectID, projectName, workflowVersion string) *Manifest {
	return &Manifest{
		FormatVersion:             "1.0",
		ProjectID:                 projectID,
		ProjectName:               projectName,
		ExportedAt:                time.Now().UTC().Format(time.RFC3339),
		WorkflowDefinitionVersion: workflowVersion,
		LoopCounts:                make(map[string]int),
	}
}

// ToJSON serializes the manifest as indented JSON.
func (m *Manifest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// GenerateREADME produces a bundle README explaining the export contents.
func (m *Manifest) GenerateREADME() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Export: %s\n\n", m.ProjectName))
	b.WriteString(fmt.Sprintf("Exported: %s\n", m.ExportedAt))
	b.WriteString(fmt.Sprintf("Workflow Version: %s\n\n", m.WorkflowDefinitionVersion))

	// Artifacts.
	b.WriteString("## Artifacts\n\n")
	if len(m.Artifacts) == 0 {
		b.WriteString("No artifacts in this export.\n\n")
	} else {
		for _, a := range m.Artifacts {
			canonical := ""
			if a.IsCanonical {
				canonical = " (CANONICAL)"
			}
			b.WriteString(fmt.Sprintf("- **%s** %s%s — checksum: %s\n", a.Type, a.VersionLabel, canonical, a.Checksum[:8]))
		}
		b.WriteString("\n")
	}

	// Options.
	b.WriteString("## Export Options\n\n")
	b.WriteString(fmt.Sprintf("- Canonical only: %v\n", m.Options.CanonicalOnly))
	b.WriteString(fmt.Sprintf("- Include intermediates: %v\n", m.Options.IncludeIntermediates))
	b.WriteString(fmt.Sprintf("- Include raw outputs: %v\n", m.Options.IncludeRaw))
	b.WriteString("\n")

	// Prompt versions.
	if len(m.PromptVersions) > 0 {
		b.WriteString("## Prompt Versions\n\n")
		for _, p := range m.PromptVersions {
			b.WriteString(fmt.Sprintf("- %s v%d (%s)\n", p.Name, p.Version, p.Stage))
		}
		b.WriteString("\n")
	}

	// Loop counts.
	if len(m.LoopCounts) > 0 {
		b.WriteString("## Review Loop Iterations\n\n")
		for loop, count := range m.LoopCounts {
			b.WriteString(fmt.Sprintf("- %s: %d iterations\n", loop, count))
		}
		b.WriteString("\n")
	}

	// File listing.
	if len(m.Files) > 0 {
		b.WriteString("## Files\n\n")
		for _, f := range m.Files {
			b.WriteString(fmt.Sprintf("- `%s` (%s)\n", f.Path, f.Type))
		}
		b.WriteString("\n")
	}

	return b.String()
}
