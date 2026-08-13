package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// G3: the operator auth middleware that guards the custody webhook (and other
// sensitive endpoints) must reject unauthenticated requests and accept the
// configured AUTH_TOKEN via Bearer or X-Auth-Token.
func TestAuthMiddlewareRejectsAndAccepts(t *testing.T) {
	t.Setenv("AUTH_TOKEN", "topsecret")

	mw := NewAuthMiddleware()
	if !mw.IsEnabled() {
		t.Fatal("middleware should be enabled when AUTH_TOKEN is set")
	}

	called := false
	handler := mw.WrapFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// No token -> 401, handler not called.
	req := httptest.NewRequest(http.MethodPost, "/api/custody/holds/t1/webhook?token=x", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-token code = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("handler must not run without a token")
	}

	// Bearer token -> 200.
	req = httptest.NewRequest(http.MethodPost, "/api/custody/holds/t1/webhook?token=x", nil)
	req.Header.Set("Authorization", "Bearer topsecret")
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bearer code = %d, want 200", rec.Code)
	}
	if !called {
		t.Fatal("handler must run with valid Bearer token")
	}

	// X-Auth-Token header -> 200.
	called = false
	req = httptest.NewRequest(http.MethodPost, "/api/custody/holds/t1/webhook?token=x", nil)
	req.Header.Set("X-Auth-Token", "topsecret")
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("x-auth-token code = %d, want 200", rec.Code)
	}
	if !called {
		t.Fatal("handler must run with valid X-Auth-Token")
	}

	// Wrong token -> 401.
	called = false
	req = httptest.NewRequest(http.MethodPost, "/api/custody/holds/t1/webhook?token=x", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-token code = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("handler must not run with wrong token")
	}
}

// G3: with AUTH_TOKEN unset (dev mode), auth is disabled and the handler runs
// unconditionally.
func TestAuthMiddlewareDisabledWhenNoToken(t *testing.T) {
	t.Setenv("AUTH_TOKEN", "")
	if NewAuthMiddleware() != nil {
		t.Fatal("NewAuthMiddleware must return nil when AUTH_TOKEN is unset")
	}
	mw := NewAuthMiddleware() // nil
	called := false
	handler := mw.WrapFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/custody/holds/t1/webhook", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("handler should run unauthenticated in dev mode (code=%d called=%v)", rec.Code, called)
	}
}
