// Package queries provides database access layers for the flywheel-planner application.
package queries

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Project represents a project row in the database.
type Project struct {
	ID                        string
	Name                      string
	Description               string
	Status                    string
	WorkflowDefinitionVersion string
	CurrentStage              string
	CreatedAt                 string
	UpdatedAt                 string
	ArchivedAt                sql.NullString
}

// ProjectFilter configures list queries.
type ProjectFilter struct {
	IncludeArchived bool
	Limit           int
	Offset          int
}

// ProjectRepo provides CRUD operations for the projects table.
type ProjectRepo struct {
	db *sql.DB
}

// NewProjectRepo creates a new ProjectRepo.
func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// Create inserts a new project and returns its ID.
func (r *ProjectRepo) Create(ctx context.Context, name, description string) (*Project, error) {
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, description, status, workflow_definition_version, current_stage, created_at, updated_at)
		VALUES (?, ?, ?, 'active', '1', '', ?, ?)
	`, id, name, description, now, now)
	if err != nil {
		return nil, fmt.Errorf("inserting project: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID returns a project by its ID.
func (r *ProjectRepo) GetByID(ctx context.Context, id string) (*Project, error) {
	p := &Project{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, status, workflow_definition_version, current_stage, created_at, updated_at, archived_at
		FROM projects WHERE id = ?
	`, id).Scan(
		&p.ID, &p.Name, &p.Description, &p.Status,
		&p.WorkflowDefinitionVersion, &p.CurrentStage,
		&p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting project %s: %w", id, err)
	}
	return p, nil
}

// List returns projects matching the filter.
func (r *ProjectRepo) List(ctx context.Context, filter ProjectFilter) ([]*Project, error) {
	query := `SELECT id, name, description, status, workflow_definition_version, current_stage, created_at, updated_at, archived_at FROM projects`
	var args []any

	if !filter.IncludeArchived {
		query += " WHERE archived_at IS NULL"
	}
	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	projects := make([]*Project, 0)
	for rows.Next() {
		p := &Project{}
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.Status,
			&p.WorkflowDefinitionVersion, &p.CurrentStage,
			&p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning project row: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// Update patches mutable fields on a project. Only non-empty values are applied.
func (r *ProjectRepo) Update(ctx context.Context, id string, updates map[string]string) (*Project, error) {
	if len(updates) == 0 {
		return r.GetByID(ctx, id)
	}

	// Allowed mutable fields.
	allowed := map[string]bool{
		"name": true, "description": true, "status": true,
		"current_stage": true, "workflow_definition_version": true,
	}

	var setClauses []string
	var args []any
	for field, value := range updates {
		if !allowed[field] {
			return nil, fmt.Errorf("field %q is not updatable", field)
		}
		setClauses = append(setClauses, field+" = ?")
		args = append(args, value)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now)
	args = append(args, id)

	query := fmt.Sprintf("UPDATE projects SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("updating project %s: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("project %s not found", id)
	}

	return r.GetByID(ctx, id)
}

// Archive sets the archived_at timestamp, hiding the project from default views.
func (r *ProjectRepo) Archive(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := r.db.ExecContext(ctx, `
		UPDATE projects SET archived_at = ?, updated_at = ? WHERE id = ? AND archived_at IS NULL
	`, now, now, id)
	if err != nil {
		return fmt.Errorf("archiving project %s: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("project %s not found or already archived", id)
	}
	return nil
}

// Resume clears the archived_at timestamp, restoring the project to active views.
func (r *ProjectRepo) Resume(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := r.db.ExecContext(ctx, `
		UPDATE projects SET archived_at = NULL, updated_at = ? WHERE id = ? AND archived_at IS NOT NULL
	`, now, id)
	if err != nil {
		return fmt.Errorf("resuming project %s: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("project %s not found or not archived", id)
	}
	return nil
}
