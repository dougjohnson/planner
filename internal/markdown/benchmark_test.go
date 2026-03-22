package markdown

import (
	"fmt"
	"strings"
	"testing"
)

// generateLargeDocument creates a markdown document with the specified number
// of ## sections, each with content of the given line count.
func generateLargeDocument(sections, linesPerSection int) []byte {
	var b strings.Builder
	b.WriteString("# Large Document\n\nPreamble content before any section.\n\n")
	for i := 0; i < sections; i++ {
		b.WriteString(fmt.Sprintf("## Section %d: Topic %d\n\n", i+1, i+1))
		for j := 0; j < linesPerSection; j++ {
			b.WriteString(fmt.Sprintf("Line %d of section %d. This is detailed content about the topic including requirements, constraints, and implementation details.\n", j+1, i+1))
		}
		b.WriteString("\n")
	}
	return []byte(b.String())
}

func BenchmarkDecompose_Small(b *testing.B) {
	doc := generateLargeDocument(5, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decompose(doc)
	}
}

func BenchmarkDecompose_Medium(b *testing.B) {
	doc := generateLargeDocument(15, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decompose(doc)
	}
}

func BenchmarkDecompose_Large(b *testing.B) {
	doc := generateLargeDocument(30, 50)
	b.SetBytes(int64(len(doc)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decompose(doc)
	}
}

func BenchmarkDecompose_Stress(b *testing.B) {
	doc := generateLargeDocument(55, 100) // ~55 sections, 10K+ lines
	b.SetBytes(int64(len(doc)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decompose(doc)
	}
}

func BenchmarkDecompose_NoHeadings(b *testing.B) {
	// Document with no ## headings — single preamble fragment.
	var builder strings.Builder
	for i := 0; i < 500; i++ {
		builder.WriteString(fmt.Sprintf("Line %d of a document with no headings.\n", i+1))
	}
	doc := []byte(builder.String())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decompose(doc)
	}
}

func BenchmarkDecompose_ManySmallSections(b *testing.B) {
	doc := generateLargeDocument(100, 3) // 100 sections, 3 lines each
	b.SetBytes(int64(len(doc)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decompose(doc)
	}
}
