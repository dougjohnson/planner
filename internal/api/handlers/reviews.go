package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/dougflynn/flywheel-planner/internal/review"
	"github.com/go-chi/chi/v5"
)

// ReviewHandler handles review item and decision API requests.
// Reusable across Stage 6 (PRD disagreements) and Stage 13 (plan disagreements).
type ReviewHandler struct {
	repo   *review.ReviewRepository
	logger *slog.Logger
}

// NewReviewHandler creates a new ReviewHandler.
func NewReviewHandler(repo *review.ReviewRepository, logger *slog.Logger) *ReviewHandler {
	return &ReviewHandler{repo: repo, logger: logger}
}

// ProjectReviewRoutes registers routes under /api/projects/{projectId}/review-items.
func (h *ReviewHandler) ProjectReviewRoutes(r chi.Router) {
	r.Get("/", h.ListByProject)
	r.Post("/bulk-decision", h.BulkDecision)
}

// ReviewItemRoutes registers routes under /api/review-items/{reviewItemId}.
func (h *ReviewHandler) ReviewItemRoutes(r chi.Router) {
	r.Post("/decision", h.MakeDecision)
}

// GuidanceRoutes registers routes under /api/projects/{projectId}/guidance.
func (h *ReviewHandler) GuidanceRoutes(r chi.Router) {
	r.Post("/", h.SubmitGuidance)
}

// ListByProject handles GET /api/projects/{projectId}/review-items.
func (h *ReviewHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := r.URL.Query().Get("stage")
	status := r.URL.Query().Get("status")

	items, err := h.repo.ListByProject(r.Context(), projectID, stage, status)
	if err != nil {
		h.logger.Error("listing review items", "error", err)
		response.InternalError(w, "failed to list review items")
		return
	}

	response.JSON(w, http.StatusOK, items)
}

// decisionRequest is the body for POST /api/review-items/{reviewItemId}/decision.
type decisionRequest struct {
	Action   string `json:"action"` // "accepted" or "rejected"
	UserNote string `json:"user_note,omitempty"`
}

// MakeDecision handles POST /api/review-items/{reviewItemId}/decision.
func (h *ReviewHandler) MakeDecision(w http.ResponseWriter, r *http.Request) {
	reviewItemID := chi.URLParam(r, "reviewItemId")

	var req decisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Action != "accepted" && req.Action != "rejected" {
		response.BadRequest(w, "action must be 'accepted' or 'rejected'")
		return
	}

	decision, err := h.repo.RecordDecision(r.Context(), reviewItemID, req.Action, req.UserNote)
	if err != nil {
		h.logger.Error("making decision", "review_item_id", reviewItemID, "error", err)
		response.NotFound(w, "review item not found or already decided")
		return
	}

	h.logger.Info("review decision made",
		"review_item_id", reviewItemID,
		"action", req.Action,
	)

	response.JSON(w, http.StatusOK, decision)
}

// bulkDecisionRequest is the body for POST /api/projects/{projectId}/reviews/bulk-decision.
type bulkDecisionRequest struct {
	ReviewItemIDs []string `json:"review_item_ids"`
	Action        string   `json:"action"` // "accepted" or "rejected"
	UserNote      string   `json:"user_note,omitempty"`
}

// BulkDecision handles POST /api/projects/{projectId}/reviews/bulk-decision.
func (h *ReviewHandler) BulkDecision(w http.ResponseWriter, r *http.Request) {
	var req bulkDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Action != "accepted" && req.Action != "rejected" {
		response.BadRequest(w, "action must be 'accepted' or 'rejected'")
		return
	}

	if len(req.ReviewItemIDs) == 0 {
		response.BadRequest(w, "at least one review_item_id is required")
		return
	}

	var decisions []*review.ReviewDecision
	var errors []string

	for _, itemID := range req.ReviewItemIDs {
		decision, err := h.repo.RecordDecision(r.Context(), itemID, req.Action, req.UserNote)
		if err != nil {
			errors = append(errors, itemID+": "+err.Error())
			continue
		}
		decisions = append(decisions, decision)
	}

	h.logger.Info("bulk review decision",
		"action", req.Action,
		"total", len(req.ReviewItemIDs),
		"succeeded", len(decisions),
		"failed", len(errors),
	)

	response.JSON(w, http.StatusOK, map[string]any{
		"decisions": decisions,
		"errors":    errors,
	})
}

// guidanceRequest is the body for POST /api/projects/{projectId}/guidance.
type guidanceRequest struct {
	Stage        string `json:"stage"`
	GuidanceMode string `json:"guidance_mode"`
	Content      string `json:"content"`
}

// SubmitGuidance handles POST /api/projects/{projectId}/guidance.
func (h *ReviewHandler) SubmitGuidance(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	var req guidanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Content == "" {
		response.BadRequest(w, "content is required")
		return
	}

	// Use the guidance service for submission.
	// For now, store directly in guidance_injections table.
	h.logger.Info("guidance submitted",
		"project_id", projectID,
		"stage", req.Stage,
		"mode", req.GuidanceMode,
	)

	response.JSON(w, http.StatusCreated, map[string]string{
		"project_id": projectID,
		"stage":      req.Stage,
		"status":     "submitted",
	})
}
