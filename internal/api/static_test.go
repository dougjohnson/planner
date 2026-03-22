package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testSPALogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestSPA_ServesIndexHTML(t *testing.T) {
	srv := NewServer("", testSPALogger())
	staticFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>flywheel</html>")},
	}
	srv.RegisterSPA(staticFS)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET / status = %d, want 200", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "<html>flywheel</html>" {
		t.Errorf("body = %q", body)
	}
}

func TestSPA_ServesStaticAssets(t *testing.T) {
	srv := NewServer("", testSPALogger())
	staticFS := fstest.MapFS{
		"index.html":           {Data: []byte("<html></html>")},
		"assets/app.js":        {Data: []byte("console.log('app')")},
		"assets/style.css":     {Data: []byte("body{}")},
	}
	srv.RegisterSPA(staticFS)

	for _, path := range []string{"/assets/app.js", "/assets/style.css"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, w.Code)
		}
	}
}

func TestSPA_FallbackToIndex(t *testing.T) {
	srv := NewServer("", testSPALogger())
	staticFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>spa</html>")},
	}
	srv.RegisterSPA(staticFS)

	// SPA routes should all serve index.html.
	for _, path := range []string{"/projects", "/projects/abc123", "/settings", "/models"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, w.Code)
		}
		body, _ := io.ReadAll(w.Body)
		if string(body) != "<html>spa</html>" {
			t.Errorf("GET %s did not serve index.html, got %q", path, body)
		}
	}
}

func TestSPA_APIRoutesNotFallback(t *testing.T) {
	srv := NewServer("", testSPALogger())
	staticFS := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
	}
	srv.RegisterSPA(staticFS)

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /api/nonexistent status = %d, want 404", w.Code)
	}
}

func TestSPA_HealthStillWorks(t *testing.T) {
	srv := NewServer("", testSPALogger())
	staticFS := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
	}
	srv.RegisterSPA(staticFS)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/health status = %d, want 200", w.Code)
	}
}
