package review

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Review item and decision errors.
var (
	ErrReviewItemNotFound = errors.New("review item not found")
	ErrAlreadyDecided     = errors.New("review item already has a decision")
)

// ReviewItemStatus represents the lifecycle of a review item.
type ReviewItemStatus string

const (
	StatusPending  ReviewItemStatus = "pending"
	StatusAccepted ReviewItemStatus = "accepted"
	StatusRejected ReviewItemStatus = "rejected"
)

// ReviewItem represents a disagreement or flagged change from a model run
// that requires user review. Created directly from report_disagreement tool calls.
type ReviewItem struct {
	ID              string           `json:"id"`
	ProjectID       string           `json:"project_id"`
	FragmentID      string           `json:"fragment_id"`
	Stage           string           `json:"stage"`
	RunID           string           `json:"run_id"`
	Severity        string           `json:"severity"`
	Summary         string           `json:"summary"`
	Rationale       string           `json:"rationale"`
	SuggestedChange string           `json:"suggested_change"`
	Status          ReviewItemStatus `json:"status"`
	CreatedAt       string           `json:"created_at"`
}

// ReviewDecision records the user's accept/reject decision on a review item.
type ReviewDecision struct {
	ID           string `json:"id"`
	ReviewItemID string `json:"review_item_id"`
	Action       string `json:"action"` // "accepted" or "rejected"
	UserNote     string `json:"user_note,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// ReviewRepository manages review items and decisions.
type ReviewRepository struct {
	db *sql.DB
}

// NewReviewRepository creates a new repository backed by the given database.
func NewReviewRepository(db *sql.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

// CreateReviewItem inserts a new review item from a report_disagreement tool call.
func (r *ReviewRepository) CreateReviewItem(ctx context.Context, projectID, fragmentID, stage, runID, severity, summary, rationale, suggestedChange string) (*ReviewItem, error) {
	item := &ReviewItem{
		ID:              uuid.NewString(),
		ProjectID:       projectID,
		FragmentID:      fragmentID,
		Stage:           stage,
		RunID:           runID,
		Severity:        severity,
		Summary:         summary,
		Rationale:       rationale,
		SuggestedChange: suggestedChange,
		Status:          StatusPending,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO review_items (id, project_id, fragment_id, stage, run_id, severity, summary, rationale, suggested_change, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.ProjectID, item.FragmentID, item.Stage, item.RunID,
		item.Severity, item.Summary, item.Rationale, item.SuggestedChange,
		string(item.Status), item.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting review item: %w", err)
	}
	return item, nil
}

// GetReviewItem returns a review item by ID.
func (r *ReviewRepository) GetReviewItem(ctx context.Context, id string) (*ReviewItem, error) {
	item := &ReviewItem{}
	var status string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, fragment_id, stage, run_id, severity, summary, rationale, suggested_change, status, created_at
		 FROM review_items WHERE id = ?`, id,
	).Scan(&item.ID, &item.ProjectID, &item.FragmentID, &item.Stage, &item.RunID,
		&item.Severity, &item.Summary, &item.Rationale, &item.SuggestedChange,
		&status, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrReviewItemNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying review item: %w", err)
	}
	item.Status = ReviewItemStatus(status)
	return item, nil
}

// ListByProject returns review items for a project, optionally filtered by stage and status.
func (r *ReviewRepository) ListByProject(ctx context.Context, projectID string, stage string, status string) ([]*ReviewItem, error) {
	query := `SELECT id, project_id, fragment_id, stage, run_id, severity, summary, rationale, suggested_change, status, created_at
		FROM review_items WHERE project_id = ?`
	args := []any{projectID}

	if stage != "" {
		query += " AND stage = ?"
		args = append(args, stage)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing review items: %w", err)
	}
	defer rows.Close()

	var items []*ReviewItem
	for rows.Next() {
		item := &ReviewItem{}
		var s string
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.FragmentID, &item.Stage, &item.RunID,
			&item.Severity, &item.Summary, &item.Rationale, &item.SuggestedChange,
			&s, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning review item: %w", err)
		}
		item.Status = ReviewItemStatus(s)
		items = append(items, item)
	}
	return items, rows.Err()
}

// RecordDecision records a user's accept/reject decision and updates the review item status.
// The status check and update are performed atomically inside a transaction to prevent
// TOCTOU races where two concurrent calls could both see "pending" and both succeed.
func (r *ReviewRepository) RecordDecision(ctx context.Context, reviewItemID, action, userNote string) (*ReviewDecision, error) {
	if action != "accepted" && action != "rejected" {
		return nil, fmt.Errorf("invalid action %q: must be 'accepted' or 'rejected'", action)
	}

	decision := &ReviewDecision{
		ID:           uuid.NewString(),
		ReviewItemID: reviewItemID,
		Action:       action,
		UserNote:     userNote,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Atomically check status and update in one statement. If the item is not
	// pending, zero rows are affected and we detect the conflict.
	result, err := tx.ExecContext(ctx,
		`UPDATE review_items SET status = ? WHERE id = ? AND status = 'pending'`,
		action, reviewItemID,
	)
	if err != nil {
		return nil, fmt.Errorf("updating review item status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected: %w", err)
	}
	if affected == 0 {
		// Either the item doesn't exist or it's not pending.
		// Query within the transaction to avoid deadlock with MaxOpenConns(1).
		var currentStatus string
		err := tx.QueryRowContext(ctx,
			"SELECT status FROM review_items WHERE id = ?", reviewItemID,
		).Scan(&currentStatus)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrReviewItemNotFound, reviewItemID)
		}
		return nil, fmt.Errorf("%w: current status is %s", ErrAlreadyDecided, currentStatus)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO review_decisions (id, review_item_id, action, user_note, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		decision.ID, decision.ReviewItemID, decision.Action, decision.UserNote, decision.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting decision: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing decision: %w", err)
	}

	return decision, nil
}

// BulkDecide records decisions for multiple review items in a single transaction.
func (r *ReviewRepository) BulkDecide(ctx context.Context, decisions []struct {
	ReviewItemID string
	Action       string
	UserNote     string
}) (accepted int, rejected int, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	for _, d := range decisions {
		if d.Action != "accepted" && d.Action != "rejected" {
			return 0, 0, fmt.Errorf("invalid action %q for item %s", d.Action, d.ReviewItemID)
		}

		decisionID := uuid.NewString()
		now := time.Now().UTC().Format(time.RFC3339)

		_, err = tx.ExecContext(ctx,
			`INSERT INTO review_decisions (id, review_item_id, action, user_note, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			decisionID, d.ReviewItemID, d.Action, d.UserNote, now,
		)
		if err != nil {
			return 0, 0, fmt.Errorf("inserting decision for %s: %w", d.ReviewItemID, err)
		}

		_, err = tx.ExecContext(ctx,
			`UPDATE review_items SET status = ? WHERE id = ?`,
			d.Action, d.ReviewItemID,
		)
		if err != nil {
			return 0, 0, fmt.Errorf("updating status for %s: %w", d.ReviewItemID, err)
		}

		if d.Action == "accepted" {
			accepted++
		} else {
			rejected++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("committing bulk decisions: %w", err)
	}

	return accepted, rejected, nil
}
