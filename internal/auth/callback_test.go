package auth

import (
	"path/filepath"
	"testing"
	"time"
)

// F174 AC: a freshly issued callback token is valid until its expiry, and the
// default degrade policy is fail-closed (deny).
func TestCallbackToken_ValidUntilExpiry(t *testing.T) {
	now := time.Now()
	tok := NewCallbackToken("t1", "op-1", []string{"thread:write"}, now.Add(time.Hour))
	if !tok.Valid(now) {
		t.Fatal("fresh token must be valid")
	}
	if tok.Degrade != DegradeDeny {
		t.Fatalf("default degrade must be fail-closed deny, got %q", tok.Degrade)
	}
	// After expiry → invalid (fail-closed).
	if tok.Valid(now.Add(2 * time.Hour)) {
		t.Fatal("expired token must be invalid")
	}
}

// F174 AC: a recorded failure reason invalidates the token even if unexpired.
func TestCallbackToken_FailureInvalidates(t *testing.T) {
	now := time.Now()
	tok := NewCallbackToken("t1", "op-1", nil, now.Add(time.Hour))
	tok.MarkFailure(ReasonRevoked)
	if tok.Valid(now) {
		t.Fatal("token with failure reason must be invalid")
	}
}

// F174 AC: Refresh rotates the expiry AND clears the failure reason, restoring
// validity (the resilience/续期 path).
func TestCallbackToken_RefreshRestores(t *testing.T) {
	now := time.Now()
	tok := NewCallbackToken("t1", "op-1", nil, now.Add(-time.Hour)) // already expired
	tok.MarkFailure(ReasonExpired)
	if tok.Valid(now) {
		t.Fatal("precondition: expired+failed token invalid")
	}
	tok.Refresh(now.Add(time.Hour)) // rotate + clear failure
	if !tok.Valid(now) {
		t.Fatal("refreshed token must be valid")
	}
	if tok.Failure != "" {
		t.Fatalf("refresh must clear failure reason, got %q", tok.Failure)
	}
}

// F174 AC: Lookup only returns currently-valid tokens. Missing / expired /
// failed tokens are never handed back as a usable credential (fail-closed).
func TestCallbackTokenStore_LookupFailClosed(t *testing.T) {
	now := time.Now()
	store := NewCallbackTokenStore(filepath.Join(t.TempDir(), "cbauth.db"))

	// Missing token → (nil, false).
	if c, ok := store.Lookup("ghost", now); ok || c != nil {
		t.Fatal("unknown token must not resolve to a usable credential")
	}

	good := NewCallbackToken("good", "op-1", []string{"thread:write"}, now.Add(time.Hour))
	store.Put(good)
	if c, ok := store.Lookup("good", now); !ok || c == nil {
		t.Fatal("valid token must be found")
	}

	// Expired token → (nil, false).
	expired := NewCallbackToken("expired", "op-1", nil, now.Add(-time.Hour))
	store.Put(expired)
	if c, ok := store.Lookup("expired", now); ok || c != nil {
		t.Fatal("expired token must not resolve to a usable credential")
	}

	// Failed (revoked) token → (nil, false).
	revoked := NewCallbackToken("revoked", "op-1", nil, now.Add(time.Hour))
	revoked.MarkFailure(ReasonRevoked)
	store.Put(revoked)
	if c, ok := store.Lookup("revoked", now); ok || c != nil {
		t.Fatal("revoked token must not resolve to a usable credential")
	}
}

// F174 AC: NewCallbackToken defaults to deny (no silent degrade-open).
func TestCallbackToken_DefaultDegradeDeny(t *testing.T) {
	tok := NewCallbackToken("t1", "op-1", nil, time.Now().Add(time.Hour))
	if tok.Degrade != DegradeDeny {
		t.Fatalf("new token must default to DegradeDeny, got %q", tok.Degrade)
	}
}
