package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/api"
	"github.com/dougflynn/flywheel-planner/internal/api/handlers"
	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/dougflynn/flywheel-planner/internal/db/migrations"
	"github.com/dougflynn/flywheel-planner/internal/db/queries"
	"github.com/dougflynn/flywheel-planner/internal/events"
	_ "modernc.org/sqlite"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func setupIntegration(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	ctx := context.Background()
	logger := testLogger()
	if err := migrations.Run(ctx, db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	// Set up services.
	projectRepo := queries.NewProjectRepo(db)
	hub := sse.NewHub(logger)
	eventPub := events.NewPublisher(db, hub, logger)

	// Build server.
	srv := api.NewServer("", logger)
	srv.MountProjectRoutes(handlers.NewProjectHandler(projectRepo, logger))
	srv.MountWorkflowRoutes(handlers.NewWorkflowHandler(db, eventPub, logger))
	srv.MountSSE(hub)

	ts := httptest.NewServer(srv.Router())
	t.Cleanup(func() {
		ts.Close()
		db.Close()
	})

	return ts, db
}

func TestIntegration_CreateProject_GetWorkflowStatus(t *testing.T) {
	ts, _ := setupIntegration(t)

	// Create project.
	body := `{"name": "Integration Test Project"}`
	resp, err := http.Post(ts.URL+"/api/projects", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var createResp map[string]any
	json.NewDecoder(resp.Body).Decode(&createResp)
	data := createResp["data"].(map[string]any)
	projectID := data["id"].(string)

	// Get workflow status.
	resp2, err := http.Get(ts.URL + "/api/projects/" + projectID + "/workflow")
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var wfResp map[string]any
	json.NewDecoder(resp2.Body).Decode(&wfResp)
	wfData := wfResp["data"].(map[string]any)
	stages := wfData["stages"].([]any)
	if len(stages) == 0 {
		t.Error("expected non-empty stages in workflow status")
	}
}

func TestIntegration_StartStage_PublishesEvent(t *testing.T) {
	ts, db := setupIntegration(t)
	ctx := context.Background()

	// Create project first.
	now := "2026-01-01T00:00:00Z"
	db.ExecContext(ctx,
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)

	// Start a stage.
	resp, err := http.Post(ts.URL+"/api/projects/p-1/workflow/stages/foundations/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start stage: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify event was persisted.
	var count int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workflow_events WHERE project_id = 'p-1'").Scan(&count)
	if count == 0 {
		t.Error("expected at least one event after starting stage")
	}
}

func TestIntegration_InvalidStage_Returns400(t *testing.T) {
	ts, db := setupIntegration(t)
	ctx := context.Background()

	now := "2026-01-01T00:00:00Z"
	db.ExecContext(ctx,
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)

	resp, err := http.Post(ts.URL+"/api/projects/p-1/workflow/stages/nonexistent/start", "application/json", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIntegration_HealthEndpoint(t *testing.T) {
	ts, _ := setupIntegration(t)

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_SecurityHeaders(t *testing.T) {
	ts, _ := setupIntegration(t)

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
}
