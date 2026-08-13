package settings

import (
	"errors"

	"sounds-great-ai/pkg/pack"
)

// ErrBreedNotFound is returned by DeleteBreed when the breed is absent from the
// catalog. The pack-api handler tolerates this on a delete that already
// succeeded against the in-memory pack (idempotent removal).
var ErrBreedNotFound = errors.New("breed not found")

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
	Nickname        string   `json:"nickname,omitempty"`
	Avatar          string   `json:"avatar,omitempty"`
	ColorPrimary    string   `json:"color_primary,omitempty"`
	ColorSecondary  string   `json:"color_secondary,omitempty"`
	MentionPatterns []string `json:"mention_patterns,omitempty"`
	Personality     string   `json:"personality,omitempty"`
	RoleDescription string   `json:"role_description,omitempty"`
	TeamStrengths   []string `json:"team_strengths,omitempty"`
	Caution         string   `json:"caution,omitempty"`

	// CLI config
	CLICommand    string `json:"cli_command,omitempty"`
	OutputFormat  string `json:"output_format,omitempty"`
	DefaultArgs   string `json:"default_args,omitempty"`
	Effort        string `json:"effort,omitempty"`
	ContextWindow int    `json:"context_window,omitempty"`

	// Context budget
	MaxPromptTokens  int `json:"max_prompt_tokens,omitempty"`
	MaxContextTokens int `json:"max_context_tokens,omitempty"`
	MaxMessages      int `json:"max_messages,omitempty"`

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
	// Deprecated: members are stored as breeds (clowder-homologous). These
	// methods remain only to keep the legacy /api/settings/members endpoint
	// working until it is removed in favor of /api/breeds.
	ListMembers() ([]*Member, error)
	CreateMember(breedID, displayName, role string, enabled bool) (*Member, error)
	UpdateMember(id string, updates map[string]any) error
	DeleteMember(id string) error

	// Breeds (runtime member catalog; clowder-homologous structure).
	ListBreeds() ([]*pack.BreedConfig, error)
	CreateBreed(b *pack.BreedConfig) error
	UpdateBreed(id string, b *pack.BreedConfig) error
	DeleteBreed(id string) error
	// ListDeletedBreeds returns IDs of breeds the customer explicitly removed,
	// so the upgrade sync can skip resurrecting them (decision D2).
	ListDeletedBreeds() ([]string, error)
	// ListSeenTemplateBreeds returns the IDs of template breeds the catalog has
	// already been "exposed to". Breeds in this set are never auto-added again
	// (no resurrection). It unifies D1 (empty first run) and D3 (auto-add new
	// template breeds on upgrade); see docs/DESIGN-STORYS/SG-MEM-002.
	ListSeenTemplateBreeds() ([]string, error)
	// AddSeenTemplateBreeds marks the given template breed IDs as seen and
	// persists them. Called by the upgrade sync when a new template breed is
	// added, and on first run (all template IDs) so a later restart does not
	// re-inject an empty catalog.
	AddSeenTemplateBreeds(ids []string) error
	// CatalogFileExists reports whether the runtime catalog file exists on disk.
	// It distinguishes a first run (file absent → write empty catalog + seen)
	// from an existing customer catalog.
	CatalogFileExists() bool
	// ReorderBreeds reorders the persisted catalog breeds[] array to match the
	// given order (clowder-homologous: the array order is the sort truth).
	// IDs not present in the catalog are ignored; catalog breeds missing from
	// order keep their previous relative position appended at the end.
	ReorderBreeds(order []string) error

	// Roster: per-breed runtime assignment (available/roles/lead/family/evaluation).
	GetRoster() (map[string]pack.RosterEntry, error)
	UpdateRosterEntry(id string, e pack.RosterEntry) error
	DeleteRosterEntry(id string) error

	// ReviewPolicy: pack-level review configuration.
	GetReviewPolicy() (*pack.ReviewPolicy, error)
	UpdateReviewPolicy(p *pack.ReviewPolicy) error

	ListAccounts() ([]*Account, error)
	CreateAccount(provider, apiKey string) (*Account, error)
	DeleteAccount(id string) error

	ListConfig() ([]*SystemConfig, error)

	UpdateAccount(id string, updates map[string]any) error
	UpdateConfig(key, value string) error
	// UpsertConfig creates or updates a system config key. Unlike UpdateConfig
	// it does not require the key to pre-exist (used for new keys like repo_url).
	UpsertConfig(key, value string) error

	// Leader (operator) config, persisted in dog-catalog.json.
	GetLeader() (*pack.Leader, error)
	UpdateLeader(l *pack.Leader) error
}
