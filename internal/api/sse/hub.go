// Package sse provides an in-process Server-Sent Events hub for real-time
// workflow event delivery. The hub is keyed by project ID — events for one
// project never reach subscribers of another.
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	// HeartbeatInterval is the time between heartbeat comments on idle connections.
	HeartbeatInterval = 30 * time.Second

	// subscriberBuffer is the channel buffer size per subscriber.
	// Non-blocking publish drops events for slow subscribers.
	subscriberBuffer = 64
)

// Event represents a structured SSE event.
type Event struct {
	Type      string `json:"type"`      // e.g. "workflow:stage_started"
	ProjectID string `json:"project_id"`
	Data      any    `json:"data"`
}

// subscriber is a single SSE client.
type subscriber struct {
	ch     chan Event
	cancel context.CancelFunc
}

// Hub manages SSE subscriptions keyed by project ID.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[*subscriber]struct{} // projectID → set of subscribers
	logger      *slog.Logger
}

// NewHub creates a new SSE Hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		subscribers: make(map[string]map[*subscriber]struct{}),
		logger:      logger,
	}
}

// Publish sends an event to all subscribers of the given project.
// Non-blocking: slow subscribers that can't keep up will have events dropped.
func (h *Hub) Publish(projectID string, eventType string, data any) {
	evt := Event{
		Type:      eventType,
		ProjectID: projectID,
		Data:      data,
	}

	// Snapshot subscriber channels under lock to avoid races with subscribe/unsubscribe.
	h.mu.RLock()
	subs := h.subscribers[projectID]
	channels := make([]chan Event, 0, len(subs))
	for sub := range subs {
		channels = append(channels, sub.ch)
	}
	h.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- evt:
		default:
			h.logger.Warn("dropping event for slow subscriber",
				"project_id", projectID, "event_type", eventType)
		}
	}
}

// Subscribe registers a new subscriber for the given project.
// Returns a channel that receives events and a cancel function.
func (h *Hub) Subscribe(ctx context.Context, projectID string) (<-chan Event, context.CancelFunc) {
	subCtx, cancel := context.WithCancel(ctx)

	sub := &subscriber{
		ch:     make(chan Event, subscriberBuffer),
		cancel: cancel,
	}

	h.mu.Lock()
	if h.subscribers[projectID] == nil {
		h.subscribers[projectID] = make(map[*subscriber]struct{})
	}
	h.subscribers[projectID][sub] = struct{}{}
	count := len(h.subscribers[projectID])
	h.mu.Unlock()

	h.logger.Debug("subscriber added", "project_id", projectID, "count", count)

	// Auto-unregister when context is cancelled.
	go func() {
		<-subCtx.Done()
		h.mu.Lock()
		delete(h.subscribers[projectID], sub)
		if len(h.subscribers[projectID]) == 0 {
			delete(h.subscribers, projectID)
		}
		h.mu.Unlock()
		// Do not close sub.ch here — a concurrent Publish may still be
		// sending to a snapshot of the channel. The channel will be GC'd
		// once all references are gone. ServeHTTP detects closure via
		// context cancellation instead.
		h.logger.Debug("subscriber removed", "project_id", projectID)
	}()

	return sub.ch, cancel
}

// SubscriberCount returns the number of active subscribers for a project.
func (h *Hub) SubscriberCount(projectID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers[projectID])
}

// ServeHTTP handles an SSE connection for a specific project.
// The projectID must be extracted from the URL and passed via context or query param.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request, projectID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events, cancel := h.Subscribe(r.Context(), projectID)
	defer cancel()

	heartbeat := time.NewTicker(HeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
			if err != nil {
				h.logger.Error("marshaling event", "error", err)
				continue
			}
			_, writeErr := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
			if writeErr != nil {
				h.logger.Debug("client disconnected", "project_id", projectID, "error", writeErr)
				return
			}
			flusher.Flush()

		case <-heartbeat.C:
			_, writeErr := fmt.Fprintf(w, ": heartbeat\n\n")
			if writeErr != nil {
				h.logger.Debug("heartbeat write failed", "project_id", projectID)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}
