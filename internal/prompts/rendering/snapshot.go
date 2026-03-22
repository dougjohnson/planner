package rendering

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/dougflynn/flywheel-planner/internal/artifacts"
	"github.com/dougflynn/flywheel-planner/internal/prompts/redaction"
)

// SnapshotService persists rendered prompt snapshots to the filesystem
// with credential redaction applied. Each snapshot is written to the project's
// prompts/ directory and recorded in the prompt_renders table.
type SnapshotService struct {
	store    *artifacts.Store
	redactor *redaction.Redactor
	db       *sql.DB
}

// NewSnapshotService creates a new snapshot service.
func NewSnapshotService(db *sql.DB, store *artifacts.Store, redactor *redaction.Redactor) *SnapshotService {
	return &SnapshotService{
		store:    store,
		redactor: redactor,
		db:       db,
	}
}

// SnapshotInput contains the data needed to persist a prompt snapshot.
type SnapshotInput struct {
	// WorkflowRunID links the snapshot to a workflow run.
	WorkflowRunID string
	// ProjectSlug is the project directory slug for filesystem paths.
	ProjectSlug string
	// RenderedPrompt is the assembled prompt to persist.
	RenderedPrompt *RenderedPrompt
}

// SnapshotOutput contains the result of persisting a prompt snapshot.
type SnapshotOutput struct {
	// RenderID is the prompt render record ID.
	RenderID string
	// FilePath is the relative path where the snapshot was written.
	FilePath string
	// Redacted indicates whether any credential redaction was applied.
	Redacted bool
	// Checksum is the SHA-256 of the written content.
	Checksum string
}

// Persist writes a rendered prompt snapshot to the filesystem with credential
// redaction applied, then creates a prompt_renders record.
func (ss *SnapshotService) Persist(ctx context.Context, input SnapshotInput) (*SnapshotOutput, error) {
	if input.WorkflowRunID == "" {
		return nil, fmt.Errorf("workflow_run_id is required")
	}
	if input.RenderedPrompt == nil {
		return nil, fmt.Errorf("rendered prompt is required")
	}

	// Serialize the prompt snapshot.
	snapshot := map[string]any{
		"id":                 input.RenderedPrompt.ID,
		"prompt_template_id": input.RenderedPrompt.PromptTemplateID,
		"artifact_ids":       input.RenderedPrompt.ArtifactIDs,
		"rendered_at":        input.RenderedPrompt.RenderedAt,
		"segments":           input.RenderedPrompt.Segments,
		"full_text":          input.RenderedPrompt.FullText(),
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling prompt snapshot: %w", err)
	}

	// Apply credential redaction.
	redacted := false
	if ss.redactor != nil {
		data, redacted = ss.redactor.RedactBytes(data)
	}

	// Write to filesystem.
	relPath := filepath.Join("projects", input.ProjectSlug, "prompts",
		fmt.Sprintf("render-%s.json", input.RenderedPrompt.ID))

	checksum, err := ss.store.WriteFile(relPath, data)
	if err != nil {
		return nil, fmt.Errorf("writing prompt snapshot: %w", err)
	}

	// Determine redaction status.
	redactionStatus := "clean"
	if redacted {
		redactionStatus = "redacted"
	}

	// Create prompt_renders record.
	_, err = ss.db.ExecContext(ctx, `
		INSERT INTO prompt_renders (id, workflow_run_id, prompt_template_id, rendered_prompt_path, redaction_status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, input.RenderedPrompt.ID, input.WorkflowRunID, input.RenderedPrompt.PromptTemplateID,
		relPath, redactionStatus, input.RenderedPrompt.RenderedAt)
	if err != nil {
		return nil, fmt.Errorf("saving prompt render record: %w", err)
	}

	return &SnapshotOutput{
		RenderID: input.RenderedPrompt.ID,
		FilePath: relPath,
		Redacted: redacted,
		Checksum: checksum,
	}, nil
}

// GetRenderPath returns the filesystem path for a prompt render record.
func (ss *SnapshotService) GetRenderPath(ctx context.Context, renderID string) (string, error) {
	var path string
	err := ss.db.QueryRowContext(ctx,
		"SELECT rendered_prompt_path FROM prompt_renders WHERE id = ?", renderID,
	).Scan(&path)
	if err != nil {
		return "", fmt.Errorf("querying render path: %w", err)
	}
	return path, nil
}
