package api

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
)

// RegisterSPA configures the router to serve embedded frontend static assets
// for any non-API route, with SPA fallback to index.html for client-side routing.
//
// In production, the frontend build output is embedded via embed.FS.
// In development, the Vite dev server handles frontend requests via proxy.
func (s *Server) RegisterSPA(staticFS fs.FS) {
	// Read index.html once at startup for the SPA fallback.
	indexHTML, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		s.logger.Error("SPA index.html not found in embedded FS", "error", err)
		return
	}

	// Create a file server for static assets (JS, CSS, images).
	fileServer := http.FileServer(http.FS(staticFS))

	s.router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		// Don't serve SPA for API routes that weren't matched.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"error":{"code":"not_found","message":"endpoint not found"}}`)
			return
		}

		// Try to serve the exact file from the embedded FS.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		f, err := staticFS.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for React Router to handle.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(indexHTML)
	})
}
