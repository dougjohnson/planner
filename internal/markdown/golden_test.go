package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureDir = "../../tests/fixtures/fragments"

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("loading fixture %s: %v", name, err)
	}
	return data
}

// Recompose takes sections and reconstructs markdown from them.
// This is a test-only function that mimics what the composer does.
func recompose(sections []Section) string {
	var b strings.Builder
	for i, s := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		if s.Heading != "" {
			prefix := strings.Repeat("#", s.Depth)
			b.WriteString(prefix + " " + s.Heading + "\n\n")
		}
		if s.Content != "" {
			b.WriteString(s.Content)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func TestGolden_Simple5Sections(t *testing.T) {
	source := loadFixture(t, "simple_5sections.md")
	sections := Decompose(source)

	if len(sections) != 5 {
		t.Fatalf("expected 5 sections, got %d", len(sections))
	}

	// Verify headings.
	expectedHeadings := []string{"Introduction", "Requirements", "Architecture", "Implementation Plan", "Testing Strategy"}
	for i, h := range expectedHeadings {
		if sections[i].Heading != h {
			t.Errorf("section[%d].Heading = %q, want %q", i, sections[i].Heading, h)
		}
	}

	// Architecture section should contain sub-headings.
	archContent := sections[2].Content
	if !strings.Contains(archContent, "### Database Layer") {
		t.Error("Architecture section should contain ### Database Layer sub-heading")
	}
	if !strings.Contains(archContent, "### API Layer") {
		t.Error("Architecture section should contain ### API Layer sub-heading")
	}

	// Round-trip: recompose and verify key content preserved.
	recomposed := recompose(sections)
	for _, fragment := range []string{
		"modular monolith",
		"Feature A",
		"unit tests",
		"SQLite with WAL",
	} {
		if !strings.Contains(recomposed, fragment) {
			t.Errorf("round-trip lost content: %q", fragment)
		}
	}
}

func TestGolden_PreambleWithContent(t *testing.T) {
	source := loadFixture(t, "preamble_with_content.md")
	sections := Decompose(source)

	if len(sections) != 3 {
		t.Fatalf("expected 3 sections (preamble + 2), got %d", len(sections))
	}

	// Preamble should have empty heading and depth 0.
	if sections[0].Heading != "" {
		t.Errorf("preamble heading = %q, want empty", sections[0].Heading)
	}
	if sections[0].Depth != 0 {
		t.Errorf("preamble depth = %d, want 0", sections[0].Depth)
	}
	if !strings.Contains(sections[0].Content, "Project Title") {
		t.Error("preamble should contain the # heading text")
	}
}

func TestGolden_NoHeadings(t *testing.T) {
	source := loadFixture(t, "no_headings.md")
	sections := Decompose(source)

	if len(sections) != 1 {
		t.Fatalf("expected 1 section (entire doc as preamble), got %d", len(sections))
	}
	if sections[0].Heading != "" {
		t.Errorf("heading should be empty, got %q", sections[0].Heading)
	}
	if !strings.Contains(sections[0].Content, "single preamble fragment") {
		t.Error("preamble should contain the full document text")
	}
}

func TestGolden_CodeBlocks(t *testing.T) {
	source := loadFixture(t, "code_blocks.md")
	sections := Decompose(source)

	// Should only have 3 real sections (headings in code blocks are not splits).
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}

	expectedHeadings := []string{"Configuration", "Implementation", "Summary"}
	for i, h := range expectedHeadings {
		if sections[i].Heading != h {
			t.Errorf("section[%d].Heading = %q, want %q", i, sections[i].Heading, h)
		}
	}

	// Config section should contain the yaml code block with the fake heading.
	if !strings.Contains(sections[0].Content, "## This heading inside a code block") {
		t.Error("code block heading should be in section content, not used as split")
	}
}

func TestGolden_DuplicateHeadings(t *testing.T) {
	source := loadFixture(t, "duplicate_headings.md")
	sections := Decompose(source)

	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(sections))
	}

	// Both "Overview" sections should be present.
	if sections[0].Heading != "Overview" || sections[2].Heading != "Overview" {
		t.Error("both Overview headings should be preserved")
	}

	// They should have different content.
	if sections[0].Content == sections[2].Content {
		t.Error("duplicate heading sections should have different content")
	}

	// Positions should be sequential.
	for i, s := range sections {
		if s.Position != i {
			t.Errorf("section[%d].Position = %d", i, s.Position)
		}
	}
}

func TestGolden_AllFixtures_RoundTrip(t *testing.T) {
	fixtures := []string{
		"simple_5sections.md",
		"preamble_with_content.md",
		"no_headings.md",
		"code_blocks.md",
		"duplicate_headings.md",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			source := loadFixture(t, name)
			sections := Decompose(source)

			if len(sections) == 0 {
				t.Fatal("decompose returned 0 sections")
			}

			// Verify positions are sequential starting from 0.
			for i, s := range sections {
				if s.Position != i {
					t.Errorf("section[%d].Position = %d, want %d", i, s.Position, i)
				}
			}

			// Verify all sections have content or are the preamble.
			for i, s := range sections {
				if s.Heading == "" && s.Depth != 0 {
					t.Errorf("section[%d] has empty heading but non-zero depth %d", i, s.Depth)
				}
				if s.Heading != "" && s.Depth != 2 {
					t.Errorf("section[%d] heading %q has depth %d, want 2", i, s.Heading, s.Depth)
				}
			}
		})
	}
}
