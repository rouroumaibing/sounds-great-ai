package settings

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemorySettingsStore implements SettingsStore with in-memory data structures.
type InMemorySettingsStore struct {
	mu       sync.RWMutex
	members  map[string]*Member
	accounts map[string]*Account
	configs  []*SystemConfig
}

// NewInMemorySettingsStore creates a new in-memory settings store.
func NewInMemorySettingsStore() *InMemorySettingsStore {
	return &InMemorySettingsStore{
		members:  make(map[string]*Member),
		accounts: make(map[string]*Account),
		configs:  defaultConfig(),
	}
}

func (s *InMemorySettingsStore) ListMembers() ([]*Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Member, 0, len(s.members))
	for _, m := range s.members {
		result = append(result, m)
	}
	return result, nil
}

func (s *InMemorySettingsStore) CreateMember(breedID, displayName, role string, enabled bool) (*Member, error) {
	m := &Member{
		ID:          uuid.NewString(),
		BreedID:     breedID,
		DisplayName: displayName,
		Role:        role,
		Enabled:     enabled,
		CreatedAt:   time.Now().UnixMilli(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[m.ID] = m
	return m, nil
}

func (s *InMemorySettingsStore) UpdateMember(id string, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[id]
	if !ok {
		return fmt.Errorf("member %q not found", id)
	}
	applyMemberUpdates(m, updates)
	return nil
}

// applyMemberUpdates mutates a Member in place from a loose-typed updates map.
func applyMemberUpdates(m *Member, updates map[string]any) {
	if v, ok := updates["display_name"]; ok {
		if s, ok := v.(string); ok {
			m.DisplayName = s
		}
	}
	if v, ok := updates["role"]; ok {
		if s, ok := v.(string); ok {
			m.Role = s
		}
	}
	if v, ok := updates["enabled"]; ok {
		if b, ok := v.(bool); ok {
			m.Enabled = b
		}
	}
	if v, ok := updates["breed_id"]; ok {
		if s, ok := v.(string); ok {
			m.BreedID = s
		}
	}
	// Account binding
	if v, ok := updates["client_id"]; ok {
		if s, ok := v.(string); ok {
			m.ClientID = s
		}
	}
	if v, ok := updates["account_ref"]; ok {
		if s, ok := v.(string); ok {
			m.AccountRef = s
		}
	}
	if v, ok := updates["default_model"]; ok {
		if s, ok := v.(string); ok {
			m.DefaultModel = s
		}
	}
	if v, ok := updates["provider"]; ok {
		if s, ok := v.(string); ok {
			m.Provider = s
		}
	}
	// Identity
	if v, ok := updates["nickname"]; ok {
		if s, ok := v.(string); ok {
			m.Nickname = s
		}
	}
	if v, ok := updates["avatar"]; ok {
		if s, ok := v.(string); ok {
			m.Avatar = s
		}
	}
	if v, ok := updates["color_primary"]; ok {
		if s, ok := v.(string); ok {
			m.ColorPrimary = s
		}
	}
	if v, ok := updates["color_secondary"]; ok {
		if s, ok := v.(string); ok {
			m.ColorSecondary = s
		}
	}
	if v, ok := updates["mention_patterns"]; ok {
		if ss, ok := v.([]string); ok {
			m.MentionPatterns = ss
		}
	}
	if v, ok := updates["personality"]; ok {
		if s, ok := v.(string); ok {
			m.Personality = s
		}
	}
	if v, ok := updates["role_description"]; ok {
		if s, ok := v.(string); ok {
			m.RoleDescription = s
		}
	}
	if v, ok := updates["team_strengths"]; ok {
		if ss, ok := v.([]string); ok {
			m.TeamStrengths = ss
		}
	}
	if v, ok := updates["caution"]; ok {
		if s, ok := v.(string); ok {
			m.Caution = s
		}
	}
	// CLI config
	if v, ok := updates["cli_command"]; ok {
		if s, ok := v.(string); ok {
			m.CLICommand = s
		}
	}
	if v, ok := updates["output_format"]; ok {
		if s, ok := v.(string); ok {
			m.OutputFormat = s
		}
	}
	if v, ok := updates["default_args"]; ok {
		if s, ok := v.(string); ok {
			m.DefaultArgs = s
		}
	}
	if v, ok := updates["effort"]; ok {
		if s, ok := v.(string); ok {
			m.Effort = s
		}
	}
	if v, ok := updates["context_window"]; ok {
		if n, ok := toInt(v); ok {
			m.ContextWindow = n
		}
	}
	// Context budget
	if v, ok := updates["max_prompt_tokens"]; ok {
		if n, ok := toInt(v); ok {
			m.MaxPromptTokens = n
		}
	}
	if v, ok := updates["max_context_tokens"]; ok {
		if n, ok := toInt(v); ok {
			m.MaxContextTokens = n
		}
	}
	if v, ok := updates["max_messages"]; ok {
		if n, ok := toInt(v); ok {
			m.MaxMessages = n
		}
	}
	// Session strategy
	if v, ok := updates["mcp_support"]; ok {
		if b, ok := v.(bool); ok {
			m.MCPSupport = b
		}
	}
	if v, ok := updates["session_chain"]; ok {
		if b, ok := v.(bool); ok {
			m.SessionChain = b
		}
	}
	if v, ok := updates["strategy"]; ok {
		if s, ok := v.(string); ok {
			m.Strategy = s
		}
	}
}

func (s *InMemorySettingsStore) DeleteMember(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.members[id]; !ok {
		return fmt.Errorf("member %q not found", id)
	}
	delete(s.members, id)
	return nil
}

func (s *InMemorySettingsStore) ListAccounts() ([]*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Account, 0, len(s.accounts))
	for _, a := range s.accounts {
		result = append(result, a)
	}
	return result, nil
}

func (s *InMemorySettingsStore) CreateAccount(provider, apiKey string) (*Account, error) {
	a := &Account{
		ID:         uuid.NewString(),
		Provider:   provider,
		KeyPreview: maskKey(apiKey),
		KeySet:     apiKey != "",
		UpdatedAt:  time.Now().UnixMilli(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[a.ID] = a
	return a, nil
}

func (s *InMemorySettingsStore) DeleteAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[id]; !ok {
		return fmt.Errorf("account %q not found", id)
	}
	delete(s.accounts, id)
	return nil
}

func (s *InMemorySettingsStore) ListConfig() ([]*SystemConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*SystemConfig, len(s.configs))
	copy(result, s.configs)
	return result, nil
}

func (s *InMemorySettingsStore) UpdateAccount(id string, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok {
		return fmt.Errorf("account %q not found", id)
	}
	if v, ok := updates["provider"]; ok {
		if s, ok := v.(string); ok {
			a.Provider = s
		}
	}
	if v, ok := updates["api_key"]; ok {
		if s, ok := v.(string); ok {
			a.KeyPreview = maskKey(s)
			a.KeySet = s != ""
		}
	}
	// Extended fields
	if v, ok := updates["name"]; ok {
		if s, ok := v.(string); ok {
			a.Name = s
		}
	}
	if v, ok := updates["client_id"]; ok {
		if s, ok := v.(string); ok {
			a.ClientID = s
		}
	}
	if v, ok := updates["display_name"]; ok {
		if s, ok := v.(string); ok {
			a.DisplayName = s
		}
	}
	if v, ok := updates["base_url"]; ok {
		if s, ok := v.(string); ok {
			a.BaseURL = s
		}
	}
	if v, ok := updates["models"]; ok {
		if ss, ok := v.([]string); ok {
			a.Models = ss
		}
	}
	if v, ok := updates["model_aliases"]; ok {
		if mm, ok := v.(map[string]string); ok {
			a.ModelAliases = mm
		}
	}
	if v, ok := updates["env_vars"]; ok {
		if mm, ok := v.(map[string]string); ok {
			a.EnvVars = mm
		}
	}
	if v, ok := updates["auth_type"]; ok {
		if s, ok := v.(string); ok {
			a.AuthType = s
		}
	}
	if v, ok := updates["mode"]; ok {
		if s, ok := v.(string); ok {
			a.Mode = s
		}
	}
	if v, ok := updates["builtin"]; ok {
		if b, ok := v.(bool); ok {
			a.Builtin = b
		}
	}
	a.UpdatedAt = time.Now().UnixMilli()
	return nil
}

func (s *InMemorySettingsStore) UpdateConfig(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.configs {
		if c.Key == key {
			c.Value = value
			return nil
		}
	}
	return fmt.Errorf("config key %q not found", key)
}

func maskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:2] + "****" + key[len(key)-2:]
}

// toInt converts a loose-typed value from a map[string]any to an int.
// JSON decoding produces float64 for numbers.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func defaultConfig() []*SystemConfig {
	return []*SystemConfig{
		{Key: "max_concurrent_barks", Value: "8", Category: "runtime"},
		{Key: "ws_ping_interval", Value: "30s", Category: "runtime"},
		{Key: "rag_top_k", Value: "5", Category: "rag"},
		{Key: "rag_similarity_threshold", Value: "0.7", Category: "rag"},
		{Key: "default_breed", Value: "bianmu", Category: "routing"},
		{Key: "review_policy", Value: "cross_breed", Category: "routing"},
	}
}
