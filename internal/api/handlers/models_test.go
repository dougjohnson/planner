package handlers

import (
	"bytes"
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

func setupModelHandler(t *testing.T) (*ModelHandler, *chi.Mux) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	logger := testutil.NewTestLogger(t)
	h := NewModelHandler(tdb.DB, logger)

	r := chi.NewRouter()
	r.Route("/api/models", h.Routes)
	return h, r
}

func TestListModels_Empty(t *testing.T) {
	_, router := setupModelHandler(t)

	req := httptest.NewRequest("GET", "/api/models/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var env response.Envelope
	err := json.Unmarshal(w.Body.Bytes(), &env)
	require.NoError(t, err)
	assert.NotNil(t, env.Data)
}

func TestCreateModel(t *testing.T) {
	_, router := setupModelHandler(t)

	body := `{"provider":"openai","model_name":"gpt-4o"}`
	req := httptest.NewRequest("POST", "/api/models/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var env response.Envelope
	err := json.Unmarshal(w.Body.Bytes(), &env)
	require.NoError(t, err)

	data, _ := json.Marshal(env.Data)
	var model modelConfigResponse
	json.Unmarshal(data, &model)
	assert.Equal(t, "openai", model.Provider)
	assert.Equal(t, "gpt-4o", model.ModelName)
	assert.Equal(t, "standard", model.ReasoningMode)
	assert.True(t, model.EnabledGlobal)
	assert.False(t, model.HasCredential)
}

func TestCreateModel_InvalidProvider(t *testing.T) {
	_, router := setupModelHandler(t)

	body := `{"provider":"gemini","model_name":"gemini-pro"}`
	req := httptest.NewRequest("POST", "/api/models/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateModel_MissingFields(t *testing.T) {
	_, router := setupModelHandler(t)

	body := `{"provider":"openai"}`
	req := httptest.NewRequest("POST", "/api/models/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListModels_AfterCreate(t *testing.T) {
	_, router := setupModelHandler(t)

	// Create a model.
	body := `{"provider":"anthropic","model_name":"claude-opus-4-6","reasoning_mode":"extended"}`
	req := httptest.NewRequest("POST", "/api/models/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// List models.
	req = httptest.NewRequest("GET", "/api/models/", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var env response.Envelope
	json.Unmarshal(w.Body.Bytes(), &env)
	data, _ := json.Marshal(env.Data)
	var models []modelConfigResponse
	json.Unmarshal(data, &models)
	require.Len(t, models, 1)
	assert.Equal(t, "anthropic", models[0].Provider)
	assert.Equal(t, "extended", models[0].ReasoningMode)
}

func TestSetCredential_NeverEchosKey(t *testing.T) {
	_, router := setupModelHandler(t)

	// Create model first.
	body := `{"provider":"openai","model_name":"gpt-4o"}`
	req := httptest.NewRequest("POST", "/api/models/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var env response.Envelope
	json.Unmarshal(w.Body.Bytes(), &env)
	data, _ := json.Marshal(env.Data)
	var created modelConfigResponse
	json.Unmarshal(data, &created)

	// Set credential.
	credBody := `{"api_key":"sk-test-secret-key-12345"}`
	req = httptest.NewRequest("PUT", "/api/models/"+created.ID+"/credential", bytes.NewBufferString(credBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// The response must NEVER contain the raw key.
	assert.NotContains(t, w.Body.String(), "sk-test-secret-key-12345")
	assert.Contains(t, w.Body.String(), "credential_set")
}
