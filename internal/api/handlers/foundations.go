package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/dougflynn/flywheel-planner/internal/foundations"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// FoundationsHandler handles foundation artifact API requests.
type FoundationsHandler struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewFoundationsHandler creates a new FoundationsHandler.
func NewFoundationsHandler(db *sql.DB, logger *slog.Logger) *FoundationsHandler {
	return &FoundationsHandler{db: db, logger: logger}
}

// Routes registers foundation routes on the given router.
func (h *FoundationsHandler) Routes(r chi.Router) {
	r.Get("/", h.Get)
	r.Post("/", h.Submit)
	r.Put("/", h.Update)
	r.Post("/lock", h.Lock)
}

// foundationsRequest is the request body for POST/PUT foundations.
type foundationsRequest struct {
	ProjectName           string   `json:"project_name"`
	Description           string   `json:"description"`
	TechStack             []string `json:"tech_stack"`
	ArchitectureDirection string   `json:"architecture_direction"`
	BuiltInGuides         []string `json:"built_in_guides"`
}

// foundationFile is the response shape for foundation files.
type foundationFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Source  string `json:"source"`
}

// Get handles GET /api/projects/{projectId}/foundations.
// Returns persisted foundation files.
func (h *FoundationsHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT original_filename, content_path, source_type
		 FROM project_inputs
		 WHERE project_id = ? AND role = 'foundation'
		 ORDER BY created_at ASC`, projectID)
	if err != nil {
		h.logger.Error("querying foundations", "error", err)
		response.InternalError(w, "failed to load foundations")
		return
	}
	defer rows.Close()

	files := make([]foundationFile, 0)
	for rows.Next() {
		var name, content, source string
		if err := rows.Scan(&name, &content, &source); err != nil {
			h.logger.Error("scanning foundation", "error", err)
			continue
		}
		files = append(files, foundationFile{
			Name:    name,
			Content: content,
			Source:  source,
		})
	}

	response.JSON(w, http.StatusOK, files)
}

// Submit handles POST /api/projects/{projectId}/foundations.
// Generates foundation artifacts, persists them, and returns them.
func (h *FoundationsHandler) Submit(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	var req foundationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if strings.TrimSpace(req.ProjectName) == "" {
		response.BadRequest(w, "project_name is required")
		return
	}

	files, err := h.generateFoundationFiles(req)
	if err != nil {
		h.logger.Error("generating foundations", "error", err)
		response.InternalError(w, "failed to generate foundation files")
		return
	}

	if err := h.persistFiles(r.Context(), projectID, files); err != nil {
		h.logger.Error("persisting foundations", "error", err)
		response.InternalError(w, "failed to save foundation files")
		return
	}

	h.logger.Info("foundations submitted",
		"project_id", projectID,
		"file_count", len(files),
	)

	response.JSON(w, http.StatusCreated, files)
}

// Update handles PUT /api/projects/{projectId}/foundations.
// Regenerates and replaces persisted foundation files.
func (h *FoundationsHandler) Update(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	var req foundationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	files, err := h.generateFoundationFiles(req)
	if err != nil {
		h.logger.Error("generating foundations", "error", err)
		response.InternalError(w, "failed to generate foundation files")
		return
	}

	// Delete old foundations before inserting new ones.
	_, err = h.db.ExecContext(r.Context(),
		`DELETE FROM project_inputs WHERE project_id = ? AND role = 'foundation'`, projectID)
	if err != nil {
		h.logger.Error("clearing old foundations", "error", err)
		response.InternalError(w, "failed to update foundation files")
		return
	}

	if err := h.persistFiles(r.Context(), projectID, files); err != nil {
		h.logger.Error("persisting foundations", "error", err)
		response.InternalError(w, "failed to save foundation files")
		return
	}

	h.logger.Info("foundations updated", "project_id", projectID)

	response.JSON(w, http.StatusOK, files)
}

// Lock handles POST /api/projects/{projectId}/foundations/lock.
// Advances the project past the foundations stage.
func (h *FoundationsHandler) Lock(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	// Update the project's current_stage to advance past foundations.
	_, err := h.db.ExecContext(r.Context(),
		`UPDATE projects SET current_stage = 'prd_intake', updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), projectID)
	if err != nil {
		h.logger.Error("locking foundations", "error", err)
		response.InternalError(w, "failed to lock foundations")
		return
	}

	h.logger.Info("foundations locked", "project_id", projectID)

	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"status":     "locked",
	})
}

// generateFoundationFiles creates the AGENTS.md, TECH_STACK.md, and
// ARCHITECTURE.md files from the user's input.
func (h *FoundationsHandler) generateFoundationFiles(req foundationsRequest) ([]foundationFile, error) {
	input := foundations.FoundationsInput{
		ProjectName:           req.ProjectName,
		Description:           req.Description,
		TechStack:             req.TechStack,
		ArchitectureDirection: req.ArchitectureDirection,
	}

	knownGuides := foundations.KnownStackGuides(req.TechStack)
	for _, g := range knownGuides {
		input.BuiltInGuides = append(input.BuiltInGuides, foundations.GuideReference{
			Name:     g.Name,
			Filename: g.Filename,
			Source:   "built_in",
		})
	}

	agentsMD, err := foundations.AssembleAgentsMD(input)
	if err != nil {
		return nil, fmt.Errorf("assembling AGENTS.md: %w", err)
	}

	techStackMD, err := foundations.GenerateTechStack(foundations.TechStackInput{
		ProjectName: req.ProjectName,
		Languages:   req.TechStack,
	})
	if err != nil {
		return nil, fmt.Errorf("generating tech stack: %w", err)
	}

	archContent := fmt.Sprintf("# Architecture Direction\n\n%s\n", req.ArchitectureDirection)

	return []foundationFile{
		{Name: "AGENTS.md", Content: agentsMD, Source: "generated"},
		{Name: "TECH_STACK.md", Content: techStackMD, Source: "generated"},
		{Name: "ARCHITECTURE.md", Content: archContent, Source: "generated"},
	}, nil
}

// persistFiles saves foundation files as project_inputs rows.
// Content is stored inline in the content_path column for simplicity.
func (h *FoundationsHandler) persistFiles(ctx context.Context, projectID string, files []foundationFile) error {
	now := time.Now().UTC().Format(time.RFC3339)
	for _, f := range files {
		_, err := h.db.ExecContext(ctx,
			`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, original_filename, detected_mime, created_at, updated_at)
			 VALUES (?, ?, 'foundation', ?, ?, ?, 'text/markdown', ?, ?)`,
			uuid.NewString(), projectID, f.Source, f.Content, f.Name, now, now)
		if err != nil {
			return fmt.Errorf("inserting %s: %w", f.Name, err)
		}
	}
	return nil
}
