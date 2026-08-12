package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
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

func TestCreateAccount_PersistsCredential(t *testing.T) {
	store := settings.NewInMemorySettingsStore()
	credStore := settings.NewMemoryCredentialStore()
	bus := config.NewEventBus()
	handler := NewSettingsHandlerWithCredentials(store, credStore, bus)
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	// Create with an api_key → credential must be persisted.
	body := `{"provider":"openai","api_key":"sk-create-key"}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/settings/accounts", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: want 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	id, _ := created["id"].(string)

	if key, err := credStore.Get(id); err != nil || key != "sk-create-key" {
		t.Fatalf("credential: want sk-create-key, got %q (err %v)", key, err)
	}

	// Create without an api_key → no credential stored.
	body2 := `{"provider":"anthropic"}`
	req2, err := http.NewRequest(http.MethodPost, server.URL+"/api/settings/accounts", strings.NewReader(body2))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp2.Body.Close()
	var created2 map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&created2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	id2, _ := created2["id"].(string)
	if credStore.Has(id2) {
		t.Fatalf("no api_key should not create a credential")
	}
}

func TestUpdateAccount_CredentialFailure(t *testing.T) {
	store := settings.NewInMemorySettingsStore()
	bus := config.NewEventBus()

	// Failing credential store: Set returns an error.
	failingCred := &failingCredentialStore{err: fmt.Errorf("disk full")}
	handler := NewSettingsHandlerWithCredentials(store, failingCred, bus)
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	account, err := store.CreateAccount("openai", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	body := `{"api_key":"sk-new-key"}`
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
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", resp.StatusCode)
	}

	// The account metadata provider must NOT have been changed by a
	// credential failure (the handler returns before UpdateAccount).
	accounts, _ := store.ListAccounts()
	for _, a := range accounts {
		if a.ID == account.ID && a.Provider != "openai" {
			t.Fatalf("provider should be unchanged on credential failure, got %q", a.Provider)
		}
	}
}

// failingCredentialStore is a CredentialStore whose Set always fails.
type failingCredentialStore struct {
	err error
}

func (f *failingCredentialStore) Get(string) (string, error) { return "", f.err }
func (f *failingCredentialStore) Set(string, string) error   { return f.err }
func (f *failingCredentialStore) Delete(string) error        { return f.err }
func (f *failingCredentialStore) Has(string) bool            { return false }

func TestRosterEndpoints(t *testing.T) {
	store := settings.NewInMemorySettingsStore()
	credStore := settings.NewMemoryCredentialStore()
	bus := config.NewEventBus()
	handler := NewSettingsHandlerWithCredentials(store, credStore, bus)
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	// A breed must exist for a roster entry to be writable.
	if err := store.CreateBreed(&pack.BreedConfig{
		ID:               "bianmu",
		Name:             "Border Collie",
		DefaultVariantID: "default",
		Variants:         []pack.Variant{{ID: "default", ClientID: "claude"}},
		Source:           pack.BreedSourceUser,
	}); err != nil {
		t.Fatalf("create breed: %v", err)
	}

	// PATCH roster (partial update: available=false, family=router).
	body := `{"available":false,"family":"router"}`
	req, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/settings/roster/bianmu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch roster: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch roster: want 200, got %d", resp.StatusCode)
	}

	// GET single entry reflects the update.
	req2, _ := http.NewRequest(http.MethodGet, server.URL+"/api/settings/roster/bianmu", nil)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("get roster: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("get roster: want 200, got %d", resp2.StatusCode)
	}
	var entry pack.RosterEntry
	if err := json.NewDecoder(resp2.Body).Decode(&entry); err != nil {
		t.Fatalf("decode entry: %v", err)
	}
	if entry.Available {
		t.Errorf("available: want false, got true")
	}
	if entry.Family != "router" {
		t.Errorf("family: want router, got %q", entry.Family)
	}

	// GET full roster contains the entry.
	req3, _ := http.NewRequest(http.MethodGet, server.URL+"/api/settings/roster", nil)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("list roster: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("list roster: want 200, got %d", resp3.StatusCode)
	}
	var roster map[string]pack.RosterEntry
	if err := json.NewDecoder(resp3.Body).Decode(&roster); err != nil {
		t.Fatalf("decode roster: %v", err)
	}
	if _, ok := roster["bianmu"]; !ok {
		t.Errorf("roster missing bianmu entry")
	}

	// Unknown breed → 404.
	req4, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/settings/roster/nope", strings.NewReader(`{"available":true}`))
	req4.Header.Set("Content-Type", "application/json")
	resp4, err := http.DefaultClient.Do(req4)
	if err != nil {
		t.Fatalf("patch unknown: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusNotFound {
		t.Fatalf("patch unknown breed: want 404, got %d", resp4.StatusCode)
	}
}

func TestReviewPolicyEndpoints(t *testing.T) {
	store := settings.NewInMemorySettingsStore()
	bus := config.NewEventBus()
	handler := NewSettingsHandlerWithCredentials(store, nil, bus)
	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	body := `{"require_different_breed":true,"prefer_active_in_thread":true}`
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/settings/review-policy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put review-policy: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put review-policy: want 200, got %d", resp.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodGet, server.URL+"/api/settings/review-policy", nil)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("get review-policy: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("get review-policy: want 200, got %d", resp2.StatusCode)
	}
	var policy pack.ReviewPolicy
	if err := json.NewDecoder(resp2.Body).Decode(&policy); err != nil {
		t.Fatalf("decode policy: %v", err)
	}
	if !policy.RequireDifferentBreed {
		t.Errorf("require_different_breed: want true, got false")
	}
	if !policy.PreferActiveInThread {
		t.Errorf("prefer_active_in_thread: want true, got false")
	}
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
	// Bind a member to this account via account_ref.
	member, err := store.CreateMember("bianmu", "Border Collie", "router", true)
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	if err := store.UpdateMember(member.ID, map[string]any{"account_ref": account.ID}); err != nil {
		t.Fatalf("bind member: %v", err)
	}

	server := httptest.NewServer(handler.Routes())
	defer server.Close()

	// Delete without force → 409 Conflict with bound_member_ids.
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
	boundIDs, ok := conflict["bound_member_ids"].([]any)
	if !ok || len(boundIDs) != 1 {
		t.Fatalf("bound_member_ids: want 1 item, got %v", conflict["bound_member_ids"])
	}
	if got, want := boundIDs[0].(string), member.ID; got != want {
		t.Fatalf("bound_member_ids[0]: want %q, got %q", want, got)
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
