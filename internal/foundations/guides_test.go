package foundations

import (
	"testing"
)

func TestListBuiltInGuides(t *testing.T) {
	guides, err := ListBuiltInGuides()
	if err != nil {
		t.Fatalf("ListBuiltInGuides: %v", err)
	}

	if len(guides) < 2 {
		t.Fatalf("expected at least 2 built-in guides, got %d", len(guides))
	}

	techMap := map[string]bool{}
	for _, g := range guides {
		techMap[g.Technology] = true
		if len(g.Content) == 0 {
			t.Errorf("guide %q has empty content", g.Name)
		}
		if g.Filename == "" {
			t.Errorf("guide %q has empty filename", g.Name)
		}
	}

	if !techMap["Go"] {
		t.Error("missing Go built-in guide")
	}
	if !techMap["React"] {
		t.Error("missing React built-in guide")
	}
}

func TestGuidesForStack_KnownStack(t *testing.T) {
	guides, err := GuidesForStack([]string{"Go", "React"})
	if err != nil {
		t.Fatalf("GuidesForStack: %v", err)
	}

	if len(guides) != 2 {
		t.Errorf("expected 2 guides for Go+React, got %d", len(guides))
	}
}

func TestGuidesForStack_UnknownStack(t *testing.T) {
	guides, err := GuidesForStack([]string{"Rust", "Elixir"})
	if err != nil {
		t.Fatalf("GuidesForStack: %v", err)
	}

	if len(guides) != 0 {
		t.Errorf("expected 0 guides for unknown stacks, got %d", len(guides))
	}
}

func TestGuidesForStack_CaseInsensitive(t *testing.T) {
	guides, err := GuidesForStack([]string{"go", "REACT"})
	if err != nil {
		t.Fatalf("GuidesForStack: %v", err)
	}

	if len(guides) != 2 {
		t.Errorf("expected 2 guides (case-insensitive), got %d", len(guides))
	}
}

func TestValidateCustomGuide_Valid(t *testing.T) {
	err := ValidateCustomGuide("CUSTOM.md", []byte("# Custom Guide"), 1024*1024)
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateCustomGuide_Empty(t *testing.T) {
	err := ValidateCustomGuide("EMPTY.md", nil, 1024)
	if err == nil {
		t.Error("expected error for empty guide")
	}
}

func TestValidateCustomGuide_TooLarge(t *testing.T) {
	err := ValidateCustomGuide("BIG.md", make([]byte, 1025), 1024)
	if err == nil {
		t.Error("expected error for oversized guide")
	}
}

func TestValidateCustomGuide_WrongExtension(t *testing.T) {
	err := ValidateCustomGuide("guide.txt", []byte("content"), 1024)
	if err == nil {
		t.Error("expected error for non-.md extension")
	}
}
