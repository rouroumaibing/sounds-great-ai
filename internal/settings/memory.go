package settings

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"sounds-great-ai/pkg/pack"
)

// InMemorySettingsStore implements SettingsStore with in-memory data structures.
// It mirrors FileSettingsStore: breeds + roster + review_policy are the source
// of truth (clowder-homologous); the deprecated Member methods are projected
// onto that structure via the shared helpers breedToMember/applyBreedUpdates.
type InMemorySettingsStore struct {
	mu            sync.RWMutex
	breeds        map[string]*pack.BreedConfig
	breedOrder    []string
	roster        map[string]pack.RosterEntry
	reviewPolicy  *pack.ReviewPolicy
	accounts      map[string]*Account
	configs       []*SystemConfig
	leader        *pack.Leader
	deletedBreeds map[string]bool
	seenTemplateBreeds map[string]bool
}

// NewInMemorySettingsStore creates a new in-memory settings store.
func NewInMemorySettingsStore() *InMemorySettingsStore {
	l := pack.DefaultLeaderConfig()
	return &InMemorySettingsStore{
		breeds:        make(map[string]*pack.BreedConfig),
		roster:        make(map[string]pack.RosterEntry),
		accounts:      make(map[string]*Account),
		configs:       defaultConfig(),
		leader:        &l,
		deletedBreeds: make(map[string]bool),
		seenTemplateBreeds: make(map[string]bool),
	}
}

// ---------------------------------------------------------------------------
// Breeds (runtime member catalog)
// ---------------------------------------------------------------------------

func (s *InMemorySettingsStore) ListBreeds() ([]*pack.BreedConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*pack.BreedConfig, 0, len(s.breeds))
	seen := make(map[string]bool, len(s.breedOrder))
	for _, id := range s.breedOrder {
		if b, ok := s.breeds[id]; ok {
			out = append(out, b)
			seen[id] = true
		}
	}
	for id, b := range s.breeds {
		if !seen[id] {
			out = append(out, b)
		}
	}
	return out, nil
}

func (s *InMemorySettingsStore) CreateBreed(b *pack.BreedConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	if b.DefaultVariantID == "" && len(b.Variants) > 0 {
		b.DefaultVariantID = b.Variants[0].ID
	}
	s.breeds[b.ID] = b
	s.breedOrder = append(s.breedOrder, b.ID)
	if _, ok := s.roster[b.ID]; !ok {
		s.roster[b.ID] = pack.RosterEntry{Available: b.Enabled}
	}
	return nil
}

func (s *InMemorySettingsStore) UpdateBreed(id string, b *pack.BreedConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.breeds[id]; !ok {
		return fmt.Errorf("breed %q not found", id)
	}
	b.ID = id
	s.breeds[id] = b
	if _, ok := s.roster[id]; !ok {
		s.roster[id] = pack.RosterEntry{Available: b.Enabled}
	}
	return nil
}

func (s *InMemorySettingsStore) DeleteBreed(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.breeds[id]; !ok {
		return fmt.Errorf("breed %q: %w", id, ErrBreedNotFound)
	}
	delete(s.breeds, id)
	delete(s.roster, id)
	s.deletedBreeds[id] = true
	for i, oid := range s.breedOrder {
		if oid == id {
			s.breedOrder = append(s.breedOrder[:i], s.breedOrder[i+1:]...)
			break
		}
	}
	return nil
}

// ListDeletedBreeds returns the IDs of breeds the customer has explicitly
// deleted (see FileSettingsStore.ListDeletedBreeds for semantics).
func (s *InMemorySettingsStore) ListDeletedBreeds() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.deletedBreeds))
	for id := range s.deletedBreeds {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// ListSeenTemplateBreeds returns the template breed IDs seen by this catalog.
func (s *InMemorySettingsStore) ListSeenTemplateBreeds() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.seenTemplateBreeds))
	for id := range s.seenTemplateBreeds {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// AddSeenTemplateBreeds marks template breed IDs as seen (in-memory).
func (s *InMemorySettingsStore) AddSeenTemplateBreeds(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if id != "" {
			s.seenTemplateBreeds[id] = true
		}
	}
	return nil
}

// CatalogFileExists reports whether a catalog file exists. The in-memory store
// is non-persistent, so it always reports true (treated as an existing catalog;
// the seen set governs add/no-add behavior the same way).
func (s *InMemorySettingsStore) CatalogFileExists() bool { return true }

func (s *InMemorySettingsStore) ReorderBreeds(order []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	newOrder := make([]string, 0, len(s.breeds))
	seen := make(map[string]bool)
	for _, id := range order {
		if _, ok := s.breeds[id]; ok && !seen[id] {
			newOrder = append(newOrder, id)
			seen[id] = true
		}
	}
	for _, id := range s.breedOrder {
		if !seen[id] {
			newOrder = append(newOrder, id)
			seen[id] = true
		}
	}
	s.breedOrder = newOrder
	return nil
}

func (s *InMemorySettingsStore) GetRoster() (map[string]pack.RosterEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]pack.RosterEntry, len(s.roster))
	for k, v := range s.roster {
		out[k] = v
	}
	return out, nil
}

func (s *InMemorySettingsStore) UpdateRosterEntry(id string, e pack.RosterEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.breeds[id]; !ok {
		return fmt.Errorf("breed %q: %w", id, ErrBreedNotFound)
	}
	s.roster[id] = e
	return nil
}

func (s *InMemorySettingsStore) DeleteRosterEntry(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.roster, id)
	return nil
}

func (s *InMemorySettingsStore) GetReviewPolicy() (*pack.ReviewPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.reviewPolicy == nil {
		return &pack.ReviewPolicy{}, nil
	}
	return s.reviewPolicy, nil
}

func (s *InMemorySettingsStore) UpdateReviewPolicy(p *pack.ReviewPolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reviewPolicy = p
	return nil
}

// ---------------------------------------------------------------------------
// Members (deprecated; projected onto breeds for the legacy endpoint)
// ---------------------------------------------------------------------------

func (s *InMemorySettingsStore) ListMembers() ([]*Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Member, 0, len(s.breeds))
	for _, b := range s.breeds {
		out = append(out, breedToMember(b, s.roster[b.ID]))
	}
	return out, nil
}

func (s *InMemorySettingsStore) CreateMember(breedID, displayName, role string, enabled bool) (*Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Breed key (breedID) -> BreedConfig.Name; human label (displayName) ->
	// BreedConfig.DisplayName; legacy role -> roster family assignment.
	b := &pack.BreedConfig{
		ID:               uuid.NewString(),
		Name:             breedID,
		DisplayName:      displayName,
		DefaultVariantID: "default",
		Variants: []pack.Variant{
			{ID: "default", ClientID: breedID, MCPSupport: true},
		},
		Source:  pack.BreedSourceUser,
		Enabled: enabled,
	}
	s.breeds[b.ID] = b
	s.roster[b.ID] = pack.RosterEntry{Family: role, Available: enabled}
	return breedToMember(b, s.roster[b.ID]), nil
}

func (s *InMemorySettingsStore) UpdateMember(id string, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.breeds[id]
	if !ok {
		return fmt.Errorf("member %q not found", id)
	}
	applyBreedUpdates(b, updates)
	if _, ok := s.roster[id]; !ok {
		s.roster[id] = pack.RosterEntry{Available: b.Enabled}
	}
	return nil
}

func (s *InMemorySettingsStore) DeleteMember(id string) error {
	return s.DeleteBreed(id)
}

// ---------------------------------------------------------------------------
// Accounts & Config (unchanged)
// ---------------------------------------------------------------------------

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

// UpsertConfig creates or updates a config key (mirrors FileSettingsStore).
func (s *InMemorySettingsStore) UpsertConfig(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.configs {
		if c.Key == key {
			c.Value = value
			return nil
		}
	}
	s.configs = append(s.configs, &SystemConfig{Key: key, Value: value, Category: "system"})
	return nil
}

func (s *InMemorySettingsStore) GetLeader() (*pack.Leader, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.leader == nil {
		l := pack.DefaultLeaderConfig()
		return &l, nil
	}
	l := *s.leader
	return &l, nil
}

func (s *InMemorySettingsStore) UpdateLeader(l *pack.Leader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leader = l
	return nil
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
