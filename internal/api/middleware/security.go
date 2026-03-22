// Package middleware provides HTTP middleware for the flywheel-planner application.
package middleware

import (
	"net/http"
)

// SecurityHeaders adds hardened security headers to every response.
// Per §6.4 and §15.2.1: CSP (self-only), nosniff, DENY framing.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
