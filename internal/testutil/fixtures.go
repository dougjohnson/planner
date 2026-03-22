package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fixturesRoot returns the absolute path to the tests/fixtures directory,
// resolved relative to the project root.
func fixturesRoot() string {
	// Walk up from this file to find the project root.
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile is internal/testutil/fixtures.go → project root is ../../
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "tests", "fixtures")
}

// FixturePath returns the absolute path to a fixture file.
// The path components are joined: FixturePath("seeds", "sample.json")
// returns tests/fixtures/seeds/sample.json.
func FixturePath(parts ...string) string {
	return filepath.Join(append([]string{fixturesRoot()}, parts...)...)
}

// LoadFixture reads a fixture file and returns its raw bytes.
// Fails the test if the file cannot be read.
func LoadFixture(t testing.TB, parts ...string) []byte {
	t.Helper()
	path := FixturePath(parts...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("testutil: loading fixture %s: %v", path, err)
	}
	return data
}

// LoadFixtureString reads a fixture file as a string.
func LoadFixtureString(t testing.TB, parts ...string) string {
	t.Helper()
	return string(LoadFixture(t, parts...))
}

// LoadFixtureJSON reads a fixture file and unmarshals it into the given value.
func LoadFixtureJSON(t testing.TB, v any, parts ...string) {
	t.Helper()
	data := LoadFixture(t, parts...)
	if err := json.Unmarshal(data, v); err != nil {
		path := FixturePath(parts...)
		t.Fatalf("testutil: parsing fixture JSON %s: %v", path, err)
	}
}

// MustWriteFixture writes content to a fixture path, creating directories
// as needed. Useful in test setup to create temporary fixtures.
// Only writes to t.TempDir()-based paths for safety.
func MustWriteFixture(t testing.TB, dir string, filename string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("testutil: creating fixture dir: %v", err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("testutil: writing fixture %s: %v", path, err)
	}
	return path
}
