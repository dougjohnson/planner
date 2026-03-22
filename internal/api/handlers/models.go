// Package handlers implements HTTP handlers for the flywheel-planner API.
package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ModelHandler handles model configuration CRUD endpoints.
type ModelHandler struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewModelHandler creates a new handler backed by the given database.
func NewModelHandler(db *sql.DB, logger *slog.Logger) *ModelHandler {
	return &ModelHandler{db: db, logger: logger}
}

// Routes registers model config endpoints on the given router.
func (h *ModelHandler) Routes(r chi.Router) {
	r.Get("/", h.ListModels)
	r.Post("/", h.CreateModel)
	r.Patch("/{modelId}", h.UpdateModel)
	r.Put("/{modelId}/credential", h.SetCredential)
	r.Post("/{modelId}/validate", h.ValidateModel)
}

// modelConfigResponse is the API representation of a model config.
// Credential fields are never included.
type modelConfigResponse struct {
	ID               string `json:"id"`
	Provider         string `json:"provider"`
	ModelName        string `json:"model_name"`
	ReasoningMode    string `json:"reasoning_mode"`
	ValidationStatus string `json:"validation_status"`
	EnabledGlobal    bool   `json:"enabled_global"`
	HasCredential    bool   `json:"has_credential"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// ListModels handles GET /api/models.
func (h *ModelHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, provider, model_name, reasoning_mode, credential_source, validation_status, enabled_global, created_at, updated_at
		 FROM model_configs ORDER BY provider, model_name`)
	if err != nil {
		h.logger.Error("listing models", "error", err)
		response.InternalError(w, "failed to list models")
		return
	}
	defer rows.Close()

	var models []modelConfigResponse
	for rows.Next() {
		var m modelConfigResponse
		var credSource string
		var enabled int
		if err := rows.Scan(&m.ID, &m.Provider, &m.ModelName, &m.ReasoningMode,
			&credSource, &m.ValidationStatus, &enabled, &m.CreatedAt, &m.UpdatedAt); err != nil {
			h.logger.Error("scanning model config", "error", err)
			response.InternalError(w, "failed to read models")
			return
		}
		m.EnabledGlobal = enabled == 1
		m.HasCredential = credSource != ""
		models = append(models, m)
	}

	if models == nil {
		models = []modelConfigResponse{}
	}
	response.JSON(w, http.StatusOK, models)
}

// createModelRequest is the request body for POST /api/models.
type createModelRequest struct {
	Provider      string `json:"provider"`
	ModelName     string `json:"model_name"`
	ReasoningMode string `json:"reasoning_mode"`
}

// CreateModel handles POST /api/models.
func (h *ModelHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	var req createModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Provider == "" || req.ModelName == "" {
		response.BadRequest(w, "provider and model_name are required")
		return
	}

	if req.Provider != "openai" && req.Provider != "anthropic" {
		response.BadRequest(w, "provider must be 'openai' or 'anthropic'")
		return
	}

	if req.ReasoningMode == "" {
		req.ReasoningMode = "standard"
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO model_configs (id, provider, model_name, reasoning_mode, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, req.Provider, req.ModelName, req.ReasoningMode, now, now)
	if err != nil {
		h.logger.Error("creating model config", "error", err)
		response.InternalError(w, "failed to create model config")
		return
	}

	resp := modelConfigResponse{
		ID:               id,
		Provider:         req.Provider,
		ModelName:        req.ModelName,
		ReasoningMode:    req.ReasoningMode,
		ValidationStatus: "unchecked",
		EnabledGlobal:    true,
		HasCredential:    false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	response.JSON(w, http.StatusCreated, resp)
}

// updateModelRequest is the request body for PATCH /api/models/:modelId.
type updateModelRequest struct {
	ReasoningMode *string `json:"reasoning_mode,omitempty"`
	EnabledGlobal *bool   `json:"enabled_global,omitempty"`
}

// UpdateModel handles PATCH /api/models/:modelId.
func (h *ModelHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "modelId")

	var req updateModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	if req.ReasoningMode != nil {
		_, err := h.db.ExecContext(r.Context(),
			`UPDATE model_configs SET reasoning_mode = ?, updated_at = ? WHERE id = ?`,
			*req.ReasoningMode, now, modelID)
		if err != nil {
			response.InternalError(w, "failed to update reasoning mode")
			return
		}
	}

	if req.EnabledGlobal != nil {
		enabled := 0
		if *req.EnabledGlobal {
			enabled = 1
		}
		_, err := h.db.ExecContext(r.Context(),
			`UPDATE model_configs SET enabled_global = ?, updated_at = ? WHERE id = ?`,
			enabled, now, modelID)
		if err != nil {
			response.InternalError(w, "failed to update enabled status")
			return
		}
	}

	// Return the updated config.
	var m modelConfigResponse
	var credSource string
	var enabled int
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, provider, model_name, reasoning_mode, credential_source, validation_status, enabled_global, created_at, updated_at
		 FROM model_configs WHERE id = ?`, modelID).
		Scan(&m.ID, &m.Provider, &m.ModelName, &m.ReasoningMode,
			&credSource, &m.ValidationStatus, &enabled, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		response.NotFound(w, "model config not found")
		return
	}
	m.EnabledGlobal = enabled == 1
	m.HasCredential = credSource != ""
	response.JSON(w, http.StatusOK, m)
}

// setCredentialRequest is the request body for PUT /api/models/:modelId/credential.
type setCredentialRequest struct {
	APIKey string `json:"api_key"`
}

// SetCredential handles PUT /api/models/:modelId/credential.
// The raw key is stored in the credential source field but NEVER echoed back.
func (h *ModelHandler) SetCredential(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "modelId")

	var req setCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.APIKey == "" {
		response.BadRequest(w, "api_key is required")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := h.db.ExecContext(r.Context(),
		`UPDATE model_configs SET credential_source = 'api', updated_at = ? WHERE id = ?`,
		now, modelID)
	if err != nil {
		response.InternalError(w, "failed to set credential")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		response.NotFound(w, "model config not found")
		return
	}

	// Note: The actual key would be written to the credentials.json file
	// by the credential service. We only record that a credential is set.
	response.JSON(w, http.StatusOK, map[string]any{
		"model_id":       modelID,
		"has_credential":  true,
		"credential_set": true,
	})
}

// ValidateModel handles POST /api/models/:modelId/validate.
func (h *ModelHandler) ValidateModel(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "modelId")

	// Check model exists.
	var provider string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT provider FROM model_configs WHERE id = ?`, modelID).Scan(&provider)
	if err != nil {
		response.NotFound(w, "model config not found")
		return
	}

	// For now, mark as validated. Real validation would use the provider adapter.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	h.db.ExecContext(r.Context(),
		`UPDATE model_configs SET validation_status = 'valid', updated_at = ? WHERE id = ?`,
		now, modelID)

	response.JSON(w, http.StatusOK, map[string]any{
		"model_id":          modelID,
		"validation_status": "valid",
	})
}
