package foundations

import (
	"strings"
	"testing"
)

func TestAssembleAgentsMD_BasicProject(t *testing.T) {
	input := FoundationsInput{
		ProjectName: "my-app",
		Description: "A web application for task management.",
		TechStack:   []string{"Go", "React", "TypeScript"},
		BuiltInGuides: []GuideReference{
			{Name: "Go Best Practices", Filename: "BEST_PRACTICE_GO.md", Source: "built_in"},
			{Name: "React Best Practices", Filename: "BEST_PRACTICE_REACT.md", Source: "built_in"},
		},
	}

	result, err := AssembleAgentsMD(input)
	if err != nil {
		t.Fatalf("AssembleAgentsMD: %v", err)
	}

	// Check key sections are present.
	for _, expected := range []string{
		"# AGENTS.md — my-app",
		"A web application for task management.",
		"- Go",
		"- React",
		"- TypeScript",
		"TECH_STACK.md",
		"ARCHITECTURE.md",
		"Go Best Practices",
		"React Best Practices",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("missing expected content: %q", expected)
		}
	}
}

func TestAssembleAgentsMD_NoGuides(t *testing.T) {
	input := FoundationsInput{
		ProjectName: "bare-project",
		Description: "A project without guides.",
		TechStack:   []string{"Rust"},
	}

	result, err := AssembleAgentsMD(input)
	if err != nil {
		t.Fatalf("AssembleAgentsMD: %v", err)
	}

	if !strings.Contains(result, "No guides configured") {
		t.Error("expected warning about no guides")
	}
}

func TestAssembleAgentsMD_CustomGuides(t *testing.T) {
	input := FoundationsInput{
		ProjectName: "custom-app",
		Description: "An app with custom guides.",
		TechStack:   []string{"Python"},
		CustomGuides: []GuideReference{
			{Name: "Python Standards", Filename: "PYTHON_STANDARDS.md", Source: "user_upload"},
		},
	}

	result, err := AssembleAgentsMD(input)
	if err != nil {
		t.Fatalf("AssembleAgentsMD: %v", err)
	}

	if !strings.Contains(result, "Python Standards") {
		t.Error("missing custom guide reference")
	}
	if !strings.Contains(result, "*(custom)*") {
		t.Error("missing custom guide marker")
	}
}

func TestAssembleAgentsMD_MixedGuides(t *testing.T) {
	input := FoundationsInput{
		ProjectName: "mixed-app",
		Description: "An app with both guide types.",
		TechStack:   []string{"Go"},
		BuiltInGuides: []GuideReference{
			{Name: "Go Best Practices", Filename: "BEST_PRACTICE_GO.md", Source: "built_in"},
		},
		CustomGuides: []GuideReference{
			{Name: "Internal Standards", Filename: "INTERNAL.md", Source: "user_upload"},
		},
	}

	result, err := AssembleAgentsMD(input)
	if err != nil {
		t.Fatalf("AssembleAgentsMD: %v", err)
	}

	if !strings.Contains(result, "Go Best Practices") {
		t.Error("missing built-in guide")
	}
	if !strings.Contains(result, "Internal Standards") {
		t.Error("missing custom guide")
	}
}

func TestKnownStackGuides(t *testing.T) {
	guides := KnownStackGuides([]string{"Go", "React", "TypeScript"})
	if len(guides) != 3 {
		t.Fatalf("expected 3 guides, got %d", len(guides))
	}

	names := map[string]bool{}
	for _, g := range guides {
		names[g.Name] = true
		if g.Source != "built_in" {
			t.Errorf("guide %q should be built_in", g.Name)
		}
	}
	if !names["Go Best Practices"] {
		t.Error("missing Go guide")
	}
	if !names["React Best Practices"] {
		t.Error("missing React guide")
	}
}

func TestKnownStackGuides_UnknownStack(t *testing.T) {
	guides := KnownStackGuides([]string{"Rust", "Elixir"})
	if len(guides) != 0 {
		t.Errorf("expected 0 guides for unknown stacks, got %d", len(guides))
	}
}

func TestKnownStackGuides_CaseInsensitive(t *testing.T) {
	guides := KnownStackGuides([]string{"GO", "react"})
	if len(guides) != 2 {
		t.Errorf("expected 2 guides, got %d", len(guides))
	}
}

func TestKnownStackGuides_NoDuplicates(t *testing.T) {
	guides := KnownStackGuides([]string{"Go", "go", "GO"})
	if len(guides) != 1 {
		t.Errorf("expected 1 guide (deduplicated), got %d", len(guides))
	}
}
