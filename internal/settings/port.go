package settings

// Member represents a team member (breed assigned to the pack).
type Member struct {
	ID          string `json:"id"`
	BreedID     string `json:"breed_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   int64  `json:"created_at"`

	// Account binding
	ClientID     string `json:"client_id,omitempty"`
	AccountRef   string `json:"account_ref,omitempty"`
	DefaultModel string `json:"default_model,omitempty"`
	Provider     string `json:"provider,omitempty"`

	// Identity
	Nickname       string   `json:"nickname,omitempty"`
	Avatar         string   `json:"avatar,omitempty"`
	ColorPrimary   string   `json:"color_primary,omitempty"`
	ColorSecondary string   `json:"color_secondary,omitempty"`
	MentionPatterns []string `json:"mention_patterns,omitempty"`
	Personality    string   `json:"personality,omitempty"`
	RoleDescription string  `json:"role_description,omitempty"`
	TeamStrengths  []string `json:"team_strengths,omitempty"`
	Caution        string   `json:"caution,omitempty"`

	// CLI config
	CLICommand   string `json:"cli_command,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
	DefaultArgs  string `json:"default_args,omitempty"`
	Effort       string `json:"effort,omitempty"`
	ContextWindow int   `json:"context_window,omitempty"`

	// Context budget
	MaxPromptTokens int `json:"max_prompt_tokens,omitempty"`
	MaxContextTokens int `json:"max_context_tokens,omitempty"`
	MaxMessages     int `json:"max_messages,omitempty"`

	// Session strategy
	MCPSupport   bool   `json:"mcp_support,omitempty"`
	SessionChain bool   `json:"session_chain,omitempty"`
	Strategy     string `json:"strategy,omitempty"`
}

// Account represents an API provider account.
type Account struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	KeyPreview string `json:"key_preview"`
	KeySet     bool   `json:"key_set"`
	UpdatedAt  int64  `json:"updated_at"`

	// Extended fields
	Name         string            `json:"name,omitempty"`
	ClientID     string            `json:"client_id,omitempty"`
	DisplayName  string            `json:"display_name,omitempty"`
	BaseURL      string            `json:"base_url,omitempty"`
	Models       []string          `json:"models,omitempty"`
	ModelAliases map[string]string `json:"model_aliases,omitempty"`
	EnvVars      map[string]string `json:"env_vars,omitempty"`
	AuthType     string            `json:"auth_type,omitempty"`
	Mode         string            `json:"mode,omitempty"`
	Builtin      bool              `json:"builtin,omitempty"`
}

// SystemConfig represents a system configuration key-value pair.
type SystemConfig struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Category string `json:"category"`
}

// SettingsStore is the port interface for settings storage.
type SettingsStore interface {
	ListMembers() ([]*Member, error)
	CreateMember(breedID, displayName, role string, enabled bool) (*Member, error)
	UpdateMember(id string, updates map[string]any) error
	DeleteMember(id string) error

	ListAccounts() ([]*Account, error)
	CreateAccount(provider, apiKey string) (*Account, error)
	DeleteAccount(id string) error

	ListConfig() ([]*SystemConfig, error)

	UpdateAccount(id string, updates map[string]any) error
	UpdateConfig(key, value string) error
}
