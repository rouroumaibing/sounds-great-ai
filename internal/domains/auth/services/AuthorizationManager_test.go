package services

import (
	"context"
	"testing"

	authPorts "sounds-great-ai/internal/domains/auth/ports"
	authStores "sounds-great-ai/internal/domains/auth/stores"
)

func TestAuthorizationManager_DisabledWhenNoToken(t *testing.T) {
	t.Setenv("AUTH_TOKEN", "")
	rules := authStores.NewMemoryAuthStore()
	mgr := NewAuthorizationManager(rules, rules, nil)

	if mgr.IsEnabled() {
		t.Fatal("auth should be disabled when AUTH_TOKEN is empty")
	}

	resp, err := mgr.Check(context.Background(), authPorts.PermissionRequest{
		Action: "test", Method: "GET", Path: "/api/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Granted {
		t.Fatal("disabled auth should grant all requests")
	}
}

func TestAuthorizationManager_TokenCheck(t *testing.T) {
	t.Setenv("AUTH_TOKEN", "secret123")
	rules := authStores.NewMemoryAuthStore()
	mgr := NewAuthorizationManager(rules, rules, nil)

	if !mgr.IsEnabled() {
		t.Fatal("auth should be enabled when AUTH_TOKEN is set")
	}

	resp, err := mgr.Check(context.Background(), authPorts.PermissionRequest{
		Action: "test", Method: "GET", Path: "/api/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Granted {
		t.Fatal("should not grant without matching rule")
	}
}

func TestAuthorizationManager_RuleMatch(t *testing.T) {
	t.Setenv("AUTH_TOKEN", "secret123")
	rules := authStores.NewMemoryAuthStore()
	mgr := NewAuthorizationManager(rules, rules, nil)

	rule := authPorts.Rule{
		ID:       "rule-1",
		Action:   "read",
		Methods:  []string{"GET"},
		Decision: "allow",
	}
	if err := rules.Add(context.Background(), rule); err != nil {
		t.Fatalf("add rule: %v", err)
	}

	resp, err := mgr.Check(context.Background(), authPorts.PermissionRequest{
		Action: "read", Method: "GET", Path: "/api/data",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Granted {
		t.Fatal("should grant when rule matches and allows")
	}
}
