// Package api provides the HTTP server for the flywheel-planner application.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/api/handlers"
	"github.com/dougflynn/flywheel-planner/internal/api/middleware"
	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

const (
	// DefaultAddr is the default listen address (loopback only).
	DefaultAddr = "127.0.0.1:7432"

	// shutdownTimeout is the grace period for in-flight requests during shutdown.
	shutdownTimeout = 10 * time.Second
)

// Server is the flywheel-planner HTTP server.
type Server struct {
	httpServer *http.Server
	router     *chi.Mux
	logger     *slog.Logger
}

// NewServer creates a new Server bound to the given address.
// Pass "" for addr to use DefaultAddr.
func NewServer(addr string, logger *slog.Logger) *Server {
	if addr == "" {
		addr = DefaultAddr
	}

	r := chi.NewRouter()

	// Global middleware stack.
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.StructuredLogger(logger))
	r.Use(middleware.SecurityHeaders)
	r.Use(chimw.Recoverer)

	s := &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           r,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
			BaseContext: func(_ net.Listener) context.Context {
				return context.Background()
			},
		},
		router: r,
		logger: logger,
	}

	s.routes()
	return s
}

// routes registers all route groups on the chi router.
func (s *Server) routes() {
	s.router.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)

		// Future route groups will be registered here:
		// r.Route("/projects", func(r chi.Router) { ... })
		// r.Route("/models", func(r chi.Router) { ... })
		// r.Route("/prompts", func(r chi.Router) { ... })
		// r.Route("/exports", func(r chi.Router) { ... })
	})
}

// handleHealth responds with a simple health check.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok"}`)
}

// ListenAndServe starts the HTTP server. It blocks until the server stops.
func (s *Server) ListenAndServe() error {
	s.logger.Info("http server listening", "addr", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("http listen: %w", err)
}

// Shutdown gracefully stops the server, waiting for in-flight requests.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	return nil
}

// Router returns the underlying chi.Mux for testing or extension.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// MountProjectRoutes registers project CRUD routes under /api/projects.
func (s *Server) MountProjectRoutes(h *handlers.ProjectHandler) {
	s.router.Route("/api/projects", h.Routes)
}

// MountWorkflowRoutes registers workflow routes under /api/projects/{projectId}/workflow.
func (s *Server) MountWorkflowRoutes(h *handlers.WorkflowHandler) {
	s.router.Route("/api/projects/{projectId}/workflow", h.Routes)
}

// MountModelRoutes registers model config routes under /api/models.
func (s *Server) MountModelRoutes(h *handlers.ModelHandler) {
	s.router.Route("/api/models", h.Routes)
}

// MountPromptRoutes registers prompt inspection routes under /api/prompts
// and prompt render routes under /api/workflow-runs.
func (s *Server) MountPromptRoutes(h *handlers.PromptHandler) {
	s.router.Route("/api/prompts", h.Routes)
	s.router.Route("/api/workflow-runs", h.RunPromptRoutes)
}

// MountFoundationsRoutes registers foundations routes under /api/projects/{projectId}/foundations.
func (s *Server) MountFoundationsRoutes(h *handlers.FoundationsHandler) {
	s.router.Route("/api/projects/{projectId}/foundations", h.Routes)
}

// MountSSE registers the SSE endpoint for real-time events.
func (s *Server) MountSSE(hub *sse.Hub) {
	s.router.Get("/api/projects/{projectId}/events", func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "projectId")
		hub.ServeHTTP(w, r, projectID)
	})
}
