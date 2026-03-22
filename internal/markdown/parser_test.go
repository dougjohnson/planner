package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecompose_BasicDocument(t *testing.T) {
	src := []byte(`## Introduction

This is the introduction.

## Architecture

This describes architecture.

## Conclusion

Final thoughts.
`)
	sections := Decompose(src)
	require.Len(t, sections, 3)

	assert.Equal(t, "Introduction", sections[0].Heading)
	assert.Equal(t, 2, sections[0].Depth)
	assert.Equal(t, 0, sections[0].Position)
	assert.Contains(t, sections[0].Content, "This is the introduction.")

	assert.Equal(t, "Architecture", sections[1].Heading)
	assert.Equal(t, 1, sections[1].Position)

	assert.Equal(t, "Conclusion", sections[2].Heading)
	assert.Equal(t, 2, sections[2].Position)
	assert.Contains(t, sections[2].Content, "Final thoughts.")
}

func TestDecompose_WithPreamble(t *testing.T) {
	src := []byte(`# Top-Level Title

Some preamble content here.

## First Section

Section content.

## Second Section

More content.
`)
	sections := Decompose(src)
	require.Len(t, sections, 3)

	// Preamble
	assert.Equal(t, "", sections[0].Heading)
	assert.Equal(t, 0, sections[0].Depth)
	assert.Equal(t, 0, sections[0].Position)
	assert.Contains(t, sections[0].Content, "Top-Level Title")
	assert.Contains(t, sections[0].Content, "Some preamble content here.")

	// First section
	assert.Equal(t, "First Section", sections[1].Heading)
	assert.Equal(t, 1, sections[1].Position)

	// Second section
	assert.Equal(t, "Second Section", sections[2].Heading)
	assert.Equal(t, 2, sections[2].Position)
}

func TestDecompose_NoHeadings(t *testing.T) {
	src := []byte(`Just some plain text.

With multiple paragraphs.

No headings at all.
`)
	sections := Decompose(src)
	require.Len(t, sections, 1)
	assert.Equal(t, "", sections[0].Heading)
	assert.Equal(t, 0, sections[0].Depth)
	assert.Contains(t, sections[0].Content, "Just some plain text.")
	assert.Contains(t, sections[0].Content, "No headings at all.")
}

func TestDecompose_EmptyDocument(t *testing.T) {
	sections := Decompose([]byte(""))
	require.Len(t, sections, 1)
	assert.Equal(t, "", sections[0].Heading)
	assert.Equal(t, "", sections[0].Content)
}

func TestDecompose_SubHeadingsStayInSection(t *testing.T) {
	src := []byte(`## Main Section

Intro text.

### Sub-heading A

Sub content A.

### Sub-heading B

Sub content B.

## Another Section

Different content.
`)
	sections := Decompose(src)
	require.Len(t, sections, 2)

	assert.Equal(t, "Main Section", sections[0].Heading)
	assert.Contains(t, sections[0].Content, "Sub-heading A")
	assert.Contains(t, sections[0].Content, "Sub content A.")
	assert.Contains(t, sections[0].Content, "Sub-heading B")
	assert.Contains(t, sections[0].Content, "Sub content B.")

	assert.Equal(t, "Another Section", sections[1].Heading)
	assert.Contains(t, sections[1].Content, "Different content.")
}

func TestDecompose_HeadingsInCodeBlocks(t *testing.T) {
	src := []byte("## Real Heading\n\nSome content.\n\n```markdown\n## Fake Heading In Code Block\n\nThis should not split.\n```\n\nMore content after code block.\n\n## Next Real Heading\n\nFinal content.\n")

	sections := Decompose(src)
	require.Len(t, sections, 2)

	assert.Equal(t, "Real Heading", sections[0].Heading)
	assert.Contains(t, sections[0].Content, "Fake Heading In Code Block")
	assert.Contains(t, sections[0].Content, "More content after code block.")

	assert.Equal(t, "Next Real Heading", sections[1].Heading)
	assert.Contains(t, sections[1].Content, "Final content.")
}

func TestDecompose_PositionsAreSequential(t *testing.T) {
	src := []byte(`Preamble

## A

## B

## C

## D
`)
	sections := Decompose(src)
	for i, s := range sections {
		assert.Equal(t, i, s.Position, "section %d has wrong position", i)
	}
}

func TestDecompose_SingleHeading(t *testing.T) {
	src := []byte(`## Only Section

All the content lives here.
With multiple lines.
`)
	sections := Decompose(src)
	require.Len(t, sections, 1)
	assert.Equal(t, "Only Section", sections[0].Heading)
	assert.Equal(t, 2, sections[0].Depth)
	assert.Contains(t, sections[0].Content, "All the content lives here.")
}

func TestDecompose_DuplicateHeadings(t *testing.T) {
	src := []byte(`## Overview

First overview.

## Details

Some details.

## Overview

Second overview section.
`)
	sections := Decompose(src)
	require.Len(t, sections, 3)
	assert.Equal(t, "Overview", sections[0].Heading)
	assert.Equal(t, "Details", sections[1].Heading)
	assert.Equal(t, "Overview", sections[2].Heading)
	assert.Contains(t, sections[0].Content, "First overview.")
	assert.Contains(t, sections[2].Content, "Second overview section.")
}

func TestDecompose_Level1HeadingsArePreable(t *testing.T) {
	// Level-1 headings (#) should be included in preamble or section content,
	// not used as split points.
	src := []byte(`# Title

Intro.

## Section One

Content.
`)
	sections := Decompose(src)
	require.Len(t, sections, 2)
	assert.Equal(t, "", sections[0].Heading)     // preamble with # Title
	assert.Equal(t, "Section One", sections[1].Heading)
}

func TestDecompose_ConsecutiveHeadings(t *testing.T) {
	src := []byte(`## First
## Second
## Third
`)
	sections := Decompose(src)
	require.Len(t, sections, 3)
	assert.Equal(t, "First", sections[0].Heading)
	assert.Equal(t, "Second", sections[1].Heading)
	assert.Equal(t, "Third", sections[2].Heading)
}
