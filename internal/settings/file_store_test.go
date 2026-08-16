package settings

import (
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/pkg/pack"
)

// TestFileStore_CorruptAccountsTreatedAsEmpty verifies that a corrupt
// accounts.json is treated as empty (no 500) and, per the customer-safety
// decision, is NOT auto-backed-up at load time (backups are edit-time only).
func TestFileStore_CorruptAccountsTreatedAsEmpty(t *testing.T) {
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

	// No .bak (plain or timestamped) should be written at load time.
	if _, err := os.Stat(accountsPath + ".bak"); err == nil {
		t.Errorf("corrupt accounts file should NOT be backed up at load (.bak present)")
	}
	if matches, _ := filepath.Glob(accountsPath + ".bak-*"); len(matches) > 0 {
		t.Errorf("corrupt accounts file should NOT be backed up at load (.bak-* present)")
	}
}

// TestFileStore_CorruptCredentialTreatedAsEmpty verifies a corrupt
// credentials.json is treated as empty (Get returns not found) and is NOT
// auto-backed-up at load time.
func TestFileStore_CorruptCredentialTreatedAsEmpty(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, CredentialsFileName)
	if err := os.WriteFile(credPath, []byte("@@@bad@@@"), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	store := NewFileCredentialStore(credPath, false)
	if _, err := store.Get("x"); err == nil {
		t.Fatalf("Get on corrupt store should error (not found), got nil")
	}
	if _, err := os.Stat(credPath + ".bak"); err == nil {
		t.Errorf("corrupt credential file should NOT be backed up at load (.bak present)")
	}
	if matches, _ := filepath.Glob(credPath + ".bak-*"); len(matches) > 0 {
		t.Errorf("corrupt credential file should NOT be backed up at load (.bak-* present)")
	}
}

// TestFileStore_EditCreatesTimestampedBak verifies that an edit write snapshots
// the previous file to a timestamped .bak (the customer-safety recovery point),
// and that the previous content is preserved in the backup.
func TestFileStore_EditCreatesTimestampedBak(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, AccountsFileName)
	catalogPath := filepath.Join(dir, CatalogFileName)

	store := NewFileSettingsStore(accountsPath, catalogPath, false)
	// First write establishes a baseline file.
	if _, err := store.CreateAccount("openai", "sk-old"); err != nil {
		t.Fatalf("create account: %v", err)
	}
	// Second write (edit) must create a timestamped .bak of the baseline.
	if _, err := store.CreateAccount("anthropic", "sk-new"); err != nil {
		t.Fatalf("second create account: %v", err)
	}

	matches, err := filepath.Glob(accountsPath + ".bak-*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected a timestamped .bak after edit, found none")
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read bak: %v", err)
	}
	// The backup must contain the baseline account (openai), proving it is a
	// snapshot of the pre-edit state.
	if !contains(string(raw), "openai") {
		t.Errorf("timestamped .bak is not a snapshot of the pre-edit state")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
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

// TestBreedHistory_AuditTrail (P3-b): per-breed identity changes are recorded
// with state snapshots and survive a restart, and can be purged.
func TestBreedHistory_AuditTrail(t *testing.T) {
	dir := t.TempDir()
	s := NewFileSettingsStore(filepath.Join(dir, AccountsFileName), filepath.Join(dir, CatalogFileName), false)
	b := &pack.BreedConfig{ID: "bianmu", Name: "bianmu", DisplayName: "Bianmu", Source: pack.BreedSourceUser}
	if err := s.CreateBreed(b); err != nil {
		t.Fatal(err)
	}
	b.DisplayName = "Border Collie"
	if err := s.UpdateBreed("bianmu", b); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBreed("bianmu"); err != nil {
		t.Fatal(err)
	}

	hist, err := s.ReadBreedHistory("bianmu")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 3 {
		t.Fatalf("expected 3 history entries (create/update/delete), got %d", len(hist))
	}
	if hist[0].Action != "create" || hist[1].Action != "update" || hist[2].Action != "delete" {
		t.Fatalf("unexpected action sequence: %s/%s/%s", hist[0].Action, hist[1].Action, hist[2].Action)
	}
	if hist[1].Snapshot == nil || hist[1].Snapshot.DisplayName != "Border Collie" {
		t.Errorf("update snapshot should reflect the new display name")
	}
	if hist[2].Snapshot == nil || hist[2].Snapshot.DisplayName != "Border Collie" {
		t.Errorf("delete snapshot should capture the pre-delete state")
	}

	// Reload from disk proves the audit trail survives a restart.
	s2 := NewFileSettingsStore(filepath.Join(dir, AccountsFileName), filepath.Join(dir, CatalogFileName), false)
	hist2, err := s2.ReadBreedHistory("bianmu")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist2) != 3 {
		t.Fatalf("expected history to survive reload, got %d", len(hist2))
	}

	if err := s.ClearBreedHistory("bianmu"); err != nil {
		t.Fatal(err)
	}
	hist3, _ := s.ReadBreedHistory("bianmu")
	if len(hist3) != 0 {
		t.Fatalf("expected cleared history, got %d", len(hist3))
	}
}
