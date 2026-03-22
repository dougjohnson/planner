// Package registry provides the provider registry and model dispatcher for
// flywheel-planner. The registry manages provider adapter lifecycle and routes
// model execution requests to the correct adapter.
package registry

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

const (
	// degradedThreshold is the number of consecutive failures before marking
	// a provider as degraded.
	degradedThreshold = 3
)

// ProviderHealth tracks the operational status of a registered provider.
type ProviderHealth struct {
	Provider           models.ProviderName `json:"provider"`
	Registered         bool                `json:"registered"`
	Degraded           bool                `json:"degraded"`
	LastSuccessAt      *time.Time          `json:"last_success_at,omitempty"`
	LastFailureAt      *time.Time          `json:"last_failure_at,omitempty"`
	ConsecutiveFailures int               `json:"consecutive_failures"`
}

// entry is an internal record for a registered provider.
type entry struct {
	provider            models.Provider
	health              ProviderHealth
}

// Registry manages provider adapters and dispatches execution requests.
// It is safe for concurrent use from multiple goroutines.
type Registry struct {
	mu       sync.RWMutex
	entries  map[models.ProviderName]*entry
	logger   *slog.Logger
}

// New creates a new provider registry.
func New(logger *slog.Logger) *Registry {
	return &Registry{
		entries: make(map[models.ProviderName]*entry),
		logger:  logger,
	}
}

// Register adds a provider adapter to the registry. If a provider with
// the same name is already registered, it is replaced.
func (r *Registry) Register(provider models.Provider) {
	name := provider.Name()

	r.mu.Lock()
	r.entries[name] = &entry{
		provider: provider,
		health: ProviderHealth{
			Provider:   name,
			Registered: true,
		},
	}
	r.mu.Unlock()

	r.logger.Info("provider registered", "provider", name,
		"models", len(provider.Models()))
}

// Unregister removes a provider from the registry.
func (r *Registry) Unregister(name models.ProviderName) {
	r.mu.Lock()
	delete(r.entries, name)
	r.mu.Unlock()

	r.logger.Info("provider unregistered", "provider", name)
}

// Get returns the provider adapter for the given name, or nil if not registered.
func (r *Registry) Get(name models.ProviderName) models.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.entries[name]
	if !ok {
		return nil
	}
	return e.provider
}

// IsRegistered reports whether a provider is registered.
func (r *Registry) IsRegistered(name models.ProviderName) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.entries[name]
	return ok
}

// RegisteredProviders returns the names of all registered providers.
func (r *Registry) RegisteredProviders() []models.ProviderName {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]models.ProviderName, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	return names
}

// Health returns the health status of a registered provider.
func (r *Registry) Health(name models.ProviderName) (ProviderHealth, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.entries[name]
	if !ok {
		return ProviderHealth{}, false
	}
	return e.health, true
}

// AllHealth returns health status for all registered providers.
func (r *Registry) AllHealth() []ProviderHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ProviderHealth, 0, len(r.entries))
	for _, e := range r.entries {
		result = append(result, e.health)
	}
	return result
}

// Dispatch sends a request to the specified provider and returns the response.
// It records health metrics after each call.
func (r *Registry) Dispatch(ctx context.Context, providerName models.ProviderName, req models.SessionRequest) (*models.SessionResponse, error) {
	r.mu.RLock()
	e, ok := r.entries[providerName]
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("provider %s is not registered", providerName)
	}
	provider := e.provider
	r.mu.RUnlock()

	resp, err := provider.Execute(ctx, req)

	// Record health after execution.
	r.recordResult(providerName, err)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// DispatchAll sends a request to all registered providers concurrently and
// returns a slice of results. Used for parallel generation stages (3, 10).
// Errors from individual providers are collected but do not prevent other
// providers from completing.
func (r *Registry) DispatchAll(ctx context.Context, req models.SessionRequest) ([]DispatchResult, error) {
	r.mu.RLock()
	providers := make([]models.Provider, 0, len(r.entries))
	names := make([]models.ProviderName, 0, len(r.entries))
	for name, e := range r.entries {
		providers = append(providers, e.provider)
		names = append(names, name)
	}
	r.mu.RUnlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers registered")
	}

	results := make([]DispatchResult, len(providers))
	var wg sync.WaitGroup

	for i, p := range providers {
		wg.Add(1)
		go func(idx int, prov models.Provider, name models.ProviderName) {
			defer wg.Done()
			resp, err := prov.Execute(ctx, req)
			r.recordResult(name, err)
			results[idx] = DispatchResult{
				Provider: name,
				Response: resp,
				Error:    err,
			}
		}(i, p, names[i])
	}

	wg.Wait()
	return results, nil
}

// DispatchResult holds the outcome of a single provider execution in a
// parallel dispatch.
type DispatchResult struct {
	Provider models.ProviderName    `json:"provider"`
	Response *models.SessionResponse `json:"response,omitempty"`
	Error    error                  `json:"error,omitempty"`
}

// recordResult updates the health metrics for a provider after an execution.
func (r *Registry) recordResult(name models.ProviderName, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.entries[name]
	if !ok {
		return
	}

	now := time.Now()
	if err == nil {
		e.health.LastSuccessAt = &now
		e.health.ConsecutiveFailures = 0
		if e.health.Degraded {
			e.health.Degraded = false
			r.logger.Info("provider recovered from degraded state", "provider", name)
		}
	} else {
		e.health.LastFailureAt = &now
		e.health.ConsecutiveFailures++
		if e.health.ConsecutiveFailures >= degradedThreshold && !e.health.Degraded {
			e.health.Degraded = true
			r.logger.Warn("provider marked as degraded",
				"provider", name,
				"consecutive_failures", e.health.ConsecutiveFailures,
			)
		}
	}
}
