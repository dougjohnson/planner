package sse

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestHub_PublishToSubscriber(t *testing.T) {
	hub := NewHub(testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, unsub := hub.Subscribe(ctx, "project-1")
	defer unsub()

	hub.Publish("project-1", "workflow:stage_started", map[string]string{"stage": "prd_intake"})

	select {
	case evt := <-events:
		if evt.Type != "workflow:stage_started" {
			t.Errorf("expected event type 'workflow:stage_started', got %q", evt.Type)
		}
		if evt.ProjectID != "project-1" {
			t.Errorf("expected project 'project-1', got %q", evt.ProjectID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHub_ProjectIsolation(t *testing.T) {
	hub := NewHub(testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsA, unsubA := hub.Subscribe(ctx, "project-A")
	defer unsubA()
	eventsB, unsubB := hub.Subscribe(ctx, "project-B")
	defer unsubB()

	// Publish to project A only.
	hub.Publish("project-A", "workflow:stage_completed", nil)

	// A should receive the event.
	select {
	case <-eventsA:
		// OK
	case <-time.After(time.Second):
		t.Fatal("project A subscriber should have received event")
	}

	// B should NOT receive it.
	select {
	case <-eventsB:
		t.Fatal("project B subscriber should NOT receive project A events")
	case <-time.After(100 * time.Millisecond):
		// OK — no event received
	}
}

func TestHub_MultipleSubscribers(t *testing.T) {
	hub := NewHub(testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 5
	channels := make([]<-chan Event, n)
	cancels := make([]context.CancelFunc, n)

	for i := range n {
		channels[i], cancels[i] = hub.Subscribe(ctx, "project-1")
		defer cancels[i]()
	}

	hub.Publish("project-1", "workflow:run_started", nil)

	for i := range n {
		select {
		case <-channels[i]:
			// OK
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive event", i)
		}
	}
}

func TestHub_SubscriberCount(t *testing.T) {
	hub := NewHub(testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if hub.SubscriberCount("project-1") != 0 {
		t.Error("expected 0 subscribers initially")
	}

	_, unsub1 := hub.Subscribe(ctx, "project-1")
	_, unsub2 := hub.Subscribe(ctx, "project-1")

	if hub.SubscriberCount("project-1") != 2 {
		t.Errorf("expected 2 subscribers, got %d", hub.SubscriberCount("project-1"))
	}

	unsub1()
	// Give goroutine time to clean up.
	time.Sleep(50 * time.Millisecond)

	if hub.SubscriberCount("project-1") != 1 {
		t.Errorf("expected 1 subscriber after unsub, got %d", hub.SubscriberCount("project-1"))
	}

	unsub2()
	time.Sleep(50 * time.Millisecond)

	if hub.SubscriberCount("project-1") != 0 {
		t.Errorf("expected 0 subscribers after all unsub, got %d", hub.SubscriberCount("project-1"))
	}
}

func TestHub_AutoUnregisterOnContextCancel(t *testing.T) {
	hub := NewHub(testLogger())
	ctx, cancel := context.WithCancel(context.Background())

	_, _ = hub.Subscribe(ctx, "project-1")

	if hub.SubscriberCount("project-1") != 1 {
		t.Fatal("expected 1 subscriber")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)

	if hub.SubscriberCount("project-1") != 0 {
		t.Errorf("expected 0 subscribers after context cancel, got %d", hub.SubscriberCount("project-1"))
	}
}

func TestHub_NonBlockingPublish(t *testing.T) {
	hub := NewHub(testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe but never read from the channel.
	_, unsub := hub.Subscribe(ctx, "project-1")
	defer unsub()

	// Publish more events than the buffer size — should not block.
	done := make(chan struct{})
	go func() {
		for range subscriberBuffer + 10 {
			hub.Publish("project-1", "workflow:run_progress", nil)
		}
		close(done)
	}()

	select {
	case <-done:
		// OK — publish did not block
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on slow subscriber")
	}
}

func TestHub_ConcurrentPublishAndSubscribe(t *testing.T) {
	hub := NewHub(testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Concurrent subscribers.
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, unsub := hub.Subscribe(ctx, "project-1")
			time.Sleep(50 * time.Millisecond)
			unsub()
		}()
	}

	// Concurrent publishers.
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				hub.Publish("project-1", "workflow:state_changed", nil)
			}
		}()
	}

	wg.Wait()
}
