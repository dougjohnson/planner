package handlers

import (
	"database/sql"
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
	db             *sql.DB
	eventPublisher *events.Publisher
	logger         *slog.Logger
}

// NewWorkflowHandler creates a new WorkflowHandler.
func NewWorkflowHandler(db *sql.DB, pub *events.Publisher, logger *slog.Logger) *WorkflowHandler {
	return &WorkflowHandler{db: db, eventPublisher: pub, logger: logger}
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
	ProjectID    string      `json:"project_id"`
	CurrentStage string      `json:"current_stage"`
	Stages       []StageInfo `json:"stages"`
	EventCount   int         `json:"event_count"`
}

// StageInfo describes a stage's current state.
type StageInfo struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Number   int                  `json:"number"`
	Status   workflow.StageStatus `json:"status"`
	RunCount int                  `json:"run_count"`
}

// stageRunInfo holds aggregated run data for a single stage.
type stageRunInfo struct {
	runCount     int
	latestStatus string
}

// GetStatus handles GET /api/projects/{projectId}/workflow.
func (h *WorkflowHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	ctx := r.Context()

	// Load the project's current stage.
	var currentStage string
	err := h.db.QueryRowContext(ctx,
		`SELECT current_stage FROM projects WHERE id = ?`, projectID).Scan(&currentStage)
	if err != nil {
		response.NotFound(w, "project not found")
		return
	}

	// Load per-stage run counts and latest run status.
	stageRuns := make(map[string]stageRunInfo)

	rows, err := h.db.QueryContext(ctx, `
		SELECT stage, COUNT(*) as run_count,
			(SELECT status FROM workflow_runs wr2
			 WHERE wr2.project_id = wr.project_id AND wr2.stage = wr.stage
			 ORDER BY wr2.created_at DESC LIMIT 1) as latest_status
		FROM workflow_runs wr
		WHERE wr.project_id = ?
		GROUP BY stage`, projectID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var stage string
			var info stageRunInfo
			if rows.Scan(&stage, &info.runCount, &info.latestStatus) == nil {
				stageRuns[stage] = info
			}
		}
	}

	// Build stage info with real statuses.
	defs := workflow.AllStages()
	reachedCurrent := false
	stages := make([]StageInfo, 0, len(defs))

	for _, d := range defs {
		status := computeStageStatus(d.ID, currentStage, stageRuns, &reachedCurrent)
		info := StageInfo{
			ID:     d.ID,
			Name:   d.Name,
			Number: d.PRDNumber,
			Status: status,
		}
		if ri, ok := stageRuns[d.ID]; ok {
			info.RunCount = ri.runCount
		}
		stages = append(stages, info)
	}

	// Count events.
	eventCount := 0
	if h.eventPublisher != nil {
		evts, err := h.eventPublisher.ListByProject(ctx, projectID, 0)
		if err == nil {
			eventCount = len(evts)
		}
	}

	response.JSON(w, http.StatusOK, WorkflowStatusResponse{
		ProjectID:    projectID,
		CurrentStage: currentStage,
		Stages:       stages,
		EventCount:   eventCount,
	})
}

// computeStageStatus derives a stage's display status from the project's
// current position and any workflow runs for that stage.
func computeStageStatus(
	stageID string,
	currentStage string,
	stageRuns map[string]stageRunInfo,
	reachedCurrent *bool,
) workflow.StageStatus {
	// If we have run data for this stage, use the latest run status.
	if ri, ok := stageRuns[stageID]; ok {
		switch ri.latestStatus {
		case "completed":
			return workflow.StageCompleted
		case "running":
			return workflow.StageRunning
		case "failed":
			return workflow.StageRetryableFailure
		case "needs_review":
			return workflow.StageAwaitingUser
		case "interrupted":
			return workflow.StageRetryableFailure
		}
	}

	// No runs — derive from position relative to current stage.
	if currentStage == "" {
		// Project just created, still in foundations.
		if stageID == "foundations" {
			return workflow.StageReady
		}
		return workflow.StageNotStarted
	}

	if stageID == currentStage {
		*reachedCurrent = true
		return workflow.StageReady
	}

	if !*reachedCurrent {
		// Before current stage — completed (we passed through it).
		return workflow.StageCompleted
	}

	// After current stage — not started yet.
	return workflow.StageNotStarted
}

// StartStage handles POST /api/projects/{projectId}/stages/{stage}/start.
func (h *WorkflowHandler) StartStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

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
	LoopCount    *int `json:"loop_count,omitempty"`
	PauseBetween *int `json:"pause_between,omitempty"`
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
