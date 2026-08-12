package settings_test

import (
	"path/filepath"
	"testing"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/testutil"
)

func TestInMemorySettingsStoreContract(t *testing.T) {
	testutil.RunSettingsStoreContract(t, settings.NewInMemorySettingsStore())
}

func TestFileSettingsStoreContract(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")
	catalogPath := filepath.Join(dir, "dog-catalog.json")
	testutil.RunSettingsStoreContract(t, settings.NewFileSettingsStore(accountsPath, catalogPath, false))
}

func TestMemoryCredentialStoreContract(t *testing.T) {
	testutil.RunCredentialStoreContract(t, settings.NewMemoryCredentialStore())
}
