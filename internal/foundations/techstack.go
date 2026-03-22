package foundations

import (
	"fmt"
	"strings"
	"text/template"
)

// TechStackInput holds the user's technology choices.
type TechStackInput struct {
	ProjectName string
	Languages   []string // e.g., ["Go 1.25+", "TypeScript 5.x"]
	Frameworks  []string // e.g., ["React 19", "chi/v5"]
	Database    string   // e.g., "SQLite (modernc.org/sqlite)"
	BuildTools  []string // e.g., ["Vite", "Go Modules"]
	Testing     []string // e.g., ["go test", "Vitest", "Testing Library"]
	Other       []string // e.g., ["goldmark", "TanStack Query"]
}

const techStackTemplate = `# Tech Stack — {{.ProjectName}}

## Languages
{{- range .Languages}}
- {{.}}
{{- end}}
{{- if eq (len .Languages) 0}}
- (not specified)
{{- end}}

## Frameworks & Libraries
{{- range .Frameworks}}
- {{.}}
{{- end}}
{{- if eq (len .Frameworks) 0}}
- (not specified)
{{- end}}

## Database
{{- if .Database}}
- {{.Database}}
{{- else}}
- (not specified)
{{- end}}

## Build Tools
{{- range .BuildTools}}
- {{.}}
{{- end}}
{{- if eq (len .BuildTools) 0}}
- (not specified)
{{- end}}

## Testing
{{- range .Testing}}
- {{.}}
{{- end}}
{{- if eq (len .Testing) 0}}
- (not specified)
{{- end}}
{{- if gt (len .Other) 0}}

## Other Dependencies
{{- range .Other}}
- {{.}}
{{- end}}
{{- end}}
`

// GenerateTechStack renders TECH_STACK.md from the user's technology choices.
func GenerateTechStack(input TechStackInput) (string, error) {
	tmpl, err := template.New("techstack").Parse(techStackTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing tech stack template: %w", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("executing tech stack template: %w", err)
	}
	return buf.String(), nil
}

// ArchitectureInput holds the user's architectural direction.
type ArchitectureInput struct {
	ProjectName string
	Pattern     string   // e.g., "Monolith", "Microservices", "Modular monolith"
	Principles  []string // e.g., ["Local-first", "Single-user", "Fragment-based storage"]
	Constraints []string // e.g., ["Must run as single binary", "Loopback-only binding"]
	Notes       string   // Free-form architectural notes
}

const architectureTemplate = `# Architecture Direction — {{.ProjectName}}

## Architectural Pattern

{{- if .Pattern}}
{{.Pattern}}
{{- else}}
(not specified)
{{- end}}

## Key Principles
{{- range .Principles}}
- {{.}}
{{- end}}
{{- if eq (len .Principles) 0}}
- (not specified)
{{- end}}

## Constraints
{{- range .Constraints}}
- {{.}}
{{- end}}
{{- if eq (len .Constraints) 0}}
- (none specified)
{{- end}}
{{- if .Notes}}

## Additional Notes

{{.Notes}}
{{- end}}
`

// GenerateArchitecture renders ARCHITECTURE.md from the user's architectural direction.
func GenerateArchitecture(input ArchitectureInput) (string, error) {
	tmpl, err := template.New("architecture").Parse(architectureTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing architecture template: %w", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("executing architecture template: %w", err)
	}
	return buf.String(), nil
}
