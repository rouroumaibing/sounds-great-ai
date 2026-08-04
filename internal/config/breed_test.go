package config

import (
	"encoding/json"
	"testing"
)

func TestBreedConfigJSONRoundTrip(t *testing.T) {
	original := BreedConfig{
		ID:               "bianmu",
		Name:             "边牧",
		DisplayName:      "边牧",
		Personality:      "聪明、善于统筹全局",
		Roles:            []string{"orchestrator", "architect"},
		MentionPatterns:  []string{"@边牧", "@bianmu"},
		DefaultVariantID: "bianmu-claude",
		Variants: []Variant{
			{
				ID:           "bianmu-claude",
				ClientID:     "anthropic",
				DefaultModel: "claude-opus-4-6",
				MCPSupport:   true,
				CLI: CLIConfig{
					Command:      "claude",
					OutputFormat: "stream-json",
					DefaultArgs:  []string{"--output-format", "stream-json"},
				},
				ContextBudget: ContextBudget{
					MaxPromptTokens:  180000,
					MaxContextTokens: 160000,
					MaxMessages:      200,
				},
			},
		},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded BreedConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, original.ID)
	}
	if len(decoded.Variants) != 1 {
		t.Fatalf("variants len = %d, want 1", len(decoded.Variants))
	}
	if decoded.Variants[0].CLI.Command != "claude" {
		t.Errorf("CLI command = %s, want claude", decoded.Variants[0].CLI.Command)
	}
}

func TestBreedConfigNoCapabilities(t *testing.T) {
	data := `{"id":"test","name":"test","variants":[]}`
	var bc BreedConfig
	if err := json.Unmarshal([]byte(data), &bc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if bc.ID != "test" {
		t.Errorf("ID = %s, want test", bc.ID)
	}
}
