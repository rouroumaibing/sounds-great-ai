package pack

import (
	"encoding/json"
	"testing"
)

func TestBreedSourceConstants(t *testing.T) {
	tests := []struct {
		name     string
		source   BreedSource
		expected string
	}{
		{"system", BreedSourceSystem, "system"},
		{"user", BreedSourceUser, "user"},
		{"plugin", BreedSourcePlugin, "plugin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.source) != tt.expected {
				t.Errorf("got %q, want %q", tt.source, tt.expected)
			}
		})
	}
}

func TestBreedConfigJSONRoundTrip(t *testing.T) {
	breed := BreedConfig{
		ID:               "bianmu",
		Name:             "bianmu",
		DisplayName:      "边牧",
		Avatar:           "🐕‍🦺",
		Personality:      "聪明、敏锐、善于统筹全局",
		MentionPatterns:  []string{"@bianmu"},
		DefaultVariantID: "v1",
		Variants: []Variant{{
			ID:           "v1",
			ClientID:     "openai",
			DefaultModel: "gpt-4o",
			MCPSupport:   false,
			CLI:          CLIConfig{Command: "claude", OutputFormat: "json"},
			SystemPrompt: "你是边牧，Pack 的协调者。",
		}},
		Source: BreedSourceSystem,
	}

	data, err := json.Marshal(breed)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded BreedConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != breed.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, breed.ID)
	}
	if decoded.DisplayName != "边牧" {
		t.Errorf("DisplayName = %q, want %q", decoded.DisplayName, "边牧")
	}
	if len(decoded.Variants) != 1 {
		t.Fatalf("Variants len = %d, want 1", len(decoded.Variants))
	}
	if decoded.Variants[0].DefaultModel != "gpt-4o" {
		t.Errorf("DefaultModel = %q, want %q", decoded.Variants[0].DefaultModel, "gpt-4o")
	}
	if decoded.Variants[0].SystemPrompt != "你是边牧，Pack 的协调者。" {
		t.Errorf("SystemPrompt = %q, want %q", decoded.Variants[0].SystemPrompt, "你是边牧，Pack 的协调者。")
	}
	if len(decoded.MentionPatterns) != 1 {
		t.Fatalf("MentionPatterns len = %d, want 1", len(decoded.MentionPatterns))
	}
	if decoded.MentionPatterns[0] != "@bianmu" {
		t.Errorf("MentionPatterns[0] = %q, want %q", decoded.MentionPatterns[0], "@bianmu")
	}
	if decoded.Source != BreedSourceSystem {
		t.Errorf("Source = %q, want %q", decoded.Source, BreedSourceSystem)
	}
}

func TestBreedConfigEmptyJSON(t *testing.T) {
	data := []byte(`{}`)
	var breed BreedConfig
	if err := json.Unmarshal(data, &breed); err != nil {
		t.Fatalf("Unmarshal empty: %v", err)
	}
	if breed.ID != "" {
		t.Errorf("ID = %q, want empty", breed.ID)
	}
	if breed.Source != "" {
		t.Errorf("Source = %q, want empty", breed.Source)
	}
}
