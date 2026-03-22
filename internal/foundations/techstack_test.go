package foundations

import (
	"strings"
	"testing"
)

func TestGenerateTechStack_Full(t *testing.T) {
	input := TechStackInput{
		ProjectName: "flywheel-planner",
		Languages:   []string{"Go 1.25+", "TypeScript 5.x"},
		Frameworks:  []string{"React 19", "chi/v5"},
		Database:    "SQLite (modernc.org/sqlite)",
		BuildTools:  []string{"Vite", "Go Modules"},
		Testing:     []string{"go test", "Vitest"},
		Other:       []string{"goldmark", "TanStack Query"},
	}

	result, err := GenerateTechStack(input)
	if err != nil {
		t.Fatalf("GenerateTechStack: %v", err)
	}

	for _, expected := range []string{
		"# Tech Stack — flywheel-planner",
		"Go 1.25+",
		"React 19",
		"SQLite",
		"Vite",
		"go test",
		"goldmark",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("missing: %q", expected)
		}
	}
}

func TestGenerateTechStack_Minimal(t *testing.T) {
	input := TechStackInput{
		ProjectName: "bare",
	}

	result, err := GenerateTechStack(input)
	if err != nil {
		t.Fatalf("GenerateTechStack: %v", err)
	}

	if !strings.Contains(result, "(not specified)") {
		t.Error("expected '(not specified)' for empty fields")
	}
	// Should NOT have Other Dependencies section
	if strings.Contains(result, "Other Dependencies") {
		t.Error("should not have Other section when empty")
	}
}

func TestGenerateArchitecture_Full(t *testing.T) {
	input := ArchitectureInput{
		ProjectName: "flywheel-planner",
		Pattern:     "Modular monolith with embedded SQLite",
		Principles:  []string{"Local-first", "Single-user", "Fragment-based storage"},
		Constraints: []string{"Single binary deployment", "Loopback-only binding"},
		Notes:       "The application serves a React SPA from the Go binary.",
	}

	result, err := GenerateArchitecture(input)
	if err != nil {
		t.Fatalf("GenerateArchitecture: %v", err)
	}

	for _, expected := range []string{
		"# Architecture Direction — flywheel-planner",
		"Modular monolith",
		"Local-first",
		"Single binary",
		"React SPA from the Go binary",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("missing: %q", expected)
		}
	}
}

func TestGenerateArchitecture_Minimal(t *testing.T) {
	input := ArchitectureInput{
		ProjectName: "bare",
	}

	result, err := GenerateArchitecture(input)
	if err != nil {
		t.Fatalf("GenerateArchitecture: %v", err)
	}

	if !strings.Contains(result, "(not specified)") {
		t.Error("expected '(not specified)' for empty pattern")
	}
	if strings.Contains(result, "Additional Notes") {
		t.Error("should not have notes section when empty")
	}
}
