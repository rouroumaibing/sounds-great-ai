package settings_test

import (
	"testing"

	"sounds-great-ai/internal/settings"
	"sounds-great-ai/testutil"
)

func TestInMemorySettingsStoreContract(t *testing.T) {
	testutil.RunSettingsStoreContract(t, settings.NewInMemorySettingsStore())
}

func TestMemoryCredentialStoreContract(t *testing.T) {
	testutil.RunCredentialStoreContract(t, settings.NewMemoryCredentialStore())
}
