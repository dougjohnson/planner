package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/dougflynn/flywheel-planner/internal/events"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/go-chi/chi/v5"
)

// WorkflowHandler handles workflow API requests.
type WorkflowHandler struct {
	eventPublisher *events.Publisher
	logger         *slog.Logger
}

// NewWorkflowHandler creates a new WorkflowHandler.
func NewWorkflowHandler(pub *events.Publisher, logger *slog.Logger) *WorkflowHandler {
	return &WorkflowHandler{eventPublisher: pub, logger: logger}
}

// Routes registers workflow routes on the given router.
func (h *WorkflowHandler) Routes(r chi.Router) {
	r.Get("/", h.GetStatus)
	r.Post("/configure", h.Configure)

	r.Route("/stages/{stage}", func(r chi.Router) {
		r.Post("/start", h.StartStage)
		r.Post("/retry", h.RetryStage)
		r.Post("/continue", h.ContinueStage)
		r.Post("/pause", h.PauseStage)
		r.Post("/cancel", h.CancelStage)
	})
}

// StageActionRequest is the body for stage mutation endpoints.
type StageActionRequest struct {
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// WorkflowStatusResponse is the DTO for GET /workflow.
type WorkflowStatusResponse struct {
	ProjectID  string        `json:"project_id"`
	Stages     []StageInfo   `json:"stages"`
	EventCount int           `json:"event_count"`
}

// StageInfo describes a stage's current state.
type StageInfo struct {
	ID     string                `json:"id"`
	Name   string                `json:"name"`
	Status workflow.StageStatus  `json:"status"`
}

// GetStatus handles GET /api/projects/{projectId}/workflow.
func (h *WorkflowHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	// Build stage info from stage definitions.
	defs := workflow.AllStages()
	stages := make([]StageInfo, 0, len(defs))
	for _, d := range defs {
		stages = append(stages, StageInfo{
			ID:     d.ID,
			Name:   d.Name,
			Status: workflow.StageNotStarted, // TODO: load from DB once stage state persistence exists
		})
	}

	// Count events.
	eventCount := 0
	if h.eventPublisher != nil {
		evts, err := h.eventPublisher.ListByProject(r.Context(), projectID, 0)
		if err == nil {
			eventCount = len(evts)
		}
	}

	response.JSON(w, http.StatusOK, WorkflowStatusResponse{
		ProjectID:  projectID,
		Stages:     stages,
		EventCount: eventCount,
	})
}

// StartStage handles POST /api/projects/{projectId}/stages/{stage}/start.
func (h *WorkflowHandler) StartStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	// Validate stage exists.
	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	// Publish event.
	if h.eventPublisher != nil {
		h.eventPublisher.Publish(r.Context(), projectID, events.StageStarted, "", events.Payload{
			Stage:   stage,
			Message: "Stage started",
		})
	}

	h.logger.Info("stage started", "project_id", projectID, "stage", stage)
	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"action":     "started",
	})
}

// RetryStage handles POST .../retry.
func (h *WorkflowHandler) RetryStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	h.logger.Info("stage retry requested", "project_id", projectID, "stage", stage)
	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"action":     "retry",
	})
}

// ContinueStage handles POST .../continue.
func (h *WorkflowHandler) ContinueStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"action":     "continue",
	})
}

// PauseStage handles POST .../pause.
func (h *WorkflowHandler) PauseStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"action":     "paused",
	})
}

// CancelStage handles POST .../cancel.
func (h *WorkflowHandler) CancelStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"action":     "cancelled",
	})
}

// configureRequest is the body for POST .../configure.
type configureRequest struct {
	LoopCount      *int `json:"loop_count,omitempty"`
	PauseBetween   *int `json:"pause_between,omitempty"`
}

// Configure handles POST /api/projects/{projectId}/workflow/configure.
func (h *WorkflowHandler) Configure(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	var req configureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	h.logger.Info("workflow configured", "project_id", projectID, "loop_count", req.LoopCount)
	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"status":     "configured",
	})
}

// isValidStage checks if a stage ID exists in the stage definitions.
func isValidStage(stageID string) bool {
	for _, d := range workflow.AllStages() {
		if d.ID == stageID {
			return true
		}
	}
	return false
}
