package stores

import (
	settingsPorts "sounds-great-ai/internal/domains/settings/ports"
	"sounds-great-ai/internal/settings"
)

// SettingsStoreAdapter wraps an existing settings.SettingsStore to implement
// the domain ports.ISettingsStore interface.
type SettingsStoreAdapter struct {
	inner settings.SettingsStore
}

// NewSettingsStoreAdapter creates a new SettingsStoreAdapter.
func NewSettingsStoreAdapter(inner settings.SettingsStore) *SettingsStoreAdapter {
	return &SettingsStoreAdapter{inner: inner}
}

func (a *SettingsStoreAdapter) ListMembers() ([]*settingsPorts.Member, error) {
	members, err := a.inner.ListMembers()
	if err != nil {
		return nil, err
	}
	result := make([]*settingsPorts.Member, len(members))
	for i, m := range members {
		result[i] = convertMember(m)
	}
	return result, nil
}

func (a *SettingsStoreAdapter) CreateMember(breedID, displayName, role string, enabled bool) (*settingsPorts.Member, error) {
	m, err := a.inner.CreateMember(breedID, displayName, role, enabled)
	if err != nil {
		return nil, err
	}
	return convertMember(m), nil
}

func (a *SettingsStoreAdapter) UpdateMember(id string, updates map[string]any) error {
	return a.inner.UpdateMember(id, updates)
}

func (a *SettingsStoreAdapter) DeleteMember(id string) error {
	return a.inner.DeleteMember(id)
}

func (a *SettingsStoreAdapter) ListAccounts() ([]*settingsPorts.Account, error) {
	accounts, err := a.inner.ListAccounts()
	if err != nil {
		return nil, err
	}
	result := make([]*settingsPorts.Account, len(accounts))
	for i, acc := range accounts {
		result[i] = convertAccount(acc)
	}
	return result, nil
}

func (a *SettingsStoreAdapter) CreateAccount(provider, apiKey string) (*settingsPorts.Account, error) {
	acc, err := a.inner.CreateAccount(provider, apiKey)
	if err != nil {
		return nil, err
	}
	return convertAccount(acc), nil
}

func (a *SettingsStoreAdapter) DeleteAccount(id string) error {
	return a.inner.DeleteAccount(id)
}

func (a *SettingsStoreAdapter) ListConfig() ([]*settingsPorts.SystemConfig, error) {
	configs, err := a.inner.ListConfig()
	if err != nil {
		return nil, err
	}
	result := make([]*settingsPorts.SystemConfig, len(configs))
	for i, c := range configs {
		result[i] = &settingsPorts.SystemConfig{Key: c.Key, Value: c.Value, Category: c.Category}
	}
	return result, nil
}

func (a *SettingsStoreAdapter) UpdateAccount(id string, updates map[string]any) error {
	return a.inner.UpdateAccount(id, updates)
}

func (a *SettingsStoreAdapter) UpdateConfig(key, value string) error {
	return a.inner.UpdateConfig(key, value)
}

func convertMember(m *settings.Member) *settingsPorts.Member {
	return &settingsPorts.Member{
		ID:              m.ID,
		BreedID:         m.BreedID,
		DisplayName:     m.DisplayName,
		Role:            m.Role,
		Enabled:         m.Enabled,
		CreatedAt:       m.CreatedAt,
		ClientID:        m.ClientID,
		AccountRef:      m.AccountRef,
		DefaultModel:    m.DefaultModel,
		Provider:        m.Provider,
		Nickname:        m.Nickname,
		Avatar:          m.Avatar,
		ColorPrimary:    m.ColorPrimary,
		ColorSecondary:  m.ColorSecondary,
		MentionPatterns: m.MentionPatterns,
		Personality:     m.Personality,
		RoleDescription: m.RoleDescription,
		TeamStrengths:   m.TeamStrengths,
		Caution:         m.Caution,
		CLICommand:      m.CLICommand,
		OutputFormat:    m.OutputFormat,
		DefaultArgs:     m.DefaultArgs,
		Effort:          m.Effort,
		ContextWindow:   m.ContextWindow,
		MaxPromptTokens: m.MaxPromptTokens,
		MaxContextTokens: m.MaxContextTokens,
		MaxMessages:     m.MaxMessages,
		MCPSupport:      m.MCPSupport,
		SessionChain:    m.SessionChain,
		Strategy:        m.Strategy,
	}
}

func convertAccount(a *settings.Account) *settingsPorts.Account {
	return &settingsPorts.Account{
		ID:           a.ID,
		Provider:     a.Provider,
		KeyPreview:   a.KeyPreview,
		KeySet:       a.KeySet,
		UpdatedAt:    a.UpdatedAt,
		Name:         a.Name,
		ClientID:     a.ClientID,
		DisplayName:  a.DisplayName,
		BaseURL:      a.BaseURL,
		Models:       a.Models,
		ModelAliases: a.ModelAliases,
		EnvVars:      a.EnvVars,
		AuthType:     a.AuthType,
		Mode:         a.Mode,
		Builtin:      a.Builtin,
	}
}
