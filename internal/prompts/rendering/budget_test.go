package rendering

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 3, EstimateTokens("Hello world")) // 11 chars → ~3 tokens
	assert.Greater(t, EstimateTokens(strings.Repeat("a", 1000)), 200)
}

func TestComputeBudget_FitsInline(t *testing.T) {
	config := BudgetConfig{
		MaxContextTokens:   128000,
		ToolOverheadTokens: 300,
		WrapperTokens:      2000,
		ReserveTokens:      4000,
	}

	est := ComputeBudget(config, 2, "System prompt here", "Short document content", nil)
	assert.Equal(t, PackagingFull, est.Strategy)
	assert.True(t, est.FitsInline)
	assert.Empty(t, est.Overflows)
}

func TestComputeBudget_LargeDocument_Attachment(t *testing.T) {
	config := BudgetConfig{
		MaxContextTokens:   10000,
		ToolOverheadTokens: 300,
		WrapperTokens:      2000,
		ReserveTokens:      4000,
	}

	// Document that's too large for inline but fits in attachment.
	largeDoc := strings.Repeat("This is a substantial document section. ", 500) // ~20k chars
	est := ComputeBudget(config, 1, "prompt", largeDoc, nil)
	assert.Equal(t, PackagingAttachment, est.Strategy)
	assert.False(t, est.FitsInline)
}

func TestComputeBudget_Blocked(t *testing.T) {
	config := BudgetConfig{
		MaxContextTokens:   1000,
		ToolOverheadTokens: 300,
		WrapperTokens:      500,
		ReserveTokens:      500,
	}

	// Massive document that exceeds everything.
	hugeDoc := strings.Repeat("x", 100000) // 100k chars → ~25k tokens
	est := ComputeBudget(config, 2, "prompt", hugeDoc, nil)
	assert.Equal(t, PackagingBlocked, est.Strategy)
	assert.NotEmpty(t, est.Overflows)
}

func TestComputeBudget_OverflowDiagnostics(t *testing.T) {
	config := BudgetConfig{
		MaxContextTokens:   1000,
		ToolOverheadTokens: 200,
		WrapperTokens:      300,
		ReserveTokens:      300,
	}

	frags := []FragmentTokenEstimate{
		{FragmentID: "f-1", Heading: "Introduction", Tokens: 500},
		{FragmentID: "f-2", Heading: "Architecture", Tokens: 3000},
		{FragmentID: "f-3", Heading: "Testing", Tokens: 200},
	}

	hugeDoc := strings.Repeat("x", 100000)
	est := ComputeBudget(config, 1, "prompt", hugeDoc, frags)
	assert.Equal(t, PackagingBlocked, est.Strategy)
	assert.GreaterOrEqual(t, len(est.Overflows), 1)

	// Should identify the largest fragment.
	hasLargest := false
	for _, d := range est.Overflows {
		if d.Component == "largest_fragment" {
			hasLargest = true
			assert.Contains(t, d.Explanation, "Architecture")
		}
	}
	assert.True(t, hasLargest)
}

func TestComputeBudget_ToolOverhead(t *testing.T) {
	config := DefaultBudgetConfig(128000)

	est1 := ComputeBudget(config, 1, "prompt", "doc", nil)
	est4 := ComputeBudget(config, 4, "prompt", "doc", nil)

	// More tools = more overhead.
	assert.Greater(t, est4.ToolTokens, est1.ToolTokens)
	assert.Equal(t, 4*config.ToolOverheadTokens, est4.ToolTokens)
}

func TestFormatBudgetSummary(t *testing.T) {
	est := BudgetEstimate{
		SystemTokens:     500,
		ToolTokens:       600,
		WrapperTokens:    2000,
		DocumentTokens:   5000,
		TotalInputTokens: 8100,
		AvailableTokens:  120000,
		Strategy:         PackagingFull,
		FitsInline:       true,
	}

	summary := FormatBudgetSummary(est)
	assert.Contains(t, summary, "full")
	assert.Contains(t, summary, "5000")
	assert.Contains(t, summary, "120000")
}

func TestDefaultBudgetConfig(t *testing.T) {
	cfg := DefaultBudgetConfig(200000)
	assert.Equal(t, 200000, cfg.MaxContextTokens)
	assert.Greater(t, cfg.ToolOverheadTokens, 0)
	assert.Greater(t, cfg.WrapperTokens, 0)
	assert.Greater(t, cfg.ReserveTokens, 0)
}
