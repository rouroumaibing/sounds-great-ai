package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/settings"
)

func TestUpdateAccount(t *testing.T) {
	store := settings.NewInMemorySettingsStore()
	credStore := settings.NewMemoryCredentialStore()
	bus := config.NewEventBus()
	handler := NewSettingsHandlerWithCredentials(store, credStore, bus)
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	account, err := store.CreateAccount("openai", "sk-original")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// PATCH with a new provider and api_key.
	body := `{"provider":"anthropic","api_key":"sk-new-key"}`
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/api/settings/accounts/"+account.ID, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}

	// The credential should be stored in the CredentialStore.
	key, err := credStore.Get(account.ID)
	if err != nil {
		t.Fatalf("get credential: %v", err)
	}
	if key != "sk-new-key" {
		t.Fatalf("credential: want sk-new-key, got %s", key)
	}

	// The provider metadata should be updated.
	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Provider != "anthropic" {
		t.Fatalf("provider: want anthropic, got %+v", accounts)
	}
}

func TestUpdateConfig(t *testing.T) {
	store := settings.NewInMemorySettingsStore()
	bus := config.NewEventBus()
	handler := NewSettingsHandlerWithCredentials(store, nil, bus)
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	body := `{"key":"rag_top_k","value":"10"}`
	req, err := http.NewRequest(http.MethodPatch, server.URL+"/api/settings/config", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}

	configs, err := store.ListConfig()
	if err != nil {
		t.Fatalf("list config: %v", err)
	}
	for _, c := range configs {
		if c.Key == "rag_top_k" {
			if c.Value != "10" {
				t.Fatalf("value: want 10, got %s", c.Value)
			}
			return
		}
	}
	t.Fatalf("rag_top_k not found in config")
}

func TestDeleteAccount_ReferentialIntegrity(t *testing.T) {
	store := settings.NewInMemorySettingsStore()
	credStore := settings.NewMemoryCredentialStore()
	bus := config.NewEventBus()
	handler := NewSettingsHandlerWithCredentials(store, credStore, bus)

	account, err := store.CreateAccount("openai", "sk-test")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	// Seed the credential store so we can verify cleanup on forced delete.
	if err := credStore.Set(account.ID, "sk-test"); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	// Bind two breeds to this account.
	handler.SetBreedBindings(map[string][]string{
		account.ID: {"bianmu", "jinmao"},
	})

	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	// Delete without force → 409 Conflict with bound_breed_ids.
	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/settings/accounts/"+account.ID, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status without force: want 409, got %d", resp.StatusCode)
	}
	var conflict map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&conflict); err != nil {
		t.Fatalf("decode conflict body: %v", err)
	}
	resp.Body.Close()
	boundIDs, ok := conflict["bound_breed_ids"].([]any)
	if !ok || len(boundIDs) != 2 {
		t.Fatalf("bound_breed_ids: want 2 items, got %v", conflict["bound_breed_ids"])
	}

	// The account must still exist.
	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("account should still exist, got %d accounts", len(accounts))
	}

	// Delete with force → 200 OK.
	req, err = http.NewRequest(http.MethodDelete, server.URL+"/api/settings/accounts/"+account.ID+"?force=true", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status with force: want 200, got %d", resp.StatusCode)
	}

	// The credential should have been cleaned up.
	if credStore.Has(account.ID) {
		t.Fatalf("credential should be deleted after forced account deletion")
	}

	// The account should be gone.
	accounts, err = store.ListAccounts()
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("account should be deleted, got %d accounts", len(accounts))
	}
}
