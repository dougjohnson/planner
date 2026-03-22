package markdown

import (
	"testing"
)

func TestIsPreamble(t *testing.T) {
	preamble := Section{Heading: "", Depth: 0, Content: "intro text"}
	if !preamble.IsPreamble() {
		t.Error("expected preamble")
	}

	heading := Section{Heading: "Overview", Depth: 2, Content: "body"}
	if heading.IsPreamble() {
		t.Error("heading section should not be preamble")
	}
}

func TestHasPreamble(t *testing.T) {
	with := []Section{
		{Heading: "", Depth: 0},
		{Heading: "Overview", Depth: 2},
	}
	if !HasPreamble(with) {
		t.Error("expected HasPreamble=true")
	}

	without := []Section{
		{Heading: "Overview", Depth: 2},
	}
	if HasPreamble(without) {
		t.Error("expected HasPreamble=false when first section has heading")
	}

	if HasPreamble(nil) {
		t.Error("expected HasPreamble=false for nil")
	}
}

func TestSplitPreamble_WithPreamble(t *testing.T) {
	sections := []Section{
		{Heading: "", Depth: 0, Content: "# Title\n\nIntro"},
		{Heading: "Overview", Depth: 2, Content: "Overview content"},
		{Heading: "Details", Depth: 2, Content: "Detail content"},
	}

	preamble, body := SplitPreamble(sections)
	if preamble == nil {
		t.Fatal("expected preamble")
	}
	if preamble.Content != "# Title\n\nIntro" {
		t.Errorf("unexpected preamble content: %q", preamble.Content)
	}
	if len(body) != 2 {
		t.Errorf("expected 2 body sections, got %d", len(body))
	}
}

func TestSplitPreamble_WithoutPreamble(t *testing.T) {
	sections := []Section{
		{Heading: "Overview", Depth: 2, Content: "body"},
	}

	preamble, body := SplitPreamble(sections)
	if preamble != nil {
		t.Error("expected no preamble")
	}
	if len(body) != 1 {
		t.Errorf("expected 1 body section, got %d", len(body))
	}
}

func TestSplitPreamble_Empty(t *testing.T) {
	preamble, body := SplitPreamble(nil)
	if preamble != nil {
		t.Error("expected nil preamble for nil input")
	}
	if body != nil {
		t.Error("expected nil body for nil input")
	}
}

func TestPreambleDecomposition_Integration(t *testing.T) {
	doc := []byte("# My PRD\n\nThis is the introduction.\n\n## Overview\n\nOverview content here.\n\n## Requirements\n\nReq content.\n")
	sections := Decompose(doc)

	if !HasPreamble(sections) {
		t.Fatal("expected preamble in decomposed document")
	}

	preamble, body := SplitPreamble(sections)
	if preamble == nil {
		t.Fatal("expected preamble section")
	}

	if len(body) != 2 {
		t.Errorf("expected 2 body sections, got %d", len(body))
	}
	if body[0].Heading != "Overview" {
		t.Errorf("expected first body heading 'Overview', got %q", body[0].Heading)
	}
}

func TestNoPreamble_DecompositionIntegration(t *testing.T) {
	doc := []byte("## Overview\n\nContent starts immediately.\n\n## Details\n\nMore content.\n")
	sections := Decompose(doc)

	if HasPreamble(sections) {
		t.Error("expected no preamble when document starts with ##")
	}

	if len(sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(sections))
	}
}
