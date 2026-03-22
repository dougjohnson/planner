package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/dougflynn/flywheel-planner/internal/events"
	"github.com/go-chi/chi/v5"
)

func setupWorkflowHandler(t *testing.T) (*WorkflowHandler, *chi.Mux) {
	t.Helper()
	db := setupTestDB(t)
	logger := testLogger()
	hub := sse.NewHub(logger)
	pub := events.NewPublisher(db, hub, logger)

	// Seed a project.
	db.ExecContext(context.Background(),
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')")

	handler := NewWorkflowHandler(db, pub, logger)

	r := chi.NewRouter()
	r.Route("/api/projects/{projectId}/workflow", handler.Routes)
	return handler, r
}

func TestGetWorkflowStatus(t *testing.T) {
	_, router := setupWorkflowHandler(t)

	req := httptest.NewRequest("GET", "/api/projects/p-1/workflow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var envelope map[string]any
	json.NewDecoder(w.Body).Decode(&envelope)
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data envelope, got %v", envelope)
	}
	// Response now includes "project" object matching frontend WorkflowStatusSchema.
	project, _ := data["project"].(map[string]any)
	if project == nil || project["id"] != "p-1" {
		t.Errorf("expected project.id 'p-1', got %v", project)
	}
	stages, _ := data["stages"].([]any)
	if len(stages) != 17 {
		t.Errorf("expected 17 stages, got %d", len(stages))
	}
}

func TestStartStage_ValidStage(t *testing.T) {
	_, router := setupWorkflowHandler(t)

	// Use the first stage from definitions.
	req := httptest.NewRequest("POST", "/api/projects/p-1/workflow/stages/foundations/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartStage_InvalidStage(t *testing.T) {
	_, router := setupWorkflowHandler(t)

	req := httptest.NewRequest("POST", "/api/projects/p-1/workflow/stages/nonexistent/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRetryStage(t *testing.T) {
	_, router := setupWorkflowHandler(t)

	req := httptest.NewRequest("POST", "/api/projects/p-1/workflow/stages/foundations/retry", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCancelStage(t *testing.T) {
	_, router := setupWorkflowHandler(t)

	req := httptest.NewRequest("POST", "/api/projects/p-1/workflow/stages/foundations/cancel", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
