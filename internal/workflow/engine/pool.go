// Package engine provides the workflow execution engine for flywheel-planner,
// including a bounded worker pool for concurrent model runs.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/dougflynn/flywheel-planner/internal/api/sse"
	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/dougflynn/flywheel-planner/internal/models/registry"
)

// SSE event type constants for workflow run events.
const (
	eventRunStarted   = "workflow:run_started"
	eventRunCompleted = "workflow:run_completed"
	eventRunFailed    = "workflow:run_failed"
)

// RunRequest describes a model execution to be submitted to the worker pool.
type RunRequest struct {
	// ProjectID is used for SSE event routing.
	ProjectID string
	// WorkflowRunID links this execution to a workflow run record.
	WorkflowRunID string
	// Provider is the target provider for this run.
	Provider models.ProviderName
	// Session is the model request to execute.
	Session models.SessionRequest
}

// RunResponse holds the result of a completed model execution.
type RunResponse struct {
	// Request is the original request for correlation.
	Request RunRequest
	// Response is the model's response (nil on error).
	Response *models.SessionResponse
	// Error is non-nil if the execution failed.
	Error error
}

// Pool is a bounded worker pool that executes model runs concurrently.
// It limits the number of simultaneous executions and supports cancellation
// via context. Workers publish SSE progress events as runs start and complete.
type Pool struct {
	registry *registry.Registry
	sseHub   *sse.Hub
	logger   *slog.Logger
	sem      chan struct{} // semaphore for bounded concurrency
	maxSize  int
}

// NewPool creates a worker pool with the given maximum concurrency.
// Pass maxConcurrency <= 0 to use a default of 4.
func NewPool(reg *registry.Registry, hub *sse.Hub, logger *slog.Logger, maxConcurrency int) *Pool {
	if maxConcurrency <= 0 {
		maxConcurrency = 4
	}
	return &Pool{
		registry: reg,
		sseHub:   hub,
		logger:   logger,
		sem:      make(chan struct{}, maxConcurrency),
		maxSize:  maxConcurrency,
	}
}

// Submit executes a single model run, blocking until a worker slot is available
// or the context is cancelled. It publishes SSE events for run start/completion.
func (p *Pool) Submit(ctx context.Context, req RunRequest) (*RunResponse, error) {
	// Acquire semaphore slot.
	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("worker pool: context cancelled while waiting for slot: %w", ctx.Err())
	}

	return p.execute(ctx, req), nil
}

// SubmitAll executes multiple model runs concurrently, bounded by pool size.
// All runs start as soon as slots are available. Returns when all complete.
// Individual run errors are captured in each RunResponse, not returned as the
// function error (which is only for pool-level failures).
func (p *Pool) SubmitAll(ctx context.Context, requests []RunRequest) ([]RunResponse, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	results := make([]RunResponse, len(requests))
	var wg sync.WaitGroup

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, r RunRequest) {
			defer wg.Done()

			// Acquire semaphore slot.
			select {
			case p.sem <- struct{}{}:
				defer func() { <-p.sem }()
			case <-ctx.Done():
				results[idx] = RunResponse{
					Request: r,
					Error:   fmt.Errorf("context cancelled while waiting for slot: %w", ctx.Err()),
				}
				return
			}

			results[idx] = *p.execute(ctx, r)
		}(i, req)
	}

	wg.Wait()
	return results, nil
}

// MaxConcurrency returns the pool's maximum concurrency limit.
func (p *Pool) MaxConcurrency() int {
	return p.maxSize
}

// ActiveWorkers returns the number of currently active workers.
func (p *Pool) ActiveWorkers() int {
	return len(p.sem)
}

// execute runs a single model request through the registry and records the result.
func (p *Pool) execute(ctx context.Context, req RunRequest) *RunResponse {
	p.logger.Debug("worker pool: starting run",
		"project_id", req.ProjectID,
		"run_id", req.WorkflowRunID,
		"provider", req.Provider,
	)

	// Publish SSE event: run started.
	if p.sseHub != nil {
		p.sseHub.Publish(req.ProjectID, eventRunStarted, map[string]string{
			"workflow_run_id": req.WorkflowRunID,
			"provider":        string(req.Provider),
		})
	}

	// Dispatch to the provider via the registry.
	resp, err := p.registry.Dispatch(ctx, req.Provider, req.Session)

	result := &RunResponse{
		Request:  req,
		Response: resp,
		Error:    err,
	}

	// Publish SSE event: run completed or failed.
	if p.sseHub != nil {
		if err != nil {
			p.sseHub.Publish(req.ProjectID, eventRunFailed, map[string]string{
				"workflow_run_id": req.WorkflowRunID,
				"provider":        string(req.Provider),
				"error":           err.Error(),
			})
		} else {
			p.sseHub.Publish(req.ProjectID, eventRunCompleted, map[string]string{
				"workflow_run_id": req.WorkflowRunID,
				"provider":        string(req.Provider),
			})
		}
	}

	if err != nil {
		p.logger.Warn("worker pool: run failed",
			"project_id", req.ProjectID,
			"run_id", req.WorkflowRunID,
			"provider", req.Provider,
			"error", err,
		)
	} else if resp != nil {
		p.logger.Debug("worker pool: run completed",
			"project_id", req.ProjectID,
			"run_id", req.WorkflowRunID,
			"provider", req.Provider,
			"tokens", resp.Usage.TotalTokens,
		)
	} else {
		// Provider returned (nil, nil) — contract violation. Log but don't panic.
		p.logger.Warn("worker pool: provider returned nil response without error",
			"project_id", req.ProjectID,
			"run_id", req.WorkflowRunID,
			"provider", req.Provider,
		)
		result.Error = fmt.Errorf("provider %s returned nil response", req.Provider)
	}

	return result
}
