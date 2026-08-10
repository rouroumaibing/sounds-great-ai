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
	VariantLabel  string        `json:"variant_label,omitempty"`
	ClientID      string        `json:"client_id"`
	DefaultModel  string        `json:"default_model"`
	MCPSupport    bool          `json:"mcp_support"`
	CLI           CLIConfig     `json:"cli"`
	SystemPrompt  string        `json:"system_prompt,omitempty"`
	Personality   string        `json:"personality,omitempty"`
	Strengths     []string      `json:"strengths,omitempty"`
	TeamStrengths string        `json:"team_strengths,omitempty"`
	Caution       string        `json:"caution,omitempty"`
	ContextBudget ContextBudget `json:"context_budget,omitempty"`
	VoiceConfig   VoiceConfig   `json:"voice_config,omitempty"`
}

// BreedConfig breed 主配置结构 (variant-based format)
type BreedConfig struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	DisplayName      string      `json:"display_name"`
	Nickname         string      `json:"nickname,omitempty"`
	Avatar           string      `json:"avatar,omitempty"`
	Color            *Color      `json:"color,omitempty"`
	Personality      string      `json:"personality"`
	RoleDescription  string      `json:"role_description,omitempty"`
	TeamStrengths    string      `json:"team_strengths,omitempty"`
	MentionPatterns  []string    `json:"mention_patterns,omitempty"`
	Roles            []string    `json:"roles,omitempty"`
	Caution          string      `json:"caution,omitempty"`
	DefaultVariantID string      `json:"default_variant_id"`
	Variants         []Variant   `json:"variants"`
	Source           BreedSource `json:"source,omitempty"`
	Enabled          bool        `json:"enabled,omitempty"`
}

// VoiceConfig 语音合成配置（voiceConfig）
type VoiceConfig struct {
	Voice       string  `json:"voice,omitempty"`
	LangCode    string  `json:"lang_code,omitempty"`
	Speed       float64 `json:"speed,omitempty"`
	RefAudio    string  `json:"ref_audio,omitempty"`
	RefText     string  `json:"ref_text,omitempty"`
	Instruct    string  `json:"instruct,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
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

// DogTemplateFile is the single consolidated pack config file
type DogTemplateFile struct {
	Version        int                      `json:"version"`
	ReviewPolicy   ReviewPolicy             `json:"review_policy"`
	RoleTemplates  []RoleTemplate           `json:"role_templates"`
	ClientDefaults map[string]ClientDefault `json:"client_defaults,omitempty"`
	Leader         Leader                   `json:"leader,omitempty"`
	Roster         map[string]RosterEntry   `json:"roster,omitempty"`
	Breeds         []BreedConfig            `json:"breeds"`
}

// RoleTemplate is a lightweight role preset used by the create-from-template UI.
type RoleTemplate struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Nickname        string   `json:"nickname,omitempty"`
	Avatar          string   `json:"avatar,omitempty"`
	Color           *Color   `json:"color,omitempty"`
	RoleDescription string   `json:"role_description,omitempty"`
	Personality     string   `json:"personality,omitempty"`
	TeamStrengths   string   `json:"team_strengths,omitempty"`
	DefaultRoles    []string `json:"default_roles,omitempty"`
}

// ClientDefault holds per-client model defaults .
type ClientDefault struct {
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
}

// Leader is the human owner of the pack
type Leader struct {
	Name            string   `json:"name"`
	Nickname        string   `json:"nickname,omitempty"`
	Avatar          string   `json:"avatar,omitempty"`
	TimeZone        string   `json:"time_zone,omitempty"`
	Aliases         []string `json:"aliases,omitempty"`
	MentionPatterns []string `json:"mention_patterns,omitempty"`
}

// RosterEntry is a per-instance assignment.
type RosterEntry struct {
	Family     string   `json:"family"`
	Roles      []string `json:"roles,omitempty"`
	Lead       bool     `json:"lead,omitempty"`
	Available  bool     `json:"available,omitempty"`
	Evaluation string   `json:"evaluation,omitempty"`
}

// ReviewPolicy configures cross-breed review requirements at pack level.
type ReviewPolicy struct {
	RequireDifferentBreed bool     `json:"require_different_breed"`
	PreferActiveInThread  bool     `json:"prefer_active_in_thread"`
	ExcludeUnavailable    bool     `json:"exclude_unavailable"`
	PreferredRoles        []string `json:"preferred_roles,omitempty"`
}
