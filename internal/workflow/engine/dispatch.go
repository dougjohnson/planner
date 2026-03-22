package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// StageHandler is the interface for stage-specific execution logic.
type StageHandler interface {
	// Execute runs the stage logic for the given project and workflow run.
	Execute(ctx context.Context, projectID string, workflowRunID string) error
}

// StageHandlerFunc adapts a function to the StageHandler interface.
type StageHandlerFunc func(ctx context.Context, projectID string, workflowRunID string) error

func (f StageHandlerFunc) Execute(ctx context.Context, projectID string, workflowRunID string) error {
	return f(ctx, projectID, workflowRunID)
}

// Dispatcher routes stage execution requests to registered handlers.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]StageHandler
	logger   *slog.Logger
}

// NewDispatcher creates a new stage handler Dispatcher.
func NewDispatcher(logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]StageHandler),
		logger:   logger,
	}
}

// Register adds a handler for the given stage ID.
func (d *Dispatcher) Register(stageID string, handler StageHandler) {
	d.mu.Lock()
	d.handlers[stageID] = handler
	d.mu.Unlock()
	d.logger.Debug("stage handler registered", "stage", stageID)
}

// Dispatch executes the handler for the given stage. Returns an error if
// no handler is registered for the stage.
func (d *Dispatcher) Dispatch(ctx context.Context, stageID, projectID, workflowRunID string) error {
	d.mu.RLock()
	handler, ok := d.handlers[stageID]
	d.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler registered for stage %q", stageID)
	}

	d.logger.Info("dispatching stage handler",
		"stage", stageID,
		"project_id", projectID,
		"run_id", workflowRunID,
	)

	return handler.Execute(ctx, projectID, workflowRunID)
}

// HasHandler reports whether a handler is registered for the given stage.
func (d *Dispatcher) HasHandler(stageID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.handlers[stageID]
	return ok
}

// RegisteredStages returns the IDs of all stages with registered handlers.
func (d *Dispatcher) RegisteredStages() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	stages := make([]string, 0, len(d.handlers))
	for id := range d.handlers {
		stages = append(stages, id)
	}
	return stages
}
