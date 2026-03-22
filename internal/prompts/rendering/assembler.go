// Package rendering provides the prompt assembly pipeline for flywheel-planner.
// It composes prompts from multiple sources in the deterministic order specified
// in §11.3.1: system instructions → foundational context → prompt text →
// artifact context → change history → user guidance.
package rendering

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Segment represents a distinct section of a rendered prompt.
type Segment struct {
	Label   string `json:"label"`
	Content string `json:"content"`
}

// RenderedPrompt is the fully assembled prompt for a single model run.
type RenderedPrompt struct {
	ID              string    `json:"id"`
	Segments        []Segment `json:"segments"`
	PromptTemplateID string   `json:"prompt_template_id"`
	ArtifactIDs     []string  `json:"artifact_ids,omitempty"`
	RenderedAt      string    `json:"rendered_at"`
}

// FullText returns the concatenated text of all segments.
func (rp *RenderedPrompt) FullText() string {
	var b strings.Builder
	for i, seg := range rp.Segments {
		if i > 0 {
			b.WriteString("\n\n---\n\n")
		}
		b.WriteString(seg.Content)
	}
	return b.String()
}

// AssemblyInput contains all the data needed to assemble a prompt.
type AssemblyInput struct {
	// Stage identifies the workflow stage.
	Stage string

	// SystemInstructions contains role definition and output format guidance.
	SystemInstructions string

	// ToolDefinitions contains stage-specific tool schemas as text.
	ToolDefinitions string

	// FoundationalContext contains project metadata, AGENTS.md, stack, architecture.
	FoundationalContext string

	// PromptTemplateID is the ID of the active prompt template.
	PromptTemplateID string

	// PromptText is the active prompt text for this stage.
	PromptText string

	// ArtifactContext contains composed documents with fragment annotations.
	ArtifactContext string

	// ArtifactIDs lists all artifact IDs referenced in the context.
	ArtifactIDs []string

	// ChangeHistory contains the loop change history summary (empty for first iteration).
	ChangeHistory string

	// UserGuidance contains user guidance injections for this stage.
	UserGuidance string
}

// Assembler builds rendered prompts from AssemblyInput.
type Assembler struct {
	db *sql.DB
}

// NewAssembler creates a new Assembler.
func NewAssembler(db *sql.DB) *Assembler {
	return &Assembler{db: db}
}

// Assemble composes a rendered prompt from the input in the deterministic
// order specified by §11.3.1. Each section becomes a distinct Segment.
func (a *Assembler) Assemble(_ context.Context, input AssemblyInput) *RenderedPrompt {
	var segments []Segment

	// 1. System instructions + tool definitions.
	if input.SystemInstructions != "" || input.ToolDefinitions != "" {
		var parts []string
		if input.SystemInstructions != "" {
			parts = append(parts, input.SystemInstructions)
		}
		if input.ToolDefinitions != "" {
			parts = append(parts, input.ToolDefinitions)
		}
		segments = append(segments, Segment{
			Label:   "system_instructions",
			Content: strings.Join(parts, "\n\n"),
		})
	}

	// 2. Foundational context.
	if input.FoundationalContext != "" {
		segments = append(segments, Segment{
			Label:   "foundational_context",
			Content: input.FoundationalContext,
		})
	}

	// 3. Active prompt text.
	if input.PromptText != "" {
		segments = append(segments, Segment{
			Label:   "prompt_text",
			Content: input.PromptText,
		})
	}

	// 4. Artifact context (composed documents with annotations).
	if input.ArtifactContext != "" {
		segments = append(segments, Segment{
			Label:   "artifact_context",
			Content: input.ArtifactContext,
		})
	}

	// 5. Change history (loop iterations after the first).
	if input.ChangeHistory != "" {
		segments = append(segments, Segment{
			Label:   "change_history",
			Content: input.ChangeHistory,
		})
	}

	// 6. User guidance injections.
	if input.UserGuidance != "" {
		segments = append(segments, Segment{
			Label:   "user_guidance",
			Content: input.UserGuidance,
		})
	}

	return &RenderedPrompt{
		ID:               uuid.NewString(),
		Segments:         segments,
		PromptTemplateID: input.PromptTemplateID,
		ArtifactIDs:      input.ArtifactIDs,
		RenderedAt:       time.Now().UTC().Format(time.RFC3339),
	}
}

// SaveRender persists a rendered prompt record to the database.
func (a *Assembler) SaveRender(ctx context.Context, workflowRunID string, rp *RenderedPrompt, renderedPath string) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO prompt_renders (id, workflow_run_id, prompt_template_id, rendered_prompt_path, redaction_status, created_at)
		VALUES (?, ?, ?, ?, 'pending', ?)
	`, rp.ID, workflowRunID, rp.PromptTemplateID, renderedPath, rp.RenderedAt)
	if err != nil {
		return fmt.Errorf("saving prompt render: %w", err)
	}
	return nil
}
