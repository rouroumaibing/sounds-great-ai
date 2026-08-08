package settings

import (
	"testing"
)

func TestInMemorySettingsStore_UpdateMemberNotFound(t *testing.T) {
	s := NewInMemorySettingsStore()
	err := s.UpdateMember("nonexistent", map[string]any{"display_name": "x"})
	if err == nil {
		t.Fatal("expected error for nonexistent member")
	}
}

func TestInMemorySettingsStore_DeleteAccountNotFound(t *testing.T) {
	s := NewInMemorySettingsStore()
	err := s.DeleteAccount("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent account")
	}
}

func TestInMemorySettingsStore_UpdateConfigNotFound(t *testing.T) {
	s := NewInMemorySettingsStore()
	err := s.UpdateConfig("nonexistent", "value")
	if err == nil {
		t.Fatal("expected error for nonexistent config key")
	}
}

func TestInMemorySettingsStore_UpdateAccount(t *testing.T) {
	s := NewInMemorySettingsStore()
	a, _ := s.CreateAccount("anthropic", "sk-test")
	err := s.UpdateAccount(a.ID, map[string]any{"provider": "openai", "api_key": "sk-new"})
	if err != nil {
		t.Fatalf("UpdateAccount: %v", err)
	}
	accounts, _ := s.ListAccounts()
	if accounts[0].Provider != "openai" {
		t.Errorf("Provider = %q, want %q", accounts[0].Provider, "openai")
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "short key", input: "ab", want: "****"},
		{name: "4 chars", input: "abcd", want: "****"},
		{name: "long key", input: "sk-1234567890", want: "sk****90"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskKey(tt.input)
			if got != tt.want {
				t.Errorf("maskKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
