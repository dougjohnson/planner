// Package foundations provides template-driven assembly of foundation artifacts
// for the flywheel-planner workflow. Foundation artifacts (AGENTS.md, tech stack,
// architecture direction, guides) are file-backed and human-driven — no model
// calls are made during Stage 1.
package foundations

import (
	"fmt"
	"strings"
	"text/template"
)

// FoundationsInput holds the user-provided data for assembling foundation artifacts.
type FoundationsInput struct {
	ProjectName          string
	Description          string
	TechStack            []string // e.g., ["Go", "React", "TypeScript"]
	ArchitectureDirection string
	CustomGuides         []GuideReference
	BuiltInGuides        []GuideReference
}

// GuideReference points to a best-practice guide file.
type GuideReference struct {
	Name     string // e.g., "Go Best Practices"
	Filename string // e.g., "BEST_PRACTICE_GO.md"
	Source   string // "built_in" or "user_upload"
}

// AllGuides returns built-in and custom guides combined.
func (f *FoundationsInput) AllGuides() []GuideReference {
	all := make([]GuideReference, 0, len(f.BuiltInGuides)+len(f.CustomGuides))
	all = append(all, f.BuiltInGuides...)
	all = append(all, f.CustomGuides...)
	return all
}

// agentsTemplate is the built-in AGENTS.md template that ties foundation files together.
const agentsTemplate = `# AGENTS.md — {{.ProjectName}}

> AI coding agent guidelines for the {{.ProjectName}} project.

## Project Overview

{{.Description}}

## Tech Stack

{{- range .TechStack}}
- {{.}}
{{- end}}

For detailed technology choices and version requirements, see [TECH_STACK.md](TECH_STACK.md).

## Architecture Direction

For architectural decisions, constraints, and design patterns, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Best-Practice Guides

The following guides define coding standards for this project:

{{- range .Guides}}
- [{{.Name}}]({{.Filename}}){{if eq .Source "user_upload"}} *(custom)*{{end}}
{{- end}}
{{- if eq (len (.Guides)) 0}}
- No guides configured. Consider adding stack-specific best-practice guides for better model outputs.
{{- end}}

## Coding Discipline

- Follow all practices defined in the best-practice guides listed above.
- Every change must be accompanied by appropriate tests.
- Run compiler checks and linters after any substantive code changes.
- No backwards-compatibility shims — do things the right way with no tech debt.

## Testing

- Every package includes test files alongside the implementation.
- Tests must cover: happy path, edge cases, and error conditions.
- Use the testing frameworks specified in the tech stack file.
`

// agentsTemplateData is the pre-computed data passed to the template.
type agentsTemplateData struct {
	ProjectName string
	Description string
	TechStack   []string
	Guides      []GuideReference
}

// AssembleAgentsMD renders the AGENTS.md template with the given input.
func AssembleAgentsMD(input FoundationsInput) (string, error) {
	tmpl, err := template.New("agents").Parse(agentsTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing AGENTS.md template: %w", err)
	}

	data := agentsTemplateData{
		ProjectName: input.ProjectName,
		Description: input.Description,
		TechStack:   input.TechStack,
		Guides:      input.AllGuides(),
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing AGENTS.md template: %w", err)
	}

	return buf.String(), nil
}

// KnownStackGuides returns built-in guide references for known technology stacks.
func KnownStackGuides(stack []string) []GuideReference {
	known := map[string]GuideReference{
		"go": {
			Name:     "Go Best Practices",
			Filename: "BEST_PRACTICE_GO.md",
			Source:   "built_in",
		},
		"react": {
			Name:     "React Best Practices",
			Filename: "BEST_PRACTICE_REACT.md",
			Source:   "built_in",
		},
		"typescript": {
			Name:     "TypeScript Best Practices",
			Filename: "BEST_PRACTICE_TYPESCRIPT.md",
			Source:   "built_in",
		},
	}

	var guides []GuideReference
	seen := map[string]bool{}
	for _, tech := range stack {
		key := strings.ToLower(strings.TrimSpace(tech))
		if g, ok := known[key]; ok && !seen[key] {
			guides = append(guides, g)
			seen[key] = true
		}
	}
	return guides
}
