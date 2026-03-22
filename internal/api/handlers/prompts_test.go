package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPromptHandler(t *testing.T) (*PromptHandler, *chi.Mux) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	logger := testutil.NewTestLogger(t)
	h := NewPromptHandler(tdb.DB, logger)

	// Seed test prompt.
	tdb.Exec(`INSERT INTO prompt_templates (id, name, version, stage, baseline_text, locked_status, created_at, updated_at)
		VALUES ('pt-1', 'GPT_PRD_SYNTHESIS_V1', 1, 'prd_synthesis', 'Synthesize the competing PRDs.', 'locked', datetime('now'), datetime('now'))`)
	tdb.Exec(`INSERT INTO prompt_templates (id, name, version, stage, baseline_text, locked_status, created_at, updated_at)
		VALUES ('pt-2', 'OPUS_PRD_INTEGRATION_V1', 1, 'prd_integration', 'Integrate the revisions.', 'locked', datetime('now'), datetime('now'))`)

	r := chi.NewRouter()
	r.Route("/api/prompts", h.Routes)
	r.Route("/api/workflow-runs", h.RunPromptRoutes)
	return h, r
}

func TestListPrompts(t *testing.T) {
	_, router := setupPromptHandler(t)

	req := httptest.NewRequest("GET", "/api/prompts/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var env response.Envelope
	err := json.Unmarshal(w.Body.Bytes(), &env)
	require.NoError(t, err)

	data, _ := json.Marshal(env.Data)
	var prompts []promptResponse
	json.Unmarshal(data, &prompts)
	assert.Len(t, prompts, 2)
}

func TestGetPrompt(t *testing.T) {
	_, router := setupPromptHandler(t)

	req := httptest.NewRequest("GET", "/api/prompts/pt-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var env response.Envelope
	json.Unmarshal(w.Body.Bytes(), &env)
	data, _ := json.Marshal(env.Data)
	var prompt promptResponse
	json.Unmarshal(data, &prompt)
	assert.Equal(t, "GPT_PRD_SYNTHESIS_V1", prompt.Name)
	assert.Equal(t, "locked", prompt.LockedStatus)
	assert.Contains(t, prompt.BaselineText, "Synthesize")
}

func TestGetPrompt_NotFound(t *testing.T) {
	_, router := setupPromptHandler(t)

	req := httptest.NewRequest("GET", "/api/prompts/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPromptRender_EmptyRuns(t *testing.T) {
	_, router := setupPromptHandler(t)

	req := httptest.NewRequest("GET", "/api/workflow-runs/nonexistent/prompt-render", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "renders")
}
