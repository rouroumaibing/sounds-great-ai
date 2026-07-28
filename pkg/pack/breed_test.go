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
		ID:           "bianmu",
		Name:         "bianmu",
		DisplayName:  "边牧",
		Avatar:       "🐕‍🦺",
		Personality:  "聪明、敏锐、善于统筹全局",
		SystemPrompt: "你是边牧，Pack 的协调者。",
		ModelConfig: ModelConfig{
			Provider:       "openai",
			Model:          "gpt-4o",
			Temperature:    0.3,
			MaxTokens:      4096,
			TimeoutSeconds: 30,
		},
		Capabilities: []CapabilityBinding{
			{Name: "task_decompose", Version: "v1"},
			{Name: "agent_dispatch", Version: "v1", Config: map[string]any{"top_k": 5}},
		},
		Workflow: WorkflowConfig{
			Steps: []WorkflowStep{
				{ID: "decompose", CapabilityRef: "task_decompose:v1"},
				{ID: "dispatch", CapabilityRef: "agent_dispatch:v1", Depends: []string{"decompose"}},
			},
		},
		Security: SecurityPolicy{
			Network:    true,
			Filesystem: FileSystemPolicy{Write: false, Paths: []string{}},
		},
		Mentions: []string{"zhonghuatianyuanquan", "zangao"},
		Source:   BreedSourceSystem,
		Version:  "v1",
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
	if decoded.ModelConfig.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %d, want 30", decoded.ModelConfig.TimeoutSeconds)
	}
	if len(decoded.Capabilities) != 2 {
		t.Fatalf("Capabilities len = %d, want 2", len(decoded.Capabilities))
	}
	if decoded.Capabilities[1].Config["top_k"] != float64(5) {
		t.Errorf("Config top_k = %v, want 5", decoded.Capabilities[1].Config["top_k"])
	}
	if len(decoded.Workflow.Steps) != 2 {
		t.Fatalf("Workflow.Steps len = %d, want 2", len(decoded.Workflow.Steps))
	}
	if decoded.Workflow.Steps[1].Depends[0] != "decompose" {
		t.Errorf("Depends[0] = %q, want %q", decoded.Workflow.Steps[1].Depends[0], "decompose")
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
