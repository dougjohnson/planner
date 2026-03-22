package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// TestLogger creates a structured slog.Logger that writes JSON output to
// both t.Log() and an internal buffer for assertion. The logger includes
// the test name in every record.
func NewTestLogger(t testing.TB) *slog.Logger {
	t.Helper()
	handler := &testLogHandler{
		t:    t,
		name: t.Name(),
	}
	return slog.New(handler)
}

// LogCapture creates a test logger that captures all log entries for
// later inspection. Use Entries() to retrieve captured records.
func NewLogCapture(t testing.TB) (*slog.Logger, *LogEntries) {
	t.Helper()
	entries := &LogEntries{}
	handler := &captureHandler{
		t:       t,
		entries: entries,
	}
	return slog.New(handler), entries
}

// LogEntries holds captured log records for test assertions.
type LogEntries struct {
	mu      sync.Mutex
	records []LogRecord
}

// LogRecord is a structured log entry captured during tests.
type LogRecord struct {
	Level   string         `json:"level"`
	Message string         `json:"msg"`
	Attrs   map[string]any `json:"attrs"`
}

// All returns all captured log records.
func (le *LogEntries) All() []LogRecord {
	le.mu.Lock()
	defer le.mu.Unlock()
	out := make([]LogRecord, len(le.records))
	copy(out, le.records)
	return out
}

// Count returns the number of captured log records.
func (le *LogEntries) Count() int {
	le.mu.Lock()
	defer le.mu.Unlock()
	return len(le.records)
}

// ContainsMessage returns true if any record has the given message.
func (le *LogEntries) ContainsMessage(msg string) bool {
	le.mu.Lock()
	defer le.mu.Unlock()
	for _, r := range le.records {
		if r.Message == msg {
			return true
		}
	}
	return false
}

// ContainsAttr returns true if any record has a matching key-value attribute.
func (le *LogEntries) ContainsAttr(key string, value any) bool {
	le.mu.Lock()
	defer le.mu.Unlock()
	for _, r := range le.records {
		if v, ok := r.Attrs[key]; ok && v == value {
			return true
		}
	}
	return false
}

// MessagesAtLevel returns all messages logged at the given level.
func (le *LogEntries) MessagesAtLevel(level string) []string {
	le.mu.Lock()
	defer le.mu.Unlock()
	var msgs []string
	for _, r := range le.records {
		if strings.EqualFold(r.Level, level) {
			msgs = append(msgs, r.Message)
		}
	}
	return msgs
}

// testLogHandler writes JSON log entries to t.Log().
type testLogHandler struct {
	t      testing.TB
	name   string
	attrs  []slog.Attr
	groups []string
}

func (h *testLogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true // capture everything during tests
}

func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer
	entry := map[string]any{
		"level": r.Level.String(),
		"msg":   r.Message,
		"test":  h.name,
	}

	// Add pre-attached attrs.
	for _, a := range h.attrs {
		entry[a.Key] = a.Value.Any()
	}

	// Add record attrs.
	r.Attrs(func(a slog.Attr) bool {
		entry[a.Key] = a.Value.Any()
		return true
	})

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(entry); err != nil {
		h.t.Logf("[LOG ERROR] %v", err)
		return nil
	}

	h.t.Log(strings.TrimSpace(buf.String()))
	return nil
}

func (h *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &testLogHandler{
		t:      h.t,
		name:   h.name,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

func (h *testLogHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &testLogHandler{
		t:      h.t,
		name:   h.name,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

// captureHandler captures log entries for assertion without writing to t.Log().
type captureHandler struct {
	t       testing.TB
	entries *LogEntries
	attrs   []slog.Attr
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	record := LogRecord{
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   make(map[string]any),
	}

	for _, a := range h.attrs {
		record.Attrs[a.Key] = a.Value.Any()
	}

	r.Attrs(func(a slog.Attr) bool {
		record.Attrs[a.Key] = a.Value.Any()
		return true
	})

	h.entries.mu.Lock()
	h.entries.records = append(h.entries.records, record)
	h.entries.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &captureHandler{
		t:       h.t,
		entries: h.entries,
		attrs:   newAttrs,
	}
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	return h // groups not deeply handled for captures
}
