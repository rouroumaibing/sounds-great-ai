package services

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	authPorts "sounds-great-ai/internal/domains/auth/ports"
)

// AuthorizationManager manages authorization rules, audit logging, and middleware.
// It extends the simple token-based auth with a rule engine and audit trail.
type AuthorizationManager struct {
	token   string
	rules   authPorts.IAuthRuleStore
	audit   authPorts.IAuthAuditStore
	pending authPorts.IPendingRequestStore
	mu      sync.RWMutex
}

// NewAuthorizationManager creates an AuthorizationManager from config.
// If AUTH_TOKEN is not set, auth is disabled (development mode).
func NewAuthorizationManager(
	rules authPorts.IAuthRuleStore,
	audit authPorts.IAuthAuditStore,
	pending authPorts.IPendingRequestStore,
) *AuthorizationManager {
	token := os.Getenv("AUTH_TOKEN")
	return &AuthorizationManager{
		token:   token,
		rules:   rules,
		audit:   audit,
		pending: pending,
	}
}

// IsEnabled returns true if auth is active.
func (m *AuthorizationManager) IsEnabled() bool {
	return m != nil && m.token != ""
}

// Check evaluates a permission request against rules and token.
func (m *AuthorizationManager) Check(ctx context.Context, req authPorts.PermissionRequest) (authPorts.PermissionResponse, error) {
	if !m.IsEnabled() {
		return authPorts.PermissionResponse{Granted: true, Reason: "auth disabled"}, nil
	}

	if m.rules != nil {
		rule, found, err := m.rules.Match(ctx, req.Action)
		if err == nil && found {
			decision := rule.Decision == "allow"
			m.auditRequest(ctx, req, decision, "rule:"+rule.ID)
			return authPorts.PermissionResponse{Granted: decision, Reason: "rule:" + rule.ID, Rule: &rule}, nil
		}
	}

	return authPorts.PermissionResponse{Granted: false, Reason: "no matching rule"}, nil
}

// Middleware returns HTTP middleware that checks auth.
func (m *AuthorizationManager) Middleware(next http.Handler) http.Handler {
	if !m.IsEnabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.checkToken(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// WrapFunc wraps an http.HandlerFunc with token authentication.
func (m *AuthorizationManager) WrapFunc(h http.HandlerFunc) http.HandlerFunc {
	if !m.IsEnabled() {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.checkToken(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		h(w, r)
	}
}

// Wrap wraps an http.Handler with token authentication.
func (m *AuthorizationManager) Wrap(h http.Handler) http.Handler {
	if !m.IsEnabled() {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.checkToken(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		h.ServeHTTP(w, r)
	})
}

// checkToken validates the request against the configured token.
func (m *AuthorizationManager) checkToken(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		provided := strings.TrimPrefix(authHeader, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(m.token)) == 1 {
			return true
		}
	}
	if t := r.Header.Get("X-Auth-Token"); t != "" {
		if subtle.ConstantTimeCompare([]byte(t), []byte(m.token)) == 1 {
			return true
		}
	}
	return false
}

// auditRequest records an audit entry if audit store is configured.
func (m *AuthorizationManager) auditRequest(ctx context.Context, req authPorts.PermissionRequest, granted bool, reason string) {
	if m.audit == nil {
		return
	}
	decision := "denied"
	if granted {
		decision = "granted"
	}
	entry := authPorts.AuditEntry{
		ID:        req.Action + ":" + req.Path,
		Action:    req.Action,
		Method:    req.Method,
		Path:      req.Path,
		Decision:  decision,
		Reason:    reason,
		Timestamp: time.Now(),
	}
	if err := m.audit.Record(ctx, entry); err != nil {
		log.Printf("warning: audit record failed: %v", err)
	}
}
