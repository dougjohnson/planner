package testutil

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewTestLogger(t *testing.T) {
	logger := NewTestLogger(t)
	// Should not panic, and should log to t.Log().
	logger.Info("test message", "key", "value")
	logger.Debug("debug message", "count", 42)
	logger.Warn("warning message")
}

func TestNewLogCapture(t *testing.T) {
	logger, entries := NewLogCapture(t)

	logger.Info("first message", "key", "val1")
	logger.Warn("second message", "count", 2)
	logger.Info("third message")

	if entries.Count() != 3 {
		t.Fatalf("expected 3 entries, got %d", entries.Count())
	}

	if !entries.ContainsMessage("first message") {
		t.Error("expected to find 'first message'")
	}
	if !entries.ContainsMessage("second message") {
		t.Error("expected to find 'second message'")
	}
	if entries.ContainsMessage("nonexistent") {
		t.Error("should not find 'nonexistent'")
	}
}

func TestLogCapture_ContainsAttr(t *testing.T) {
	logger, entries := NewLogCapture(t)
	logger.Info("test", "status", "ok")

	if !entries.ContainsAttr("status", "ok") {
		t.Error("expected to find attr status=ok")
	}
	if entries.ContainsAttr("status", "fail") {
		t.Error("should not find attr status=fail")
	}
}

func TestLogCapture_MessagesAtLevel(t *testing.T) {
	logger, entries := NewLogCapture(t)
	logger.Info("info1")
	logger.Warn("warn1")
	logger.Info("info2")
	logger.Error("error1")

	infos := entries.MessagesAtLevel("INFO")
	if len(infos) != 2 {
		t.Errorf("expected 2 INFO messages, got %d", len(infos))
	}

	warns := entries.MessagesAtLevel("WARN")
	if len(warns) != 1 {
		t.Errorf("expected 1 WARN message, got %d", len(warns))
	}
}

func TestFixturePath(t *testing.T) {
	path := FixturePath("seeds", "sample_prd.md")
	if path == "" {
		t.Fatal("fixture path should not be empty")
	}
	// Verify it ends with the expected suffix.
	expected := "tests/fixtures/seeds/sample_prd.md"
	if len(path) < len(expected) {
		t.Fatalf("path too short: %s", path)
	}
	suffix := path[len(path)-len(expected):]
	if suffix != expected {
		t.Errorf("expected path to end with %s, got %s", expected, suffix)
	}
}

func TestLoadFixture(t *testing.T) {
	data := LoadFixture(t, "seeds", "sample_prd.md")
	if len(data) == 0 {
		t.Fatal("fixture should not be empty")
	}
	content := string(data)
	if content[:1] != "#" {
		t.Errorf("expected PRD to start with #, got %q", content[:20])
	}
}

func TestLoadFixtureString(t *testing.T) {
	content := LoadFixtureString(t, "seeds", "sample_prd.md")
	if len(content) == 0 {
		t.Fatal("fixture string should not be empty")
	}
}

func TestLoadFixtureJSON(t *testing.T) {
	var foundations map[string]any
	LoadFixtureJSON(t, &foundations, "seeds", "sample_foundations.json")

	name, ok := foundations["project_name"].(string)
	if !ok || name != "Test Task Manager" {
		t.Errorf("expected project_name='Test Task Manager', got %v", foundations["project_name"])
	}
}

func TestLoadFixtureJSON_Fragments(t *testing.T) {
	var fragments []struct {
		ID      string `json:"id"`
		Heading string `json:"heading"`
		Ordinal int    `json:"ordinal"`
		Content string `json:"content"`
		Version int    `json:"version"`
	}
	LoadFixtureJSON(t, &fragments, "fragments", "sample_fragments.json")

	if len(fragments) != 4 {
		t.Fatalf("expected 4 fragments, got %d", len(fragments))
	}
	if fragments[0].Heading != "Overview" {
		t.Errorf("expected first fragment heading 'Overview', got %q", fragments[0].Heading)
	}
}

func TestMustWriteFixture(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`{"ok":true}`)
	path := MustWriteFixture(t, dir, "sub/test.json", content)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written fixture: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("expected %s, got %s", content, data)
	}
}

func TestAssertStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusOK)
	// Should not fail.
	AssertStatus(t, rr, http.StatusOK)
}

func TestAssertBodyContains(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteString(`{"status":"ok"}`)
	AssertBodyContains(t, rr, "ok")
}

func TestAssertHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Custom", "value")
	AssertHeader(t, rr, "X-Custom", "value")
}

func TestAssertNoError(t *testing.T) {
	AssertNoError(t, nil)
}

func TestAssertError(t *testing.T) {
	AssertError(t, errors.New("some error"))
}

func TestAssertErrorContains(t *testing.T) {
	err := errors.New("connection refused: port 5432")
	AssertErrorContains(t, err, "connection refused")
}

func TestAssertEqual(t *testing.T) {
	AssertEqual(t, 42, 42)
	AssertEqual(t, "hello", "hello")
	AssertEqual(t, true, true)
}

func TestDoRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	rr := DoRequest(t, handler, "GET", "/api/health", nil)
	AssertStatus(t, rr, http.StatusOK)
	AssertContentType(t, rr, "application/json")
	AssertBodyContains(t, rr, "ok")
}

func TestDoJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(body)
	})

	rr := DoJSON(t, handler, "POST", "/api/projects", map[string]string{"name": "test"})
	AssertStatus(t, rr, http.StatusCreated)
}
