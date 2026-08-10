package config

import "fmt"

// LeaderConfig holds the leader (human operator) configuration.
type LeaderConfig struct {
	Name            string   `json:"name" yaml:"name"`
	Aliases         []string `json:"aliases" yaml:"aliases"`
	MentionPatterns []string `json:"mentionPatterns" yaml:"mentionPatterns"`
	TimeZone        string   `json:"timeZone,omitempty" yaml:"timeZone,omitempty"`
	Avatar          string   `json:"avatar,omitempty" yaml:"avatar,omitempty"`
	ColorPrimary    string   `json:"colorPrimary,omitempty" yaml:"colorPrimary,omitempty"`
	ColorSecondary  string   `json:"colorSecondary,omitempty" yaml:"colorSecondary,omitempty"`
}

// DefaultLeaderConfig returns the default leader configuration.
func DefaultLeaderConfig() LeaderConfig {
	return LeaderConfig{
		Name:            "You",
		Aliases:         []string{"Owner"},
		MentionPatterns: []string{"@You", "@leader", "@owner"},
		TimeZone:        "Asia/Shanghai",
	}
}

// Validate checks that the leader config is valid.
func (l *LeaderConfig) Validate() error {
	if l.Name == "" {
		return fmt.Errorf("leader name cannot be empty")
	}
	if len(l.MentionPatterns) == 0 {
		return fmt.Errorf("leader must have at least one mention pattern")
	}
	return nil
}
