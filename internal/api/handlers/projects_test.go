package handlers

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

	"github.com/dougflynn/flywheel-planner/internal/db/migrations"
	"github.com/dougflynn/flywheel-planner/internal/db/queries"
	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")
	if err := migrations.Run(context.Background(), db, testLogger()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func setupHandler(t *testing.T) (*ProjectHandler, *chi.Mux) {
	t.Helper()
	db := setupTestDB(t)
	repo := queries.NewProjectRepo(db)
	handler := NewProjectHandler(repo, testLogger())

	r := chi.NewRouter()
	r.Route("/api/projects", handler.Routes)
	return handler, r
}

func TestCreate_Success(t *testing.T) {
	_, router := setupHandler(t)

	body := `{"name": "My Project", "description": "A test"}`
	req := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var envelope map[string]any
	json.NewDecoder(w.Body).Decode(&envelope)
	project, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data envelope, got %v", envelope)
	}
	if project["Name"] != "My Project" {
		t.Errorf("expected name 'My Project', got %v", project["Name"])
	}
}

func TestCreate_EmptyName(t *testing.T) {
	_, router := setupHandler(t)

	body := `{"name": "", "description": "test"}`
	req := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreate_InvalidJSON(t *testing.T) {
	_, router := setupHandler(t)

	req := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestList_Empty(t *testing.T) {
	_, router := setupHandler(t)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestList_WithProjects(t *testing.T) {
	_, router := setupHandler(t)

	// Create two projects.
	for _, name := range []string{"Project A", "Project B"} {
		body := `{"name": "` + name + `"}`
		req := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	req := httptest.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var envelope map[string]any
	json.NewDecoder(w.Body).Decode(&envelope)
	data, _ := envelope["data"].([]any)
	if len(data) != 2 {
		t.Errorf("expected 2 projects, got %d", len(data))
	}
}

func TestGetByID(t *testing.T) {
	_, router := setupHandler(t)

	// Create project.
	body := `{"name": "Find Me"}`
	createReq := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(body))
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)

	var createEnv map[string]any
	json.NewDecoder(createW.Body).Decode(&createEnv)
	created := createEnv["data"].(map[string]any)
	id := created["ID"].(string)

	// Get by ID.
	req := httptest.NewRequest("GET", "/api/projects/"+id, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	_, router := setupHandler(t)

	req := httptest.NewRequest("GET", "/api/projects/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestArchiveAndResume(t *testing.T) {
	_, router := setupHandler(t)

	// Create.
	body := `{"name": "Archive Me"}`
	createReq := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(body))
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)

	var createEnv2 map[string]any
	json.NewDecoder(createW.Body).Decode(&createEnv2)
	created2 := createEnv2["data"].(map[string]any)
	id := created2["ID"].(string)

	// Archive.
	archReq := httptest.NewRequest("POST", "/api/projects/"+id+"/archive", nil)
	archW := httptest.NewRecorder()
	router.ServeHTTP(archW, archReq)
	if archW.Code != http.StatusOK {
		t.Errorf("archive: expected 200, got %d", archW.Code)
	}

	// Resume.
	resReq := httptest.NewRequest("POST", "/api/projects/"+id+"/resume", nil)
	resW := httptest.NewRecorder()
	router.ServeHTTP(resW, resReq)
	if resW.Code != http.StatusOK {
		t.Errorf("resume: expected 200, got %d", resW.Code)
	}
}
