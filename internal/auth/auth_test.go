package auth

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenStore_FailClosed(t *testing.T) {
	s := NewTokenStore()
	if _, err := s.Authenticate(""); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("empty token must deny")
	}
	if _, err := s.Authenticate("unknown"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("unknown token must deny")
	}
	s.Put("tok", User{ID: "u1", Name: "Alice"})
	p, err := s.Authenticate("tok")
	if err != nil || p.ID != "u1" || string(p.Type) != "user" {
		t.Fatalf("authenticate: %v %+v", err, p)
	}
}

func TestThreadACL_FailClosed(t *testing.T) {
	a := NewThreadACL()
	// no rules => everyone denied
	if ok, err := a.Check("th1", "u1", AccessRead); ok || !errors.Is(err, ErrDenied) {
		t.Fatalf("empty ACL must deny: ok=%v err=%v", ok, err)
	}
	a.Grant("th1", "u1", AccessWrite)
	if ok, _ := a.Check("th1", "u1", AccessRead); !ok {
		t.Fatal("write implies read")
	}
	if ok, _ := a.Check("th1", "u1", AccessAdmin); ok {
		t.Fatal("write does not imply admin")
	}
	if ok, _ := a.Check("th1", "stranger", AccessRead); ok {
		t.Fatal("stranger must be denied")
	}
}

func TestThreadACL_Revoke(t *testing.T) {
	a := NewThreadACL()
	a.Grant("th1", "u1", AccessAdmin)
	a.Revoke("th1", "u1")
	if ok, _ := a.Check("th1", "u1", AccessRead); ok {
		t.Fatal("revoked user must be denied")
	}
}

func TestCallbackToken_Lifecycle(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewCallbackToken("cb1", "u1", []string{"read"}, now.Add(time.Hour))
	if !c.Valid(now) {
		t.Fatal("fresh token valid")
	}
	c.MarkFailure(ReasonExpired)
	if c.Valid(now) {
		t.Fatal("failed token invalid")
	}
	// refresh clears failure
	c.Refresh(now.Add(2 * time.Hour))
	if !c.Valid(now) {
		t.Fatal("refreshed token valid")
	}
}

func TestCallbackTokenStore_RejectsInvalid(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewCallbackTokenStore(filepath.Join(t.TempDir(), "cbauth.db"))
	c := NewCallbackToken("cb1", "u1", nil, now.Add(-time.Hour)) // expired
	store.Put(c)
	if _, ok := store.Lookup("cb1", now); ok {
		t.Fatal("expired token must not be returned")
	}
	c2 := NewCallbackToken("cb2", "u1", nil, now.Add(time.Hour))
	store.Put(c2)
	if _, ok := store.Lookup("cb2", now); !ok {
		t.Fatal("valid token must be returned")
	}
	if _, ok := store.Lookup("nope", now); ok {
		t.Fatal("unknown token must not be returned")
	}
}

func TestAgentKey_ValidateFailClosed(t *testing.T) {
	store := NewInMemoryAgentKeyStore()
	reg := NewAgentKeyRegistry(store)
	k, err := reg.Issue("bianmu", "key1")
	if err != nil || k.AgentID != "bianmu" {
		t.Fatalf("issue: %v %+v", err, k)
	}
	if ok, _ := reg.Validate("key1"); !ok {
		t.Fatal("issued key valid")
	}
	// rotate
	nu, err := reg.Rotate("key1", "key2")
	if err != nil || nu.RotatedFrom != "key1" {
		t.Fatalf("rotate: %v %+v", err, nu)
	}
	if ok, err := reg.Validate("key1"); ok || !errors.Is(err, ErrAgentKeyRevoked) {
		t.Fatalf("old key must be revoked: ok=%v err=%v", ok, err)
	}
	if ok, _ := reg.Validate("key2"); !ok {
		t.Fatal("new key valid")
	}
	// revoke new
	_ = reg.Revoke("key2")
	if ok, err := reg.Validate("key2"); ok || !errors.Is(err, ErrAgentKeyRevoked) {
		t.Fatalf("revoked key must deny: ok=%v err=%v", ok, err)
	}
}

func TestAgentKey_Unknown(t *testing.T) {
	reg := NewAgentKeyRegistry(NewInMemoryAgentKeyStore())
	if ok, err := reg.Validate("ghost"); ok || !errors.Is(err, ErrAgentKeyNotFound) {
		t.Fatalf("unknown key must deny: ok=%v err=%v", ok, err)
	}
}
