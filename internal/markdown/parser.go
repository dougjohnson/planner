// Package markdown provides heading-level decomposition of markdown documents
// using the goldmark parser. This is the foundation of the fragment system:
// documents are split at ## (level-2) headings into addressable sections.
package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Section represents a contiguous block of markdown content anchored by a
// level-2 heading. The preamble (content before the first ## heading) has
// an empty Heading and Depth of 0.
type Section struct {
	// Heading is the text of the ## heading. Empty for the preamble section.
	Heading string `json:"heading"`
	// Depth is the heading level (2 for ##). 0 for preamble.
	Depth int `json:"depth"`
	// Content is the raw markdown text of this section, excluding the heading line itself.
	Content string `json:"content"`
	// Position is the zero-based index of this section in document order.
	Position int `json:"position"`
}

// Decompose parses a markdown document and splits it into sections at ## (level-2)
// heading boundaries. Sub-headings (###, ####, etc.) within a ## section are
// included in that section's content. Headings inside fenced code blocks are
// correctly ignored by the goldmark AST parser.
//
// Returns at least one section. If the document has no ## headings, a single
// preamble section containing the entire document is returned.
func Decompose(source []byte) []Section {
	md := goldmark.New()
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	// Collect positions of all level-2 headings by walking document's
	// direct children. Level-2 headings are always top-level block children.
	type headingInfo struct {
		heading   string
		nodeStart int // byte offset where the heading line begins in source
	}

	var headings []headingInfo

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		heading, ok := child.(*ast.Heading)
		if !ok || heading.Level != 2 {
			continue
		}

		// Extract heading text from inline children.
		var headingText bytes.Buffer
		for inline := heading.FirstChild(); inline != nil; inline = inline.NextSibling() {
			if t, ok := inline.(*ast.Text); ok {
				headingText.Write(t.Segment.Value(source))
			}
		}

		// Find the byte offset of this heading in the source.
		// For ATX headings, the first child text segment tells us where the
		// heading text starts. We scan backwards to find the "## " prefix.
		nodeStart := findNodeStart(heading, source)

		headings = append(headings, headingInfo{
			heading:   headingText.String(),
			nodeStart: nodeStart,
		})
	}

	// No level-2 headings: return entire document as preamble.
	if len(headings) == 0 {
		return []Section{{
			Heading:  "",
			Depth:    0,
			Content:  string(source),
			Position: 0,
		}}
	}

	var sections []Section
	pos := 0

	// Preamble: content before the first ## heading.
	if headings[0].nodeStart > 0 {
		preamble := bytes.TrimRight(source[:headings[0].nodeStart], "\n\r ")
		if len(preamble) > 0 {
			sections = append(sections, Section{
				Heading:  "",
				Depth:    0,
				Content:  string(preamble),
				Position: pos,
			})
			pos++
		}
	}

	// Each ## heading and its content until the next ## heading or EOF.
	for i, h := range headings {
		var regionEnd int
		if i+1 < len(headings) {
			regionEnd = headings[i+1].nodeStart
		} else {
			regionEnd = len(source)
		}

		// The region is from nodeStart to regionEnd. We need to skip the
		// heading line itself to get the body content.
		region := source[h.nodeStart:regionEnd]
		body := skipHeadingLine(region)
		body = bytes.TrimRight(body, "\n\r ")
		body = bytes.TrimLeft(body, "\n")

		sections = append(sections, Section{
			Heading:  h.heading,
			Depth:    2,
			Content:  string(body),
			Position: pos,
		})
		pos++
	}

	return sections
}

// findNodeStart returns the byte offset in source where this AST node begins.
// It uses the first child's text segment and scans backward to the start of line.
func findNodeStart(n ast.Node, source []byte) int {
	// Try to get position from inline children (text segments).
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			// Scan backward from the text start to find the line beginning (## prefix).
			pos := t.Segment.Start
			for pos > 0 && source[pos-1] != '\n' {
				pos--
			}
			return pos
		}
	}

	// Fallback: try Lines() on the block node.
	if block, ok := n.(interface{ Lines() *text.Segments }); ok {
		segs := block.Lines()
		if segs.Len() > 0 {
			return segs.At(0).Start
		}
	}

	return 0
}

// skipHeadingLine advances past the first line (the ## heading line) in a byte slice.
func skipHeadingLine(b []byte) []byte {
	idx := bytes.IndexByte(b, '\n')
	if idx < 0 {
		return nil
	}
	return b[idx+1:]
}
