package transport

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// AuthMiddleware provides token-based authentication for sensitive API endpoints.
// If AUTH_TOKEN is not set, auth is disabled (development mode).
// If AUTH_TOKEN is set, requests must include it via Bearer header or X-Auth-Token.
type AuthMiddleware struct {
	token string
}

// NewAuthMiddleware creates an AuthMiddleware from the AUTH_TOKEN env var.
// Returns nil if AUTH_TOKEN is not set (auth disabled).
func NewAuthMiddleware() *AuthMiddleware {
	token := os.Getenv("AUTH_TOKEN")
	if token == "" {
		return nil
	}
	return &AuthMiddleware{token: token}
}

// IsEnabled returns true if auth is active.
func (a *AuthMiddleware) IsEnabled() bool {
	return a != nil && a.token != ""
}

// Wrap wraps an http.Handler with token authentication.
// If auth is disabled (nil middleware), returns the handler unchanged.
func (a *AuthMiddleware) Wrap(h http.Handler) http.Handler {
	if a == nil || a.token == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.checkAuth(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		h.ServeHTTP(w, r)
	})
}

// WrapFunc wraps an http.HandlerFunc with token authentication.
func (a *AuthMiddleware) WrapFunc(h http.HandlerFunc) http.HandlerFunc {
	if a == nil || a.token == "" {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.checkAuth(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		h(w, r)
	}
}

// checkAuth validates the request against the configured token.
func (a *AuthMiddleware) checkAuth(r *http.Request) bool {
	// Bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		provided := strings.TrimPrefix(authHeader, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(a.token)) == 1 {
			return true
		}
	}
	// X-Auth-Token header
	if t := r.Header.Get("X-Auth-Token"); t != "" {
		if subtle.ConstantTimeCompare([]byte(t), []byte(a.token)) == 1 {
			return true
		}
	}
	return false
}

// CORSMiddleware sets CORS headers for the given allowed origin.
func CORSMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if allowedOrigin == "" || (origin != "" && origin == allowedOrigin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if origin == "" {
					w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Auth-Token")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
