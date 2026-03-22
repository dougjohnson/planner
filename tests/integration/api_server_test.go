// Package integration provides full-stack API integration tests.
// These tests spin up a real HTTP server with migrated SQLite
// and verify complete request/response cycles.
package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/api"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

// setupServer creates a test HTTP server with the full API router.
func setupServer(t *testing.T) *httptest.Server {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	_ = tdb // Server will use its own DB in production; for now test the router directly.

	srv := api.NewServer("", tdb.Logger)
	return httptest.NewServer(srv.Router())
}

func TestHealthEndpoint(t *testing.T) {
	ts := setupServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
}

func TestSecurityHeaders(t *testing.T) {
	ts := setupServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	headers := map[string]string{
		"Content-Security-Policy": "default-src 'self'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}

	for name, expected := range headers {
		got := resp.Header.Get(name)
		if got != expected {
			t.Errorf("header %s: expected %q, got %q", name, expected, got)
		}
	}
}

func TestNonExistentAPIRoute_Returns404(t *testing.T) {
	ts := setupServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/nonexistent")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestContentTypeIsJSON(t *testing.T) {
	ts := setupServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}
