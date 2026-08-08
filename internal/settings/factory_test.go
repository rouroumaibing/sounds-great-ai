package settings

import "testing"

func TestNewSettingsStore(t *testing.T) {
	store := NewSettingsStore()
	if store == nil {
		t.Fatal("NewSettingsStore returned nil")
	}
	m, err := store.CreateMember("bianmu", "Test", "router", true)
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	if m.ID == "" {
		t.Fatal("member ID is empty")
	}
}
