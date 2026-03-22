package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ChangeHistoryEntry describes what changed in a single loop iteration.
type ChangeHistoryEntry struct {
	Iteration    int      `json:"iteration"`
	ModelFamily  string   `json:"model_family"`
	UpdatedFrags []string `json:"updated_fragments,omitempty"`
	AddedFrags   []string `json:"added_fragments,omitempty"`
	RemovedFrags []string `json:"removed_fragments,omitempty"`
	Guidance     string   `json:"guidance,omitempty"`
}

// ChangeHistory is the structured summary injected into review loop prompts.
type ChangeHistory struct {
	DocumentType string               `json:"document_type"`
	TotalIters   int                  `json:"total_iterations"`
	Entries      []ChangeHistoryEntry `json:"entries"`
}

// BuildChangeHistory constructs the change history for review loop iterations
// by querying workflow events and guidance injections. This gives the fresh
// session context about what has already been tried without replaying full
// prior conversations.
func BuildChangeHistory(ctx context.Context, db *sql.DB, projectID, documentType string) (*ChangeHistory, error) {
	reviewStage := "prd_review"
	commitStage := "prd_commit"
	if documentType == "plan" {
		reviewStage = "plan_review"
		commitStage = "plan_commit"
	}

	// Query completed review runs to build iteration history.
	rows, err := db.QueryContext(ctx, `
		SELECT wr.id, wr.stage, wr.attempt,
			COALESCE(mc.provider, '') as provider
		FROM workflow_runs wr
		LEFT JOIN model_configs mc ON mc.id = wr.model_config_id
		WHERE wr.project_id = ? AND wr.stage IN (?, ?) AND wr.status = 'completed'
		ORDER BY wr.created_at ASC`,
		projectID, reviewStage, commitStage)
	if err != nil {
		return nil, fmt.Errorf("querying review history: %w", err)
	}
	defer rows.Close()

	history := &ChangeHistory{
		DocumentType: documentType,
	}

	iterNum := 0
	for rows.Next() {
		var runID, stage, provider string
		var attempt int
		if err := rows.Scan(&runID, &stage, &attempt, &provider); err != nil {
			return nil, err
		}

		if stage == reviewStage {
			iterNum++
			family := "gpt"
			if provider == "anthropic" {
				family = "opus"
			}
			entry := ChangeHistoryEntry{
				Iteration:   iterNum,
				ModelFamily: family,
			}

			// Load fragment operations from this run's events.
			ops := loadRunFragmentOps(ctx, db, runID)
			for _, op := range ops {
				switch op {
				case "update":
					entry.UpdatedFrags = append(entry.UpdatedFrags, "updated")
				case "add":
					entry.AddedFrags = append(entry.AddedFrags, "added")
				case "remove":
					entry.RemovedFrags = append(entry.RemovedFrags, "removed")
				}
			}

			// Load any guidance injected for this iteration.
			entry.Guidance = loadIterationGuidance(ctx, db, projectID, reviewStage)

			history.Entries = append(history.Entries, entry)
		}
	}
	history.TotalIters = iterNum

	return history, rows.Err()
}

// RenderChangeHistoryMarkdown produces the concise markdown summary injected
// into the review prompt. This is not a full artifact replay — just enough
// context for a fresh session to understand the document's trajectory.
func RenderChangeHistoryMarkdown(history *ChangeHistory) string {
	if history == nil || len(history.Entries) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Prior Review History (%s)\n\n", history.DocumentType)
	fmt.Fprintf(&b, "This document has been through %d review iteration(s).\n\n", history.TotalIters)

	for _, entry := range history.Entries {
		fmt.Fprintf(&b, "### Iteration %d (%s)\n", entry.Iteration, entry.ModelFamily)

		changes := 0
		if len(entry.UpdatedFrags) > 0 {
			fmt.Fprintf(&b, "- Updated %d fragment(s)\n", len(entry.UpdatedFrags))
			changes += len(entry.UpdatedFrags)
		}
		if len(entry.AddedFrags) > 0 {
			fmt.Fprintf(&b, "- Added %d fragment(s)\n", len(entry.AddedFrags))
			changes += len(entry.AddedFrags)
		}
		if len(entry.RemovedFrags) > 0 {
			fmt.Fprintf(&b, "- Removed %d fragment(s)\n", len(entry.RemovedFrags))
			changes += len(entry.RemovedFrags)
		}
		if changes == 0 {
			b.WriteString("- No fragment operations (convergence candidate)\n")
		}
		if entry.Guidance != "" {
			fmt.Fprintf(&b, "- User guidance: %s\n", entry.Guidance)
		}
		b.WriteString("\n")
	}

	b.WriteString("Focus on changes that materially improve the document rather than re-proposing changes already made in prior iterations.\n")
	return b.String()
}

// --- Helpers ---

func loadRunFragmentOps(ctx context.Context, db *sql.DB, runID string) []string {
	rows, err := db.QueryContext(ctx,
		`SELECT event_type FROM workflow_events
		 WHERE workflow_run_id = ? AND event_type IN ('fragment_updated', 'fragment_added', 'fragment_removed')
		 ORDER BY created_at ASC`, runID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ops []string
	for rows.Next() {
		var eventType string
		if rows.Scan(&eventType) == nil {
			switch eventType {
			case "fragment_updated":
				ops = append(ops, "update")
			case "fragment_added":
				ops = append(ops, "add")
			case "fragment_removed":
				ops = append(ops, "remove")
			}
		}
	}
	return ops
}

func loadIterationGuidance(ctx context.Context, db *sql.DB, projectID, stage string) string {
	var content string
	err := db.QueryRowContext(ctx,
		`SELECT content FROM guidance_injections
		 WHERE project_id = ? AND stage = ?
		 ORDER BY created_at DESC LIMIT 1`, projectID, stage).Scan(&content)
	if err != nil {
		return ""
	}
	return content
}
