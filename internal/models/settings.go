package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrQuorumNotMet is returned when the required model families are not enabled.
	ErrQuorumNotMet = errors.New("model quorum not met: at least one GPT-family and one Opus-family model must be enabled")
)

// ProjectModelSetting represents a per-project enable/disable for a model config.
type ProjectModelSetting struct {
	ID            string `json:"id"`
	ProjectID     string `json:"project_id"`
	ModelConfigID string `json:"model_config_id"`
	Enabled       bool   `json:"enabled"`
	PriorityOrder int    `json:"priority_order"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// ModelSettingsService manages per-project model enable/disable settings.
type ModelSettingsService struct {
	db *sql.DB
}

// NewModelSettingsService creates a new settings service.
func NewModelSettingsService(db *sql.DB) *ModelSettingsService {
	return &ModelSettingsService{db: db}
}

// SetEnabled enables or disables a model for a project.
func (s *ModelSettingsService) SetEnabled(ctx context.Context, projectID, modelConfigID string, enabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	// Upsert: insert or update.
	var existingID string
	err := s.db.QueryRowContext(ctx,
		"SELECT id FROM project_model_settings WHERE project_id = ? AND model_config_id = ?",
		projectID, modelConfigID,
	).Scan(&existingID)

	if errors.Is(err, sql.ErrNoRows) {
		// Insert new setting.
		id := uuid.NewString()
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO project_model_settings (id, project_id, model_config_id, enabled, priority_order, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 0, ?, ?)`,
			id, projectID, modelConfigID, enabledInt, now, now,
		)
		if err != nil {
			return fmt.Errorf("inserting model setting: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking existing setting: %w", err)
	}

	// Update existing.
	_, err = s.db.ExecContext(ctx,
		"UPDATE project_model_settings SET enabled = ?, updated_at = ? WHERE id = ?",
		enabledInt, now, existingID,
	)
	if err != nil {
		return fmt.Errorf("updating model setting: %w", err)
	}
	return nil
}

// IsEnabled checks if a model is enabled for a project.
// Returns true if no explicit setting exists (default: enabled).
func (s *ModelSettingsService) IsEnabled(ctx context.Context, projectID, modelConfigID string) (bool, error) {
	var enabled int
	err := s.db.QueryRowContext(ctx,
		"SELECT enabled FROM project_model_settings WHERE project_id = ? AND model_config_id = ?",
		projectID, modelConfigID,
	).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil // Default: enabled.
	}
	if err != nil {
		return false, fmt.Errorf("checking model enabled: %w", err)
	}
	return enabled == 1, nil
}

// ListForProject returns all model settings for a project.
func (s *ModelSettingsService) ListForProject(ctx context.Context, projectID string) ([]*ProjectModelSetting, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, model_config_id, enabled, priority_order, created_at, updated_at
		 FROM project_model_settings
		 WHERE project_id = ?
		 ORDER BY priority_order ASC, created_at ASC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing model settings: %w", err)
	}
	defer rows.Close()

	var settings []*ProjectModelSetting
	for rows.Next() {
		s := &ProjectModelSetting{}
		var enabled int
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ModelConfigID, &enabled, &s.PriorityOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning model setting: %w", err)
		}
		s.Enabled = enabled == 1
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

// EnabledModelConfigIDs returns the IDs of all enabled models for a project.
func (s *ModelSettingsService) EnabledModelConfigIDs(ctx context.Context, projectID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT model_config_id FROM project_model_settings
		 WHERE project_id = ? AND enabled = 1
		 ORDER BY priority_order ASC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing enabled models: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning model config ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CheckQuorum verifies that at least one GPT-family and one Opus-family
// model are enabled for the project. Returns ErrQuorumNotMet if not satisfied.
func (s *ModelSettingsService) CheckQuorum(ctx context.Context, projectID string) error {
	var gptCount, opusCount int

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_model_settings pms
		 JOIN model_configs mc ON pms.model_config_id = mc.id
		 WHERE pms.project_id = ? AND pms.enabled = 1 AND mc.provider = 'openai'`,
		projectID,
	).Scan(&gptCount)
	if err != nil {
		return fmt.Errorf("checking GPT quorum: %w", err)
	}

	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_model_settings pms
		 JOIN model_configs mc ON pms.model_config_id = mc.id
		 WHERE pms.project_id = ? AND pms.enabled = 1 AND mc.provider = 'anthropic'`,
		projectID,
	).Scan(&opusCount)
	if err != nil {
		return fmt.Errorf("checking Opus quorum: %w", err)
	}

	if gptCount == 0 || opusCount == 0 {
		return ErrQuorumNotMet
	}
	return nil
}
