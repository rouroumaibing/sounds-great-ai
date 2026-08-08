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

func TestBreedConfigReviewPolicy(t *testing.T) {
	jsonData := `{
		"id": "test",
		"name": "test",
		"display_name": "test",
		"personality": "test",
		"review_policy": {
			"can_review": ["xigou", "demu"],
			"cannot_review_self": true,
			"cross_breed_preferred": true
		},
		"default_variant_id": "v1",
		"variants": [{"id": "v1", "client_id": "test", "cli": {"command": "test"}}]
	}`
	var breed BreedConfig
	if err := json.Unmarshal([]byte(jsonData), &breed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if breed.Review == nil {
		t.Fatal("expected review_policy to be non-nil")
	}
	if len(breed.Review.CanReview) != 2 {
		t.Errorf("expected 2 can_review, got %d", len(breed.Review.CanReview))
	}
	if !breed.Review.CannotReviewSelf {
		t.Error("expected cannot_review_self to be true")
	}
}
