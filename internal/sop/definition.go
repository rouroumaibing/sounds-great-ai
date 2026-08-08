package sop

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SOPDefinition is the top-level SOP definition loaded from YAML.
type SOPDefinition struct {
	ID          string     `yaml:"id"`
	Domain      string     `yaml:"domain"`
	Label       string     `yaml:"label"`
	Description string     `yaml:"description"`
	Stages      []SOPStage `yaml:"stages"`
}

// SOPStage represents one stage/lane in the development SOP.
type SOPStage struct {
	ID            string     `yaml:"id"`
	Label         string     `yaml:"label"`
	SuggestedSkill string    `yaml:"suggested_skill"`
	Optional      bool       `yaml:"optional"`
	HardRules     []HardRule `yaml:"hard_rules"`
	Pitfalls      []HardRule `yaml:"pitfalls"`
}

// HardRule is a machine-checkable rule with severity and predicate.
type HardRule struct {
	ID        string    `yaml:"id"`
	Text      string    `yaml:"text"`
	Severity  string    `yaml:"severity"` // blocker | warn
	Predicate Predicate `yaml:"predicate"`
}

// Predicate describes a machine-checkable condition.
// Only fields relevant to the Type are populated.
type Predicate struct {
	Type           string   `yaml:"type"` // git_state | command_pattern | handle_check | env_check | command_sequence | sha_dedup | manual_only
	Reason         string   `yaml:"reason"`
	Branch         string   `yaml:"branch"`
	Checks         []string `yaml:"checks"`           // git_state: ahead_zero, behind_zero, not_dirty
	BeforeCommand  string   `yaml:"before_command"`
	MustMatch      string   `yaml:"must_match"`       // command_pattern
	MustNotMatch   string   `yaml:"must_not_match"`   // command_pattern
	Constraint     string   `yaml:"constraint"`       // handle_check
	Key            string   `yaml:"key"`              // env_check
	MustInclude    any      `yaml:"must_include"`     // env_check (string) or command_sequence ([]string)
	MustNotInclude string   `yaml:"must_not_include"` // env_check
	Scope          string   `yaml:"scope"`            // sha_dedup
	AntiPattern    []string `yaml:"anti_pattern"`     // command_sequence
	Absent         []string `yaml:"absent"`           // command_sequence
	CwdContains    string   `yaml:"cwd_contains"`     // command_sequence
}

// MustIncludeString returns MustInclude as a string (for env_check).
func (p *Predicate) MustIncludeString() string {
	if s, ok := p.MustInclude.(string); ok {
		return s
	}
	return ""
}

// MustIncludeList returns MustInclude as a string slice (for command_sequence).
func (p *Predicate) MustIncludeList() []string {
	if list, ok := p.MustInclude.([]any); ok {
		result := make([]string, 0, len(list))
		for _, v := range list {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// LoadDefinition reads and parses a SOP definition YAML file.
func LoadDefinition(path string) (*SOPDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read sop definition %s: %w", path, err)
	}
	var def SOPDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse sop definition %s: %w", path, err)
	}
	return &def, nil
}

// FindStage returns the stage with the given ID, or nil if not found.
func (d *SOPDefinition) FindStage(stageID string) *SOPStage {
	for i := range d.Stages {
		if d.Stages[i].ID == stageID {
			return &d.Stages[i]
		}
	}
	return nil
}

// BlockerRules returns all blocker-severity hard_rules for a stage.
func (s *SOPStage) BlockerRules() []HardRule {
	var result []HardRule
	for _, r := range s.HardRules {
		if r.Severity == "blocker" {
			result = append(result, r)
		}
	}
	return result
}

// StageIDs returns the ordered list of stage IDs.
func (d *SOPDefinition) StageIDs() []string {
	ids := make([]string, len(d.Stages))
	for i, s := range d.Stages {
		ids[i] = s.ID
	}
	return ids
}
