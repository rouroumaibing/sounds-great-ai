package config

// BreedConfig is the new breed configuration format.
// No capabilities[], no workflow[], no single model_config.
// Abilities come from CLI tools + skills + MCP, not Go code.
type BreedConfig struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	DisplayName     string    `json:"display_name"`
	Avatar          string    `json:"avatar,omitempty"`
	Color           *Color    `json:"color,omitempty"`
	Personality     string    `json:"personality"`
	RoleDescription string    `json:"role_description,omitempty"`
	TeamStrengths   string    `json:"team_strengths,omitempty"`
	MentionPatterns []string  `json:"mention_patterns,omitempty"`
	Roles           []string  `json:"roles,omitempty"`

	Review *BreedReviewPolicy `json:"review_policy,omitempty"`

	DefaultVariantID string   `json:"default_variant_id"`
	Variants         []Variant `json:"variants"`
}

// Color holds UI theming per breed.
type Color struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
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

// BreedReviewPolicy configures per-breed review rules.
type BreedReviewPolicy struct {
	CanReview           []string `json:"can_review,omitempty"`
	CannotReviewSelf    bool     `json:"cannot_review_self"`
	CrossBreedPreferred bool     `json:"cross_breed_preferred"`
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

// PackConfig is the pack-level configuration.
type PackConfig struct {
	ReviewPolicy    ReviewPolicy  `json:"review_policy"`
	RoutingRules    []RoutingRule `json:"routing_rules,omitempty"`
	SOPDefinitions []string      `json:"sop_definitions,omitempty"`
}

// ReviewPolicy configures cross-model review requirements.
type ReviewPolicy struct {
	RequireDifferentBreed bool     `json:"require_different_breed"`
	PreferActiveInThread  bool     `json:"prefer_active_in_thread"`
	ExcludeUnavailable    bool     `json:"exclude_unavailable"`
	PreferredRoles        []string `json:"preferred_roles,omitempty"`
}

// RoutingRule maps task types to breeds/roles.
type RoutingRule struct {
	TaskType          string   `json:"task_type"`
	AssignRoles       []string `json:"assign_roles,omitempty"`
	Flow              []string `json:"flow,omitempty"`
	RequireCrossBreed bool     `json:"require_cross_breed,omitempty"`
	ReviewRequired    bool     `json:"review_required,omitempty"`
	Skills            []string `json:"skills,omitempty"`
}
