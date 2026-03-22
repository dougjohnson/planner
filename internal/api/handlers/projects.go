// Package handlers provides HTTP request handlers for the flywheel-planner API.
package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/dougflynn/flywheel-planner/internal/db/queries"
	"github.com/go-chi/chi/v5"
)

// ProjectHandler handles project CRUD API requests.
type ProjectHandler struct {
	repo   *queries.ProjectRepo
	logger *slog.Logger
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(repo *queries.ProjectRepo, logger *slog.Logger) *ProjectHandler {
	return &ProjectHandler{repo: repo, logger: logger}
}

// createProjectRequest is the request body for POST /api/projects.
type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Routes registers project routes on the given router.
func (h *ProjectHandler) Routes(r chi.Router) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{projectId}", h.GetByID)
	r.Patch("/{projectId}", h.Update)
	r.Post("/{projectId}/archive", h.Archive)
	r.Post("/{projectId}/resume", h.Resume)
}

// Create handles POST /api/projects.
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		response.BadRequest(w, "project name is required")
		return
	}

	project, err := h.repo.Create(r.Context(), name, req.Description)
	if err != nil {
		h.logger.Error("creating project", "error", err)
		response.InternalError(w, "failed to create project")
		return
	}

	h.logger.Info("project created", "id", project.ID, "name", name)
	response.JSON(w, http.StatusCreated, project)
}

// List handles GET /api/projects.
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	includeArchived := r.URL.Query().Get("include_archived") == "true"

	projects, err := h.repo.List(r.Context(), queries.ProjectFilter{
		IncludeArchived: includeArchived,
	})
	if err != nil {
		h.logger.Error("listing projects", "error", err)
		response.InternalError(w, "failed to list projects")
		return
	}

	response.JSON(w, http.StatusOK, projects)
}

// GetByID handles GET /api/projects/{projectId}.
func (h *ProjectHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")

	project, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(w, "project not found")
		} else {
			h.logger.Error("getting project", "id", id, "error", err)
			response.InternalError(w, "failed to get project")
		}
		return
	}

	response.JSON(w, http.StatusOK, project)
}

// updateProjectRequest is the request body for PATCH /api/projects/{projectId}.
type updateProjectRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// Update handles PATCH /api/projects/{projectId}.
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")

	var req updateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	updates := make(map[string]string)
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			response.BadRequest(w, "project name cannot be empty")
			return
		}
		updates["name"] = name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	project, err := h.repo.Update(r.Context(), id, updates)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(w, "project not found")
		} else {
			h.logger.Error("updating project", "id", id, "error", err)
			response.InternalError(w, "failed to update project")
		}
		return
	}

	response.JSON(w, http.StatusOK, project)
}

// Archive handles POST /api/projects/{projectId}/archive.
func (h *ProjectHandler) Archive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")

	if err := h.repo.Archive(r.Context(), id); err != nil {
		response.NotFound(w, "project not found or already archived")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

// Resume handles POST /api/projects/{projectId}/resume.
func (h *ProjectHandler) Resume(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")

	if err := h.repo.Resume(r.Context(), id); err != nil {
		response.NotFound(w, "project not found or not archived")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}
