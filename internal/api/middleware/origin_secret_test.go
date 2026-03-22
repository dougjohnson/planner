package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateOriginSecret(t *testing.T) {
	s1, err := GenerateOriginSecret()
	if err != nil {
		t.Fatalf("GenerateOriginSecret: %v", err)
	}
	if len(s1) != originSecretLength*2 { // hex encoding doubles length
		t.Errorf("secret length = %d, want %d", len(s1), originSecretLength*2)
	}

	// Each call produces a different secret.
	s2, _ := GenerateOriginSecret()
	if s1 == s2 {
		t.Error("two generated secrets should not be equal")
	}
}

func TestOriginSecretGuard_AllowsGET(t *testing.T) {
	handler := OriginSecretGuard("test-secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET should be allowed without secret, got %d", w.Code)
	}
}

func TestOriginSecretGuard_AllowsHEAD(t *testing.T) {
	handler := OriginSecretGuard("test-secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodHead, "/api/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HEAD should be allowed without secret, got %d", w.Code)
	}
}

func TestOriginSecretGuard_BlocksPOSTWithoutSecret(t *testing.T) {
	handler := OriginSecretGuard("test-secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/projects", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("POST without secret should be 403, got %d", w.Code)
	}
}

func TestOriginSecretGuard_BlocksPOSTWithWrongSecret(t *testing.T) {
	handler := OriginSecretGuard("correct-secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/projects", nil)
	req.Header.Set(OriginSecretHeader, "wrong-secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("POST with wrong secret should be 403, got %d", w.Code)
	}
}

func TestOriginSecretGuard_AllowsPOSTWithCorrectSecret(t *testing.T) {
	secret := "my-test-secret-123"
	handler := OriginSecretGuard(secret)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/projects", nil)
	req.Header.Set(OriginSecretHeader, secret)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST with correct secret should be 200, got %d", w.Code)
	}
}

func TestOriginSecretGuard_BlocksPATCH(t *testing.T) {
	handler := OriginSecretGuard("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/projects/123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("PATCH without secret should be 403, got %d", w.Code)
	}
}

func TestOriginSecretGuard_BlocksDELETE(t *testing.T) {
	handler := OriginSecretGuard("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/projects/123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("DELETE without secret should be 403, got %d", w.Code)
	}
}
