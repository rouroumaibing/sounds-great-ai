package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFileStore_CorruptAccountsBackedUp verifies that a corrupt accounts.json
// is backed up to .bak and the store treats it as empty (no 500).
func TestFileStore_CorruptAccountsBackedUp(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, AccountsFileName)
	catalogPath := filepath.Join(dir, CatalogFileName)

	// Write a corrupt accounts file.
	if err := os.WriteFile(accountsPath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	// Catalog is valid/empty.
	if err := os.WriteFile(catalogPath, []byte(`{"members":[]}`), 0o644); err != nil {
		t.Fatalf("write catalog: %v", err)
	}

	store := NewFileSettingsStore(accountsPath, catalogPath, false)
	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts should not error on corrupt file: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("corrupt accounts should be treated as empty, got %d", len(accounts))
	}

	bak := accountsPath + ".bak"
	if _, err := os.Stat(bak); err != nil {
		t.Fatalf("corrupt accounts file should be backed up to .bak: %v", err)
	}
	raw, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("read bak: %v", err)
	}
	if string(raw) != "{not valid json" {
		t.Fatalf("bak content mismatch: %q", string(raw))
	}
}

// TestFileStore_CorruptCredentialBackedUp verifies a corrupt credentials.json
// is backed up and treated as empty (Get returns not found).
func TestFileStore_CorruptCredentialBackedUp(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, CredentialsFileName)
	if err := os.WriteFile(credPath, []byte("@@@bad@@@"), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	store := NewFileCredentialStore(credPath, false)
	if _, err := store.Get("x"); err == nil {
		t.Fatalf("Get on corrupt store should error (not found), got nil")
	}
	if _, err := os.Stat(credPath + ".bak"); err != nil {
		t.Fatalf("corrupt credential file should be backed up: %v", err)
	}
}

// TestFileStore_EnvVarReservedPrefix verifies that env_vars keys with the
// SOUNDS_GREAT_AI_ prefix are dropped when updating an account.
func TestFileStore_EnvVarReservedPrefix(t *testing.T) {
	dir := t.TempDir()
	store := NewFileSettingsStore(
		filepath.Join(dir, AccountsFileName),
		filepath.Join(dir, CatalogFileName),
		false,
	)
	account, err := store.CreateAccount("openai", "")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	err = store.UpdateAccount(account.ID, map[string]any{
		"env_vars": map[string]string{
			"SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT": "/tmp/x",
			"MY_CUSTOM_VAR":                      "ok",
		},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	accounts, _ := store.ListAccounts()
	var got map[string]string
	for _, a := range accounts {
		if a.ID == account.ID {
			got = a.EnvVars
		}
	}
	if _, ok := got["SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT"]; ok {
		t.Fatalf("reserved prefix key must be dropped, got %v", got)
	}
	if got["MY_CUSTOM_VAR"] != "ok" {
		t.Fatalf("non-reserved key must be kept, got %v", got)
	}
}
