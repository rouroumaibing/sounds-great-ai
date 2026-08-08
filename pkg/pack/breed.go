package pack

// BreedSource breed 来源类型
type BreedSource string

const (
	BreedSourceSystem BreedSource = "system"
	BreedSourceUser   BreedSource = "user"
	BreedSourcePlugin BreedSource = "plugin"
)

// Color holds UI theming per breed.
type Color struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
}

// CLIConfig specifies which CLI binary and args to use.
type CLIConfig struct {
	Command      string   `json:"command"`
	OutputFormat string   `json:"output_format"`
	DefaultArgs  []string `json:"default_args,omitempty"`
	Effort       string   `json:"effort,omitempty"`
}

// ContextBudget sets per-variant token limits.
type ContextBudget struct {
	MaxPromptTokens  int `json:"max_prompt_tokens,omitempty"`
	MaxContextTokens int `json:"max_context_tokens,omitempty"`
	MaxMessages      int `json:"max_messages,omitempty"`
}

// Variant is one CLI/model configuration for a breed.
type Variant struct {
	ID            string        `json:"id"`
	ClientID      string        `json:"client_id"`
	DefaultModel  string        `json:"default_model"`
	MCPSupport    bool          `json:"mcp_support"`
	CLI           CLIConfig     `json:"cli"`
	SystemPrompt  string        `json:"system_prompt,omitempty"`
	Strengths     []string      `json:"strengths,omitempty"`
	ContextBudget ContextBudget `json:"context_budget,omitempty"`
}

// BreedConfig breed 主配置结构 (variant-based format)
type BreedConfig struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	DisplayName      string      `json:"display_name"`
	Avatar           string      `json:"avatar,omitempty"`
	Color            *Color      `json:"color,omitempty"`
	Personality      string      `json:"personality"`
	RoleDescription  string      `json:"role_description,omitempty"`
	TeamStrengths    string      `json:"team_strengths,omitempty"`
	MentionPatterns  []string    `json:"mention_patterns,omitempty"`
	Roles            []string    `json:"roles,omitempty"`
	DefaultVariantID string     `json:"default_variant_id"`
	Variants         []Variant   `json:"variants"`
	Source           BreedSource `json:"source,omitempty"`
	Enabled          bool        `json:"enabled,omitempty"`
}

// DefaultVariant returns the default variant, or the first variant if not found.
func (b *BreedConfig) DefaultVariant() *Variant {
	for i := range b.Variants {
		if b.Variants[i].ID == b.DefaultVariantID {
			return &b.Variants[i]
		}
	}
	if len(b.Variants) > 0 {
		return &b.Variants[0]
	}
	return nil
}
