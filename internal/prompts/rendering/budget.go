// Package rendering handles prompt assembly, context budgeting, and
// rendered prompt snapshot management.
package rendering

import (
	"fmt"
	"strings"
)

// PackagingStrategy describes how content is packaged for model context.
type PackagingStrategy string

const (
	PackagingFull       PackagingStrategy = "full"       // entire document fits inline
	PackagingAttachment PackagingStrategy = "attachment"  // sent as file attachment
	PackagingFragPack   PackagingStrategy = "frag_pack"  // deterministic fragment packaging
	PackagingBlocked    PackagingStrategy = "blocked"     // exceeds all limits, stage blocked
)

// BudgetConfig holds token limits and overhead estimates for context planning.
type BudgetConfig struct {
	MaxContextTokens    int `json:"max_context_tokens"`
	ToolOverheadTokens  int `json:"tool_overhead_tokens"`  // per tool definition (~200-400 tokens each)
	WrapperTokens       int `json:"wrapper_tokens"`        // system instructions, output format
	ReserveTokens       int `json:"reserve_tokens"`        // safety margin for response
}

// DefaultBudgetConfig returns sensible defaults for context budgeting.
func DefaultBudgetConfig(maxContext int) BudgetConfig {
	return BudgetConfig{
		MaxContextTokens:   maxContext,
		ToolOverheadTokens: 300,  // per tool
		WrapperTokens:      2000, // system prompt, output instructions
		ReserveTokens:      4000, // response generation headroom
	}
}

// BudgetEstimate holds the token budget breakdown for a stage invocation.
type BudgetEstimate struct {
	// Input breakdown.
	SystemTokens     int `json:"system_tokens"`
	ToolTokens       int `json:"tool_tokens"`
	WrapperTokens    int `json:"wrapper_tokens"`
	DocumentTokens   int `json:"document_tokens"`
	FragmentDetails  []FragmentTokenEstimate `json:"fragment_details,omitempty"`

	// Totals.
	TotalInputTokens int               `json:"total_input_tokens"`
	AvailableTokens  int               `json:"available_tokens"`
	Strategy         PackagingStrategy `json:"strategy"`
	FitsInline       bool              `json:"fits_inline"`

	// Diagnostics (populated when over budget).
	Overflows []OverflowDiagnostic `json:"overflows,omitempty"`
}

// FragmentTokenEstimate holds the estimated token count for a single fragment.
type FragmentTokenEstimate struct {
	FragmentID string `json:"fragment_id"`
	Heading    string `json:"heading"`
	Tokens     int    `json:"tokens"`
}

// OverflowDiagnostic explains what caused a context budget overflow.
type OverflowDiagnostic struct {
	Component   string `json:"component"`
	Tokens      int    `json:"tokens"`
	Explanation string `json:"explanation"`
}

// EstimateTokens provides a rough token estimate for text content.
// Uses the common heuristic of ~4 characters per token for English text.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	// ~4 chars per token is a reasonable approximation for English.
	// This is intentionally conservative (overestimates slightly).
	return (len(text) + 3) / 4
}

// ComputeBudget calculates the context budget for a stage invocation,
// determining the packaging strategy and producing overflow diagnostics.
func ComputeBudget(
	config BudgetConfig,
	toolCount int,
	promptText string,
	documentContent string,
	fragmentEstimates []FragmentTokenEstimate,
) BudgetEstimate {
	est := BudgetEstimate{}

	// Fixed overhead.
	est.WrapperTokens = config.WrapperTokens
	est.ToolTokens = toolCount * config.ToolOverheadTokens
	est.SystemTokens = EstimateTokens(promptText)
	est.DocumentTokens = EstimateTokens(documentContent)
	est.FragmentDetails = fragmentEstimates

	fixedOverhead := est.WrapperTokens + est.ToolTokens + est.SystemTokens + config.ReserveTokens
	est.AvailableTokens = config.MaxContextTokens - fixedOverhead
	est.TotalInputTokens = fixedOverhead + est.DocumentTokens

	// Decision logic.
	if est.DocumentTokens <= est.AvailableTokens {
		est.Strategy = PackagingFull
		est.FitsInline = true
		return est
	}

	// Try attachment strategy (providers with file support).
	// Attachments typically have separate token budgets.
	attachmentLimit := config.MaxContextTokens * 2 // attachments often allow 2x
	if est.DocumentTokens <= attachmentLimit {
		est.Strategy = PackagingAttachment
		est.FitsInline = false
		return est
	}

	// Try fragment packing — include as many fragments as fit.
	fragTotal := 0
	for _, f := range fragmentEstimates {
		fragTotal += f.Tokens
	}
	if fragTotal <= est.AvailableTokens {
		est.Strategy = PackagingFragPack
		est.FitsInline = false
		return est
	}

	// Nothing fits — stage must be blocked.
	est.Strategy = PackagingBlocked
	est.FitsInline = false
	est.Overflows = buildOverflowDiagnostics(est, config)
	return est
}

// FormatBudgetSummary returns a human-readable budget summary.
func FormatBudgetSummary(est BudgetEstimate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Context budget: %s\n", est.Strategy)
	fmt.Fprintf(&b, "  System:   %6d tokens\n", est.SystemTokens)
	fmt.Fprintf(&b, "  Tools:    %6d tokens\n", est.ToolTokens)
	fmt.Fprintf(&b, "  Wrapper:  %6d tokens\n", est.WrapperTokens)
	fmt.Fprintf(&b, "  Document: %6d tokens\n", est.DocumentTokens)
	fmt.Fprintf(&b, "  Total:    %6d tokens\n", est.TotalInputTokens)
	fmt.Fprintf(&b, "  Available:%6d tokens\n", est.AvailableTokens)

	if len(est.Overflows) > 0 {
		b.WriteString("\nOverflow diagnostics:\n")
		for _, o := range est.Overflows {
			fmt.Fprintf(&b, "  - %s: %d tokens — %s\n", o.Component, o.Tokens, o.Explanation)
		}
	}
	return b.String()
}

func buildOverflowDiagnostics(est BudgetEstimate, config BudgetConfig) []OverflowDiagnostic {
	var diags []OverflowDiagnostic

	excess := est.TotalInputTokens - config.MaxContextTokens
	diags = append(diags, OverflowDiagnostic{
		Component:   "total",
		Tokens:      excess,
		Explanation: fmt.Sprintf("exceeds context limit by %d tokens", excess),
	})

	// Identify the largest fragments contributing to overflow.
	if len(est.FragmentDetails) > 0 {
		var largestName string
		largestTokens := 0
		for _, f := range est.FragmentDetails {
			if f.Tokens > largestTokens {
				largestTokens = f.Tokens
				largestName = f.Heading
			}
		}
		if largestName != "" {
			diags = append(diags, OverflowDiagnostic{
				Component:   "largest_fragment",
				Tokens:      largestTokens,
				Explanation: fmt.Sprintf("fragment %q is the largest at %d tokens", largestName, largestTokens),
			})
		}
	}

	return diags
}
