package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

const (
	// OriginSecretHeader is the header name for the DNS rebinding defense secret.
	OriginSecretHeader = "X-Origin-Secret"

	// originSecretLength is the byte length of the generated secret.
	originSecretLength = 32
)

// GenerateOriginSecret creates a cryptographically random secret for DNS
// rebinding defense. Called once at startup; the secret lives in memory only.
func GenerateOriginSecret() (string, error) {
	b := make([]byte, originSecretLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// OriginSecretGuard is middleware that requires a valid origin secret header
// on all non-GET/HEAD/OPTIONS mutating requests. This defends against DNS
// rebinding attacks on loopback (§6.4, §15.2).
//
// The secret is generated per-process and passed to the frontend via the
// health endpoint or initial page load meta tag.
func OriginSecretGuard(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow safe methods without the secret.
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			// Require the secret on all mutating requests.
			// Use constant-time comparison to prevent timing attacks.
			provided := r.Header.Get(OriginSecretHeader)
			if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
				http.Error(w, `{"error":{"code":"origin_secret_mismatch","message":"missing or invalid origin secret"}}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
