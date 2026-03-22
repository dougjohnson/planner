package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/dougflynn/flywheel-planner/internal/foundations"
	"github.com/go-chi/chi/v5"
)

// FoundationsHandler handles foundation artifact API requests.
type FoundationsHandler struct {
	logger *slog.Logger
}

// NewFoundationsHandler creates a new FoundationsHandler.
func NewFoundationsHandler(logger *slog.Logger) *FoundationsHandler {
	return &FoundationsHandler{logger: logger}
}

// Routes registers foundation routes on the given router.
func (h *FoundationsHandler) Routes(r chi.Router) {
	r.Post("/", h.Submit)
	r.Put("/", h.Update)
	r.Post("/lock", h.Lock)
}

// foundationsRequest is the request body for POST/PUT foundations.
type foundationsRequest struct {
	ProjectName          string   `json:"project_name"`
	Description          string   `json:"description"`
	TechStack            []string `json:"tech_stack"`
	ArchitectureDirection string  `json:"architecture_direction"`
	BuiltInGuides        []string `json:"built_in_guides"`
}

// Submit handles POST /api/projects/{projectId}/foundations.
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

	// Build foundations input.
	input := foundations.FoundationsInput{
		ProjectName:          req.ProjectName,
		Description:          req.Description,
		TechStack:            req.TechStack,
		ArchitectureDirection: req.ArchitectureDirection,
	}

	for _, guide := range req.BuiltInGuides {
		input.BuiltInGuides = append(input.BuiltInGuides, foundations.GuideReference{
			Name:     guide,
			Filename: guide + ".md",
			Source:   "built_in",
		})
	}

	// Generate AGENTS.md.
	agentsMD, err := foundations.AssembleAgentsMD(input)
	if err != nil {
		h.logger.Error("assembling AGENTS.md", "error", err)
		response.InternalError(w, "failed to assemble AGENTS.md")
		return
	}

	// Generate tech stack file.
	techStackInput := foundations.TechStackInput{
		ProjectName: req.ProjectName,
		Languages:   req.TechStack,
	}
	techStackMD, _ := foundations.GenerateTechStack(techStackInput)

	h.logger.Info("foundations submitted",
		"project_id", projectID,
		"tech_stack", req.TechStack,
		"guides", len(req.BuiltInGuides),
	)

	response.JSON(w, http.StatusCreated, map[string]any{
		"project_id":   projectID,
		"agents_md":    agentsMD,
		"tech_stack":   techStackMD,
		"guide_count":  len(input.AllGuides()),
		"status":       "submitted",
	})
}

// Update handles PUT /api/projects/{projectId}/foundations.
func (h *FoundationsHandler) Update(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	var req foundationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	input := foundations.FoundationsInput{
		ProjectName:          req.ProjectName,
		Description:          req.Description,
		TechStack:            req.TechStack,
		ArchitectureDirection: req.ArchitectureDirection,
	}

	agentsMD, err := foundations.AssembleAgentsMD(input)
	if err != nil {
		h.logger.Error("assembling AGENTS.md", "error", err)
		response.InternalError(w, "failed to assemble AGENTS.md")
		return
	}

	techStackInput2 := foundations.TechStackInput{
		ProjectName: req.ProjectName,
		Languages:   req.TechStack,
	}
	techStackMD, _ := foundations.GenerateTechStack(techStackInput2)

	h.logger.Info("foundations updated", "project_id", projectID)

	response.JSON(w, http.StatusOK, map[string]any{
		"project_id":   projectID,
		"agents_md":    agentsMD,
		"tech_stack":   techStackMD,
		"guide_count":  len(input.AllGuides()),
		"status":       "updated",
	})
}

// Lock handles POST /api/projects/{projectId}/foundations/lock.
func (h *FoundationsHandler) Lock(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	h.logger.Info("foundations locked", "project_id", projectID)

	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"status":     "locked",
	})
}
