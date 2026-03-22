package foundations

import (
	"strings"
	"testing"
)

// TestIntegration_FullFoundationsWorkflow tests the complete foundations assembly
// pipeline: input → AGENTS.md + TECH_STACK + ARCHITECTURE generation.
func TestIntegration_FullFoundationsWorkflow(t *testing.T) {
	// Step 1: User provides stack and architecture.
	stack := []string{"Go", "React", "TypeScript"}
	archInput := ArchitectureInput{
		ProjectName: "test-project",
		Pattern:     "Modular monolith with embedded SQLite",
		Principles:  []string{"Local-first", "Single-user", "Fragment-based storage"},
		Constraints: []string{"Single binary", "Loopback-only"},
		Notes:       "React SPA embedded in Go binary.",
	}

	// Step 2: Match built-in guides.
	guides, err := GuidesForStack(stack)
	if err != nil {
		t.Fatalf("GuidesForStack: %v", err)
	}
	if len(guides) < 2 {
		t.Fatalf("expected at least 2 guides for Go+React, got %d", len(guides))
	}

	// Step 3: Generate tech stack file.
	techStack, err := GenerateTechStack(TechStackInput{
		ProjectName: "test-project",
		Languages:   []string{"Go 1.25+", "TypeScript 5.x"},
		Frameworks:  []string{"React 19", "chi/v5"},
		Database:    "SQLite (modernc.org/sqlite)",
		BuildTools:  []string{"Vite", "Go Modules"},
		Testing:     []string{"go test + testify", "Vitest + Testing Library"},
	})
	if err != nil {
		t.Fatalf("GenerateTechStack: %v", err)
	}

	// Step 4: Generate architecture file.
	arch, err := GenerateArchitecture(archInput)
	if err != nil {
		t.Fatalf("GenerateArchitecture: %v", err)
	}

	// Step 5: Assemble AGENTS.md.
	guideRefs := make([]GuideReference, len(guides))
	for i, g := range guides {
		guideRefs[i] = GuideReference{
			Name:     g.Name,
			Filename: g.Filename,
			Source:   "built_in",
		}
	}

	agentsMD, err := AssembleAgentsMD(FoundationsInput{
		ProjectName: "test-project",
		Description: "A test project for integration testing.",
		TechStack:   stack,
		BuiltInGuides: guideRefs,
	})
	if err != nil {
		t.Fatalf("AssembleAgentsMD: %v", err)
	}

	// Verify AGENTS.md references tech stack and architecture files.
	if !strings.Contains(agentsMD, "TECH_STACK.md") {
		t.Error("AGENTS.md should reference TECH_STACK.md")
	}
	if !strings.Contains(agentsMD, "ARCHITECTURE.md") {
		t.Error("AGENTS.md should reference ARCHITECTURE.md")
	}
	for _, g := range guides {
		if !strings.Contains(agentsMD, g.Name) {
			t.Errorf("AGENTS.md should reference guide %q", g.Name)
		}
	}

	// Verify tech stack content.
	if !strings.Contains(techStack, "Go 1.25+") {
		t.Error("TECH_STACK.md should contain Go version")
	}
	if !strings.Contains(techStack, "SQLite") {
		t.Error("TECH_STACK.md should contain database")
	}

	// Verify architecture content.
	if !strings.Contains(arch, "Modular monolith") {
		t.Error("ARCHITECTURE.md should contain architecture pattern")
	}
	if !strings.Contains(arch, "Local-first") {
		t.Error("ARCHITECTURE.md should contain principles")
	}
}

// TestIntegration_UnknownStack verifies the warning path for custom stacks.
func TestIntegration_UnknownStack(t *testing.T) {
	stack := []string{"Rust", "Elixir"}
	guides, err := GuidesForStack(stack)
	if err != nil {
		t.Fatalf("GuidesForStack: %v", err)
	}
	if len(guides) != 0 {
		t.Errorf("expected 0 built-in guides for unknown stacks, got %d", len(guides))
	}

	// AGENTS.md should still generate but warn about missing guides.
	agentsMD, err := AssembleAgentsMD(FoundationsInput{
		ProjectName: "rust-project",
		Description: "A project with no built-in guides.",
		TechStack:   stack,
	})
	if err != nil {
		t.Fatalf("AssembleAgentsMD: %v", err)
	}
	if !strings.Contains(agentsMD, "No guides configured") {
		t.Error("AGENTS.md should warn about missing guides")
	}
}

// TestIntegration_CustomGuideValidation verifies custom guide upload validation.
func TestIntegration_CustomGuideValidation(t *testing.T) {
	// Valid guide.
	err := ValidateCustomGuide("CUSTOM.md", []byte("# Custom Guide\n\nContent."), 5*1024*1024)
	if err != nil {
		t.Errorf("valid guide rejected: %v", err)
	}

	// Invalid: wrong extension.
	err = ValidateCustomGuide("guide.txt", []byte("content"), 5*1024*1024)
	if err == nil {
		t.Error("expected error for .txt extension")
	}

	// Invalid: too large.
	err = ValidateCustomGuide("big.md", make([]byte, 6*1024*1024), 5*1024*1024)
	if err == nil {
		t.Error("expected error for oversized guide")
	}

	// Invalid: empty.
	err = ValidateCustomGuide("empty.md", nil, 5*1024*1024)
	if err == nil {
		t.Error("expected error for empty guide")
	}
}

// TestIntegration_MixedGuides tests a project with both built-in and custom guides.
func TestIntegration_MixedGuides(t *testing.T) {
	builtIn, _ := GuidesForStack([]string{"Go"})
	custom := []GuideReference{
		{Name: "Internal Standards", Filename: "INTERNAL.md", Source: "user_upload"},
	}

	agentsMD, err := AssembleAgentsMD(FoundationsInput{
		ProjectName: "mixed-project",
		Description: "A project with both guide types.",
		TechStack:   []string{"Go", "Custom"},
		BuiltInGuides: func() []GuideReference {
			refs := make([]GuideReference, len(builtIn))
			for i, g := range builtIn {
				refs[i] = GuideReference{Name: g.Name, Filename: g.Filename, Source: "built_in"}
			}
			return refs
		}(),
		CustomGuides: custom,
	})
	if err != nil {
		t.Fatalf("AssembleAgentsMD: %v", err)
	}

	if !strings.Contains(agentsMD, "Go Best Practices") {
		t.Error("should contain built-in guide")
	}
	if !strings.Contains(agentsMD, "Internal Standards") {
		t.Error("should contain custom guide")
	}
	if !strings.Contains(agentsMD, "*(custom)*") {
		t.Error("custom guide should have provenance marker")
	}
}
