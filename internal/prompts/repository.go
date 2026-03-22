// Package prompts provides the prompt template data model and repository
// for flywheel-planner. Prompt templates are the canonical, versioned prompts
// used by each workflow stage. Locked templates cannot be modified — only
// cloned into wrapper variants by advanced users.
package prompts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound is returned when a prompt template does not exist.
	ErrNotFound = errors.New("prompt template not found")

	// ErrLocked is returned when attempting to modify a locked template.
	ErrLocked = errors.New("prompt template is locked")

	// ErrDuplicateVersion is returned when a name+version pair already exists.
	ErrDuplicateVersion = errors.New("prompt template version already exists")
)

// LockedStatus represents the editability of a prompt template.
type LockedStatus string

const (
	StatusUnlocked LockedStatus = "unlocked"
	StatusLocked   LockedStatus = "locked"
)

// PromptTemplate is a versioned prompt used by a workflow stage.
type PromptTemplate struct {
	ID                     string       `json:"id"`
	Name                   string       `json:"name"`
	Stage                  string       `json:"stage"`
	Version                int          `json:"version"`
	BaselineText           string       `json:"baseline_text"`
	WrapperText            string       `json:"wrapper_text"`
	OutputContractJSON     string       `json:"output_contract_json"`
	LockedStatus           LockedStatus `json:"locked_status"`
	OriginalPRDBaselineText *string     `json:"original_prd_baseline_text,omitempty"`
	CreatedAt              string       `json:"created_at"`
	UpdatedAt              string       `json:"updated_at"`
	DeprecatedAt           *string      `json:"deprecated_at,omitempty"`
}

// IsLocked returns true if the template cannot be modified.
func (pt *PromptTemplate) IsLocked() bool {
	return pt.LockedStatus == StatusLocked
}

// IsDeprecated returns true if the template has been deprecated.
func (pt *PromptTemplate) IsDeprecated() bool {
	return pt.DeprecatedAt != nil
}

// Repository provides data access operations for prompt templates.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new prompt template repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new prompt template.
func (r *Repository) Create(ctx context.Context, pt *PromptTemplate) (*PromptTemplate, error) {
	if pt.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if pt.Stage == "" {
		return nil, fmt.Errorf("stage is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result := &PromptTemplate{
		ID:                      uuid.NewString(),
		Name:                    pt.Name,
		Stage:                   pt.Stage,
		Version:                 pt.Version,
		BaselineText:            pt.BaselineText,
		WrapperText:             pt.WrapperText,
		OutputContractJSON:      pt.OutputContractJSON,
		LockedStatus:            pt.LockedStatus,
		OriginalPRDBaselineText: pt.OriginalPRDBaselineText,
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	if result.Version == 0 {
		result.Version = 1
	}
	if result.LockedStatus == "" {
		result.LockedStatus = StatusUnlocked
	}
	if result.OutputContractJSON == "" {
		result.OutputContractJSON = "{}"
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO prompt_templates (id, name, stage, version, baseline_text, wrapper_text,
		 output_contract_json, locked_status, original_prd_baseline_text, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.ID, result.Name, result.Stage, result.Version, result.BaselineText,
		result.WrapperText, result.OutputContractJSON, result.LockedStatus,
		result.OriginalPRDBaselineText, result.CreatedAt, result.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: %s v%d", ErrDuplicateVersion, pt.Name, pt.Version)
		}
		return nil, fmt.Errorf("inserting prompt template: %w", err)
	}

	return result, nil
}

// GetByID returns a prompt template by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*PromptTemplate, error) {
	return r.scanOne(ctx,
		`SELECT id, name, stage, version, baseline_text, wrapper_text, output_contract_json,
		 locked_status, original_prd_baseline_text, created_at, updated_at, deprecated_at
		 FROM prompt_templates WHERE id = ?`, id)
}

// GetByNameVersion returns the prompt template matching the given name and version.
func (r *Repository) GetByNameVersion(ctx context.Context, name string, version int) (*PromptTemplate, error) {
	return r.scanOne(ctx,
		`SELECT id, name, stage, version, baseline_text, wrapper_text, output_contract_json,
		 locked_status, original_prd_baseline_text, created_at, updated_at, deprecated_at
		 FROM prompt_templates WHERE name = ? AND version = ?`, name, version)
}

// GetLatestByName returns the latest (highest version) template with the given name.
func (r *Repository) GetLatestByName(ctx context.Context, name string) (*PromptTemplate, error) {
	return r.scanOne(ctx,
		`SELECT id, name, stage, version, baseline_text, wrapper_text, output_contract_json,
		 locked_status, original_prd_baseline_text, created_at, updated_at, deprecated_at
		 FROM prompt_templates WHERE name = ? ORDER BY version DESC LIMIT 1`, name)
}

// ListByStage returns all non-deprecated templates for a given stage,
// ordered by name and version.
func (r *Repository) ListByStage(ctx context.Context, stage string) ([]*PromptTemplate, error) {
	return r.scanMany(ctx,
		`SELECT id, name, stage, version, baseline_text, wrapper_text, output_contract_json,
		 locked_status, original_prd_baseline_text, created_at, updated_at, deprecated_at
		 FROM prompt_templates
		 WHERE stage = ? AND deprecated_at IS NULL
		 ORDER BY name ASC, version DESC`, stage)
}

// ListAll returns all non-deprecated prompt templates.
func (r *Repository) ListAll(ctx context.Context) ([]*PromptTemplate, error) {
	return r.scanMany(ctx,
		`SELECT id, name, stage, version, baseline_text, wrapper_text, output_contract_json,
		 locked_status, original_prd_baseline_text, created_at, updated_at, deprecated_at
		 FROM prompt_templates
		 WHERE deprecated_at IS NULL
		 ORDER BY stage ASC, name ASC, version DESC`)
}

// Update modifies a prompt template's mutable fields. Returns ErrLocked if
// the template is locked.
func (r *Repository) Update(ctx context.Context, id string, baselineText, wrapperText, outputContractJSON string) error {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsLocked() {
		return fmt.Errorf("%w: %s v%d", ErrLocked, existing.Name, existing.Version)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = r.db.ExecContext(ctx,
		`UPDATE prompt_templates
		 SET baseline_text = ?, wrapper_text = ?, output_contract_json = ?, updated_at = ?
		 WHERE id = ?`,
		baselineText, wrapperText, outputContractJSON, now, id,
	)
	if err != nil {
		return fmt.Errorf("updating prompt template: %w", err)
	}
	return nil
}

// Lock marks a template as locked, preventing further modification.
func (r *Repository) Lock(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx,
		"UPDATE prompt_templates SET locked_status = ?, updated_at = ? WHERE id = ?",
		StatusLocked, now, id,
	)
	if err != nil {
		return fmt.Errorf("locking prompt template: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return nil
}

// Deprecate marks a template as deprecated. Deprecated templates are excluded
// from ListByStage and ListAll queries.
func (r *Repository) Deprecate(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx,
		"UPDATE prompt_templates SET deprecated_at = ?, updated_at = ? WHERE id = ?",
		now, now, id,
	)
	if err != nil {
		return fmt.Errorf("deprecating prompt template: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return nil
}

// Clone creates a new version of a template, incrementing the version number.
// The new template is unlocked regardless of the source's lock status.
func (r *Repository) Clone(ctx context.Context, sourceID string) (*PromptTemplate, error) {
	source, err := r.GetByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("cloning: %w", err)
	}

	return r.Create(ctx, &PromptTemplate{
		Name:                    source.Name,
		Stage:                   source.Stage,
		Version:                 source.Version + 1,
		BaselineText:            source.BaselineText,
		WrapperText:             source.WrapperText,
		OutputContractJSON:      source.OutputContractJSON,
		LockedStatus:            StatusUnlocked,
		OriginalPRDBaselineText: source.OriginalPRDBaselineText,
	})
}

// scanOne scans a single row into a PromptTemplate.
func (r *Repository) scanOne(ctx context.Context, query string, args ...any) (*PromptTemplate, error) {
	pt := &PromptTemplate{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&pt.ID, &pt.Name, &pt.Stage, &pt.Version, &pt.BaselineText,
		&pt.WrapperText, &pt.OutputContractJSON, &pt.LockedStatus,
		&pt.OriginalPRDBaselineText, &pt.CreatedAt, &pt.UpdatedAt, &pt.DeprecatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("querying prompt template: %w", err)
	}
	return pt, nil
}

// scanMany scans multiple rows into PromptTemplates.
func (r *Repository) scanMany(ctx context.Context, query string, args ...any) ([]*PromptTemplate, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing prompt templates: %w", err)
	}
	defer rows.Close()

	var templates []*PromptTemplate
	for rows.Next() {
		pt := &PromptTemplate{}
		if err := rows.Scan(
			&pt.ID, &pt.Name, &pt.Stage, &pt.Version, &pt.BaselineText,
			&pt.WrapperText, &pt.OutputContractJSON, &pt.LockedStatus,
			&pt.OriginalPRDBaselineText, &pt.CreatedAt, &pt.UpdatedAt, &pt.DeprecatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning prompt template: %w", err)
		}
		templates = append(templates, pt)
	}
	return templates, rows.Err()
}

// isUniqueViolation checks if the error is a UNIQUE constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed")
}

// containsStr is kept for backwards compatibility with any callers.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
