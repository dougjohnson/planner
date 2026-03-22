package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestNewServer_DefaultAddr(t *testing.T) {
	srv := NewServer("", testLogger())
	if srv.httpServer.Addr != DefaultAddr {
		t.Errorf("expected addr %q, got %q", DefaultAddr, srv.httpServer.Addr)
	}
}

func TestNewServer_CustomAddr(t *testing.T) {
	addr := "127.0.0.1:9999"
	srv := NewServer(addr, testLogger())
	if srv.httpServer.Addr != addr {
		t.Errorf("expected addr %q, got %q", addr, srv.httpServer.Addr)
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := NewServer("", testLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
}

func TestSecurityHeaders(t *testing.T) {
	srv := NewServer("", testLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	tests := []struct {
		header   string
		expected string
	}{
		{"Content-Security-Policy", "default-src 'self'"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "no-referrer"},
	}

	for _, tc := range tests {
		got := resp.Header.Get(tc.header)
		if got != tc.expected {
			t.Errorf("header %s: expected %q, got %q", tc.header, tc.expected, got)
		}
	}
}

func TestNonExistentRoute_404(t *testing.T) {
	srv := NewServer("", testLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/nonexistent")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
