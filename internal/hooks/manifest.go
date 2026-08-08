package hooks

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type HookManifest struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	Stage          string   `yaml:"stage"`
	Order          int      `yaml:"order"`
	Version        int      `yaml:"version"`
	Enabled        bool     `yaml:"enabled"`
	Disableable    bool     `yaml:"disableable"`
	Template       string   `yaml:"template"`
	Resolver       string   `yaml:"resolver"`
	Inputs         []string `yaml:"inputs"`
	SafetyTier     string   `yaml:"safetyTier"`
	GovernanceTier string   `yaml:"governanceTier"`
}

func ParseHookManifest(dir string) (*HookManifest, error) {
	yamlPath := filepath.Join(dir, "hook.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("read hook.yaml: %w", err)
	}
	var m HookManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse hook.yaml: %w", err)
	}
	if m.ID == "" {
		return nil, fmt.Errorf("hook ID is required")
	}
	if m.Stage == "" {
		return nil, fmt.Errorf("hook stage is required")
	}
	if m.Template != "" {
		tmplPath := filepath.Join(dir, m.Template)
		if _, err := os.Stat(tmplPath); err != nil {
			return nil, fmt.Errorf("template file not found: %s", m.Template)
		}
	}
	return &m, nil
}
