package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// AssertStatus checks that an HTTP response has the expected status code.
func AssertStatus(t testing.TB, rr *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if rr.Code != expected {
		t.Errorf("expected status %d, got %d\nbody: %s", expected, rr.Code, rr.Body.String())
	}
}

// AssertJSONBody unmarshals the response body into v and fails if it can't.
func AssertJSONBody(t testing.TB, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	body := rr.Body.Bytes()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("failed to parse response body as JSON: %v\nbody: %s", err, string(body))
	}
}

// AssertBodyContains checks that the response body contains the given substring.
func AssertBodyContains(t testing.TB, rr *httptest.ResponseRecorder, substr string) {
	t.Helper()
	body := rr.Body.String()
	if !strings.Contains(body, substr) {
		t.Errorf("expected body to contain %q, got: %s", substr, body)
	}
}

// AssertHeader checks that a response header has the expected value.
func AssertHeader(t testing.TB, rr *httptest.ResponseRecorder, key, expected string) {
	t.Helper()
	got := rr.Header().Get(key)
	if got != expected {
		t.Errorf("expected header %s=%q, got %q", key, expected, got)
	}
}

// AssertContentType checks the Content-Type header.
func AssertContentType(t testing.TB, rr *httptest.ResponseRecorder, expected string) {
	t.Helper()
	AssertHeader(t, rr, "Content-Type", expected)
}

// AssertNoError fails the test if err is non-nil.
func AssertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil.
func AssertError(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// AssertErrorContains fails the test if err is nil or doesn't contain the substring.
func AssertErrorContains(t testing.TB, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("expected error to contain %q, got: %v", substr, err)
	}
}

// AssertEqual fails the test if got != want.
func AssertEqual[T comparable](t testing.TB, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// AssertJSONEqual compares two values by serializing them to JSON.
// Useful for struct comparison with unexported fields or complex types.
func AssertJSONEqual(t testing.TB, got, want any) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshaling got: %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshaling want: %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("JSON mismatch:\n  got:  %s\n  want: %s", gotJSON, wantJSON)
	}
}

// DoRequest creates and executes an HTTP request against a handler, returning
// the response recorder. Useful for testing HTTP handlers directly.
func DoRequest(t testing.TB, handler http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// DoJSON creates and executes a JSON request, marshaling the body.
func DoJSON(t testing.TB, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshaling request body: %v", err)
		}
		reader = strings.NewReader(string(data))
	}
	return DoRequest(t, handler, method, path, reader)
}
