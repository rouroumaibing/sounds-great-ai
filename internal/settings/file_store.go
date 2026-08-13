package settings

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"sounds-great-ai/pkg/pack"
)

// File names for the persisted settings (separates
// account metadata, secrets, and the member/leader catalog).
const (
	AccountsFileName    = "accounts.json"
	CatalogFileName     = "dog-catalog.json"
	RepoTrajectoryFileName = "repo-trajectory.json"
	CredentialsFileName = "credentials.json"

	// maxBackups is the number of timestamped .bak files kept per config file
	// (most recent wins); older snapshots are pruned to avoid unbounded growth.
	maxBackups = 5
)

// EnvConfigRoot overrides the root directory for persisted settings. When set,
// all config files live
// under this directory instead of the default project-root location.
const EnvConfigRoot = "SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT"

// ConfigRoot returns the directory where settings files are stored, resolved
// in this order:
//  1. SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT (env, "~" expanded per-OS)
//  2. {projectRoot}/.sounds-great-ai     (when projectRoot is non-empty)
//  3. {home}/.sounds-great-ai            (fallback when no projectRoot)
//
// The "~" prefix is expanded using os.UserHomeDir, which is correct on macOS,
// Linux, and Windows (where it maps to the user profile directory).
func ConfigRoot(projectRoot string) string {
	if d := os.Getenv(EnvConfigRoot); d != "" {
		return expandHome(d)
	}
	if projectRoot != "" {
		return filepath.Join(projectRoot, ".sounds-great-ai")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".sounds-great-ai")
	}
	return ".sounds-great-ai"
}

// expandHome replaces a leading "~" (or "~/") with the user's home directory.
func expandHome(p string) string {
	if p == "~" || (len(p) >= 2 && p[:2] == "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// EnvCredentialRoot overrides the directory for secret credentials only.
// When set, credentials.json lives under this directory instead of the
// default global home location.
const EnvCredentialRoot = "SOUNDS_GREAT_AI_CREDENTIAL_ROOT"

// CredentialRoot returns the directory where credentials.json is stored,
// resolved in this order:
//  1. SOUNDS_GREAT_AI_CREDENTIAL_ROOT (env, "~" expanded per-OS)
//  2. {home}/.sounds-great-ai            (global home, the customer-safe default)
//
// Per the customer-safety layout (aligned with clowder-ai), secrets live in a
// GLOBAL home directory independent of the project-local config root, so that
// clearing the project config never wipes API keys.
func CredentialRoot() string {
	if d := os.Getenv(EnvCredentialRoot); d != "" {
		return expandHome(d)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".sounds-great-ai")
	}
	return ".sounds-great-ai"
}

// accountsDocument is the on-disk envelope for account metadata.
type accountsDocument struct {
	Accounts []*Account `json:"accounts"`
}

// catalogDocument is the on-disk envelope for the runtime member catalog.
// It is clowder-homologous: breeds + roster + review_policy + leader + configs.
// The legacy `members` array is migrated to `breeds`+`roster` on first load.
type catalogDocument struct {
	Version      int                          `json:"version"`
	Breeds       []pack.BreedConfig          `json:"breeds"`
	Roster       map[string]pack.RosterEntry `json:"roster,omitempty"`
	ReviewPolicy *pack.ReviewPolicy          `json:"review_policy,omitempty"`
	Leader       *pack.Leader                `json:"leader,omitempty"`
	Configs      []*SystemConfig             `json:"configs,omitempty"`
	// DeletedBreeds records breed IDs the customer explicitly removed, so the
	// upgrade sync never resurrects a deleted template dog (decision D2).
	DeletedBreeds []string `json:"deleted_breeds,omitempty"`
	// SeenTemplateBreeds records template breed IDs the catalog has already been
	// "exposed to". Persisting it unifies D1 (empty first run) and D3 (auto-add
	// new template breeds on upgrade) — a breed in this set is never re-added
	// after a later deletion. See docs/DESIGN-STORYS/SG-MEM-002.
	SeenTemplateBreeds []string `json:"seen_template_breeds,omitempty"`
}

// FileSettingsStore implements SettingsStore with JSON files on disk:
//   - accounts.json    : account metadata (0644, atomic write)
//   - dog-catalog.json : breeds + roster + review_policy + leader + configs (0644, atomic write)
//   - credentials.json : secrets (handled by FileCredentialStore, 0600)
//
// When watch is true, a HotReloader polls the two files and reloads the
// in-memory cache ~30s after an external modification (config hot-load).
type FileSettingsStore struct {
	accountsPath  string
	catalogPath   string
	mu            sync.RWMutex
	breeds        map[string]*pack.BreedConfig
	breedOrder    []string
	roster        map[string]pack.RosterEntry
	reviewPolicy  *pack.ReviewPolicy
	accounts      map[string]*Account
	configs       []*SystemConfig
	leader        *pack.Leader
	deletedBreeds map[string]bool
	// seenTemplateBreeds records template breed IDs already exposed to this
	// catalog (see SeenTemplateBreeds in catalogDocument).
	seenTemplateBreeds map[string]bool
	loaded              bool
	reload              *HotReloader
}

// NewFileSettingsStore creates a file-backed settings store. accountsPath and
// catalogPath are the two JSON files; watch enables hot-reload on change.
func NewFileSettingsStore(accountsPath, catalogPath string, watch bool) *FileSettingsStore {
	if dir := filepath.Dir(accountsPath); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	if dir := filepath.Dir(catalogPath); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	s := &FileSettingsStore{
		accountsPath:  accountsPath,
		catalogPath:   catalogPath,
		breeds:        make(map[string]*pack.BreedConfig),
		roster:        make(map[string]pack.RosterEntry),
		accounts:      make(map[string]*Account),
		configs:       defaultConfig(),
		deletedBreeds: make(map[string]bool),
		seenTemplateBreeds: make(map[string]bool),
	}
	if watch {
		s.reload = NewHotReloader([]string{accountsPath, catalogPath}, func() {
			s.mu.Lock()
			_ = s.reloadFromDisk()
			s.mu.Unlock()
		})
		s.reload.Start()
	}
	return s
}

// ensureLoaded loads both files on first access. Callers must hold s.mu.
func (s *FileSettingsStore) ensureLoaded() error {
	if s.loaded {
		return nil
	}
	return s.reloadFromDisk()
}

// reloadFromDisk re-reads both files from disk, replacing in-memory state.
// Callers must hold s.mu. Safe to call repeatedly (used by the hot-reload
// watcher). Legacy `members` documents are migrated to the new
// breeds+roster structure and persisted (a pre-migration backup is written
// first; migration failure is fail-fast per plan D6).
func (s *FileSettingsStore) reloadFromDisk() error {
	s.loaded = true

	// accounts.json
	s.accounts = make(map[string]*Account)
	if raw, err := os.ReadFile(s.accountsPath); err == nil {
		var doc accountsDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			log.Printf("WARN: accounts file %s is corrupt; treating as empty (no backup written at load)", s.accountsPath)
			s.accounts = make(map[string]*Account)
		} else {
			for _, a := range doc.Accounts {
				if a != nil && a.ID != "" {
					s.accounts[a.ID] = a
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	// dog-catalog.json
	s.breeds = make(map[string]*pack.BreedConfig)
	s.breedOrder = nil
	s.roster = make(map[string]pack.RosterEntry)
	s.reviewPolicy = nil
	s.leader = nil
	s.configs = defaultConfig()
	if raw, err := os.ReadFile(s.catalogPath); err == nil {
		// Detect the legacy `members` format so we can migrate it.
		var probe struct {
			Members json.RawMessage `json:"members"`
		}
		_ = json.Unmarshal(raw, &probe)
		isLegacy := len(probe.Members) > 0

		if isLegacy {
			if err := s.migrateLegacyMembers(raw); err != nil {
				return err
			}
			return nil
		}

		var doc catalogDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			log.Printf("WARN: catalog file %s is corrupt; treating as empty (no backup written at load)", s.catalogPath)
			s.breeds = make(map[string]*pack.BreedConfig)
			s.breedOrder = nil
			s.roster = make(map[string]pack.RosterEntry)
			s.reviewPolicy = nil
			s.leader = nil
			s.configs = defaultConfig()
		} else {
			for i := range doc.Breeds {
				b := &doc.Breeds[i]
				if b.ID != "" {
					s.breeds[b.ID] = b
					s.breedOrder = append(s.breedOrder, b.ID)
				}
			}
			for k, r := range doc.Roster {
				s.roster[k] = r
			}
			s.reviewPolicy = doc.ReviewPolicy
			if doc.Leader != nil {
				s.leader = doc.Leader
			}
			if len(doc.Configs) > 0 {
				s.configs = doc.Configs
			}
			for _, id := range doc.DeletedBreeds {
				if id != "" {
					s.deletedBreeds[id] = true
				}
			}
			for _, id := range doc.SeenTemplateBreeds {
				if id != "" {
					s.seenTemplateBreeds[id] = true
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

// migrateLegacyMembers converts a legacy `members`-based catalog into the new
// breeds+roster structure and persists it. A backup is written to
// <catalogPath>.pre-migration before any write.
func (s *FileSettingsStore) migrateLegacyMembers(raw []byte) error {
	var legacy struct {
		Members []*Member       `json:"members"`
		Leader  *pack.Leader    `json:"leader,omitempty"`
		Configs []*SystemConfig `json:"configs,omitempty"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		log.Printf("WARN: legacy catalog file %s is corrupt; cannot migrate", s.catalogPath)
		return fmt.Errorf("parse legacy catalog: %w", err)
	}
	// Backup before mutating.
	if err := os.WriteFile(s.catalogPath+".pre-migration", raw, 0o644); err != nil {
		return fmt.Errorf("backup pre-migration: %w", err)
	}
	for _, m := range legacy.Members {
		if m == nil || m.ID == "" {
			continue
		}
		b, r := memberToBreedAndRoster(m)
		s.breeds[b.ID] = b
		s.breedOrder = append(s.breedOrder, b.ID)
		s.roster[b.ID] = r
	}
	if legacy.Leader != nil {
		s.leader = legacy.Leader
	}
	if len(legacy.Configs) > 0 {
		s.configs = legacy.Configs
	}
	if err := s.flushCatalog(); err != nil {
		return fmt.Errorf("persist migrated catalog: %w", err)
	}
	return nil
}

// backupBeforeWrite snapshots the existing file to a timestamped .bak before
// an edit write, so a customer can recover the previous version. Per product
// decision, backups are created ONLY on edit (not when loading a corrupt file,
// which is now treated as empty with a warning).
func backupBeforeWrite(path string) {
	if _, err := os.Stat(path); err != nil {
		return // no existing file yet; nothing to snapshot
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	ts := time.Now().Format("20060102-150405")
	bak := fmt.Sprintf("%s.bak-%s", path, ts)
	if err := os.WriteFile(bak, data, 0o644); err != nil {
		return
	}
	pruneBackups(path, maxBackups)
}

// pruneBackups keeps at most keep timestamped .bak files (most recent wins),
// removing older snapshots to avoid unbounded growth.
func pruneBackups(path string, keep int) {
	pattern := filepath.Join(filepath.Dir(path), filepath.Base(path)+".bak-*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) <= keep {
		return
	}
	sort.Strings(matches)
	for _, old := range matches[:len(matches)-keep] {
		_ = os.Remove(old)
	}
}

// sortedKeys returns the sorted keys of a string→bool set (used to persist
// deleted_breeds deterministically).
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *FileSettingsStore) flushAccounts() error {
	backupBeforeWrite(s.accountsPath)
	doc := accountsDocument{Accounts: make([]*Account, 0, len(s.accounts))}
	for _, a := range s.accounts {
		doc.Accounts = append(doc.Accounts, a)
	}
	return writeAtomic(s.accountsPath, doc, 0o644)
}

func (s *FileSettingsStore) flushCatalog() error {
	backupBeforeWrite(s.catalogPath)
	leader := s.leader
	if leader == nil {
		l := pack.DefaultLeaderConfig()
		leader = &l
	}
	doc := catalogDocument{
		Version:      2,
		Breeds:       make([]pack.BreedConfig, 0, len(s.breeds)),
		Roster:       s.roster,
		ReviewPolicy: s.reviewPolicy,
		Leader:       leader,
		Configs:      s.configs,
		DeletedBreeds:     sortedKeys(s.deletedBreeds),
		SeenTemplateBreeds: sortedKeys(s.seenTemplateBreeds),
	}
	seen := make(map[string]bool, len(s.breedOrder))
	for _, id := range s.breedOrder {
		if b, ok := s.breeds[id]; ok {
			doc.Breeds = append(doc.Breeds, *b)
			seen[id] = true
		}
	}
	// Defensive: append any breeds not tracked in breedOrder.
	for id, b := range s.breeds {
		if !seen[id] {
			doc.Breeds = append(doc.Breeds, *b)
		}
	}
	return writeAtomic(s.catalogPath, doc, 0o644)
}

// writeAtomic marshals v as indented JSON and writes it atomically
// (temp file + rename) with the given permission bits.
func writeAtomic(path string, v any, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---------------------------------------------------------------------------
// Breeds (runtime member catalog)
// ---------------------------------------------------------------------------

func (s *FileSettingsStore) ListBreeds() ([]*pack.BreedConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
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

func (s *FileSettingsStore) CreateBreed(b *pack.BreedConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
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
	return s.flushCatalog()
}

func (s *FileSettingsStore) UpdateBreed(id string, b *pack.BreedConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	if _, ok := s.breeds[id]; !ok {
		return fmt.Errorf("breed %q not found", id)
	}
	b.ID = id
	s.breeds[id] = b
	if _, ok := s.roster[id]; !ok {
		s.roster[id] = pack.RosterEntry{Available: b.Enabled}
	}
	return s.flushCatalog()
}

func (s *FileSettingsStore) DeleteBreed(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
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
	return s.flushCatalog()
}

// ListDeletedBreeds returns the IDs of breeds the customer has explicitly
// deleted. Used by the upgrade sync to avoid resurrecting removed template dogs
// (decision D2).
func (s *FileSettingsStore) ListDeletedBreeds() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	return sortedKeys(s.deletedBreeds), nil
}

// ListSeenTemplateBreeds returns the template breed IDs this catalog has already
// been exposed to (never re-added). See SG-MEM-002 §4.1.
func (s *FileSettingsStore) ListSeenTemplateBreeds() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	return sortedKeys(s.seenTemplateBreeds), nil
}

// AddSeenTemplateBreeds marks the given template breed IDs as seen and persists
// them to the catalog file. Idempotent: IDs already seen are skipped.
func (s *FileSettingsStore) AddSeenTemplateBreeds(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	changed := false
	for _, id := range ids {
		if id != "" && !s.seenTemplateBreeds[id] {
			s.seenTemplateBreeds[id] = true
			changed = true
		}
	}
	if changed {
		return s.flushCatalog()
	}
	return nil
}

// CatalogFileExists reports whether the runtime catalog file exists on disk.
// It distinguishes a first run (absent → write empty catalog + seen) from an
// existing customer catalog.
func (s *FileSettingsStore) CatalogFileExists() bool {
	_, err := os.Stat(s.catalogPath)
	return err == nil
}

// ReorderBreeds reorders the persisted catalog breeds[] to match order.
func (s *FileSettingsStore) ReorderBreeds(order []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	newOrder := make([]string, 0, len(s.breeds))
	seen := make(map[string]bool)
	for _, id := range order {
		if _, ok := s.breeds[id]; ok && !seen[id] {
			newOrder = append(newOrder, id)
			seen[id] = true
		}
	}
	// Append catalog breeds not mentioned in order, preserving prior relative order.
	for _, id := range s.breedOrder {
		if !seen[id] {
			newOrder = append(newOrder, id)
			seen[id] = true
		}
	}
	s.breedOrder = newOrder
	return s.flushCatalog()
}

func (s *FileSettingsStore) GetRoster() (map[string]pack.RosterEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	out := make(map[string]pack.RosterEntry, len(s.roster))
	for k, v := range s.roster {
		out[k] = v
	}
	return out, nil
}

func (s *FileSettingsStore) UpdateRosterEntry(id string, e pack.RosterEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	if _, ok := s.breeds[id]; !ok {
		return fmt.Errorf("breed %q: %w", id, ErrBreedNotFound)
	}
	s.roster[id] = e
	return s.flushCatalog()
}

func (s *FileSettingsStore) DeleteRosterEntry(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	delete(s.roster, id)
	return s.flushCatalog()
}

func (s *FileSettingsStore) GetReviewPolicy() (*pack.ReviewPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	if s.reviewPolicy == nil {
		return &pack.ReviewPolicy{}, nil
	}
	return s.reviewPolicy, nil
}

func (s *FileSettingsStore) UpdateReviewPolicy(p *pack.ReviewPolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	s.reviewPolicy = p
	return s.flushCatalog()
}

// ---------------------------------------------------------------------------
// Members (deprecated; projected onto breeds for the legacy endpoint)
// ---------------------------------------------------------------------------

func (s *FileSettingsStore) ListMembers() ([]*Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	out := make([]*Member, 0, len(s.breeds))
	for _, b := range s.breeds {
		out = append(out, breedToMember(b, s.roster[b.ID]))
	}
	return out, nil
}

func (s *FileSettingsStore) CreateMember(breedID, displayName, role string, enabled bool) (*Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	// The breed key (breedID) maps to BreedConfig.Name; the human label
	// (displayName) maps to BreedConfig.DisplayName; the legacy role is kept
	// as the roster family assignment.
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
	if err := s.flushCatalog(); err != nil {
		return nil, err
	}
	return breedToMember(b, s.roster[b.ID]), nil
}

func (s *FileSettingsStore) UpdateMember(id string, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	b, ok := s.breeds[id]
	if !ok {
		return fmt.Errorf("member %q not found", id)
	}
	applyBreedUpdates(b, updates)
	if _, ok := s.roster[id]; !ok {
		s.roster[id] = pack.RosterEntry{Available: b.Enabled}
	}
	return s.flushCatalog()
}

func (s *FileSettingsStore) DeleteMember(id string) error {
	return s.DeleteBreed(id)
}

// ---------------------------------------------------------------------------
// Accounts & Config (unchanged)
// ---------------------------------------------------------------------------

func (s *FileSettingsStore) ListAccounts() ([]*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	out := make([]*Account, 0, len(s.accounts))
	for _, a := range s.accounts {
		out = append(out, a)
	}
	return out, nil
}

func (s *FileSettingsStore) CreateAccount(provider, apiKey string) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	a := &Account{
		ID:         uuid.NewString(),
		Provider:   provider,
		KeyPreview: maskKey(apiKey),
		KeySet:     apiKey != "",
		UpdatedAt:  time.Now().UnixMilli(),
	}
	s.accounts[a.ID] = a
	if err := s.flushAccounts(); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *FileSettingsStore) UpdateAccount(id string, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	a, ok := s.accounts[id]
	if !ok {
		return fmt.Errorf("account %q not found", id)
	}
	if v, ok := updates["provider"]; ok {
		if str, ok := v.(string); ok {
			a.Provider = str
		}
	}
	if v, ok := updates["name"]; ok {
		if str, ok := v.(string); ok {
			a.Name = str
		}
	}
	if v, ok := updates["client_id"]; ok {
		if str, ok := v.(string); ok {
			a.ClientID = str
		}
	}
	if v, ok := updates["display_name"]; ok {
		if str, ok := v.(string); ok {
			a.DisplayName = str
		}
	}
	if v, ok := updates["base_url"]; ok {
		if str, ok := v.(string); ok {
			a.BaseURL = str
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
			// Drop keys that would clobber system-reserved environment
			// variables (e.g. SOUNDS_GREAT_AI_* used by the runtime).
			filtered := make(map[string]string, len(mm))
			for k, val := range mm {
				if strings.HasPrefix(k, "SOUNDS_GREAT_AI_") {
					continue
				}
				filtered[k] = val
			}
			a.EnvVars = filtered
		}
	}
	if v, ok := updates["auth_type"]; ok {
		if str, ok := v.(string); ok {
			a.AuthType = str
		}
	}
	if v, ok := updates["mode"]; ok {
		if str, ok := v.(string); ok {
			a.Mode = str
		}
	}
	if v, ok := updates["builtin"]; ok {
		if b, ok := v.(bool); ok {
			a.Builtin = b
		}
	}
	a.UpdatedAt = time.Now().UnixMilli()
	return s.flushAccounts()
}

func (s *FileSettingsStore) DeleteAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	if _, ok := s.accounts[id]; !ok {
		return fmt.Errorf("account %q not found", id)
	}
	delete(s.accounts, id)
	return s.flushAccounts()
}

func (s *FileSettingsStore) ListConfig() ([]*SystemConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	out := make([]*SystemConfig, len(s.configs))
	copy(out, s.configs)
	return out, nil
}

func (s *FileSettingsStore) UpdateConfig(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	for _, c := range s.configs {
		if c.Key == key {
			c.Value = value
			return s.flushCatalog()
		}
	}
	return fmt.Errorf("config key %q not found", key)
}

// UpsertConfig creates or updates a config key. Used for new keys (e.g.
// repo_url) that are not guaranteed to exist in a pre-upgrade catalog.
func (s *FileSettingsStore) UpsertConfig(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	for _, c := range s.configs {
		if c.Key == key {
			c.Value = value
			return s.flushCatalog()
		}
	}
	s.configs = append(s.configs, &SystemConfig{Key: key, Value: value, Category: "system"})
	return s.flushCatalog()
}

// GetLeader returns the persisted leader config, or the default if none.
func (s *FileSettingsStore) GetLeader() (*pack.Leader, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	if s.leader == nil {
		l := pack.DefaultLeaderConfig()
		return &l, nil
	}
	l := *s.leader
	return &l, nil
}

// UpdateLeader persists the leader config into the catalog file.
func (s *FileSettingsStore) UpdateLeader(l *pack.Leader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	s.leader = l
	return s.flushCatalog()
}

// ---------------------------------------------------------------------------
// Projection helpers (member <-> breed)
// ---------------------------------------------------------------------------

// memberToBreedAndRoster converts a legacy Member into a breed variant plus a
// roster entry, following the migration mapping table in the plan.
func memberToBreedAndRoster(m *Member) (*pack.BreedConfig, pack.RosterEntry) {
	var color *pack.Color
	if m.ColorPrimary != "" || m.ColorSecondary != "" {
		color = &pack.Color{Primary: m.ColorPrimary, Secondary: m.ColorSecondary}
	}
	v := pack.Variant{
		ID:           "default",
		ClientID:     m.ClientID,
		DefaultModel: m.DefaultModel,
		Provider:     m.Provider,
		AccountRef:   m.AccountRef,
		MCPSupport:   m.MCPSupport,
		CLI: pack.CLIConfig{
			Command:      m.CLICommand,
			OutputFormat: m.OutputFormat,
			DefaultArgs:  splitArgs(m.DefaultArgs),
			Effort:       m.Effort,
		},
		ContextBudget: pack.ContextBudget{
			MaxPromptTokens:  m.MaxPromptTokens,
			MaxContextTokens: m.MaxContextTokens,
			MaxMessages:      m.MaxMessages,
		},
		SessionChain: boolToStr(m.SessionChain),
		Strategy:     m.Strategy,
	}
	b := &pack.BreedConfig{
		ID:              m.ID,
		Name:            m.BreedID,
		DisplayName:     m.DisplayName,
		Nickname:        m.Nickname,
		Avatar:          m.Avatar,
		Color:           color,
		Personality:     m.Personality,
		RoleDescription: m.RoleDescription,
		TeamStrengths:   strings.Join(m.TeamStrengths, "\n"),
		MentionPatterns: m.MentionPatterns,
		Caution:         m.Caution,
		DefaultVariantID: "default",
		Variants:        []pack.Variant{v},
		Source:          pack.BreedSourceUser,
		Enabled:         m.Enabled,
	}
	r := pack.RosterEntry{
		Family:    m.Role,
		Roles:     []string{},
		Lead:      false,
		Available: m.Enabled,
	}
	return b, r
}

// breedToMember projects a breed + roster entry back into the legacy Member
// shape, preserving the API contract of /api/settings/members.
func breedToMember(b *pack.BreedConfig, r pack.RosterEntry) *Member {
	v := b.DefaultVariant()
	if v == nil && len(b.Variants) > 0 {
		v = &b.Variants[0]
	}
	m := &Member{
		ID:          b.ID,
		BreedID:     b.Name,
		DisplayName: b.DisplayName,
		Role:        r.Family,
		Enabled:     r.Available,
		ClientID:    v.ClientID,
		AccountRef:  v.AccountRef,
		DefaultModel: v.DefaultModel,
		Provider:    v.Provider,
		Nickname:    b.Nickname,
		Avatar:      b.Avatar,
		MentionPatterns: b.MentionPatterns,
		Personality:     b.Personality,
		RoleDescription: b.RoleDescription,
		Caution:         b.Caution,
		CLICommand:      v.CLI.Command,
		OutputFormat:    v.CLI.OutputFormat,
		DefaultArgs:     strings.Join(v.CLI.DefaultArgs, " "),
		Effort:          v.CLI.Effort,
		MaxPromptTokens:  v.ContextBudget.MaxPromptTokens,
		MaxContextTokens: v.ContextBudget.MaxContextTokens,
		MaxMessages:      v.ContextBudget.MaxMessages,
		MCPSupport:       v.MCPSupport,
		SessionChain:     v.SessionChain == "true",
		Strategy:         v.Strategy,
	}
	if b.Color != nil {
		m.ColorPrimary = b.Color.Primary
		m.ColorSecondary = b.Color.Secondary
	}
	if ts := strings.TrimSpace(b.TeamStrengths); ts != "" {
		m.TeamStrengths = strings.Split(ts, "\n")
	}
	return m
}

// applyBreedUpdates mutates a breed (and its default variant) in place from a
// loose-typed updates map, mirroring applyMemberUpdates for the new structure.
func applyBreedUpdates(b *pack.BreedConfig, updates map[string]any) {
	v := b.DefaultVariant()
	if v == nil {
		b.Variants = []pack.Variant{{ID: "default"}}
		v = &b.Variants[0]
		b.DefaultVariantID = "default"
	}
	if s, ok := updates["display_name"].(string); ok {
		b.DisplayName = s
	}
	if s, ok := updates["role"].(string); ok {
		b.DisplayName = s
	}
	if s, ok := updates["nickname"].(string); ok {
		b.Nickname = s
	}
	if s, ok := updates["avatar"].(string); ok {
		b.Avatar = s
	}
	if s, ok := updates["color_primary"].(string); ok {
		if b.Color == nil {
			b.Color = &pack.Color{}
		}
		b.Color.Primary = s
	}
	if s, ok := updates["color_secondary"].(string); ok {
		if b.Color == nil {
			b.Color = &pack.Color{}
		}
		b.Color.Secondary = s
	}
	if ss, ok := updates["mention_patterns"].([]string); ok {
		b.MentionPatterns = ss
	}
	if s, ok := updates["personality"].(string); ok {
		b.Personality = s
	}
	if s, ok := updates["role_description"].(string); ok {
		b.RoleDescription = s
	}
	if s, ok := updates["team_strengths"].(string); ok {
		b.TeamStrengths = s
	}
	if ss, ok := updates["team_strengths"].([]string); ok {
		b.TeamStrengths = strings.Join(ss, "\n")
	}
	if s, ok := updates["caution"].(string); ok {
		b.Caution = s
	}
	if s, ok := updates["client_id"].(string); ok {
		v.ClientID = s
	}
	if s, ok := updates["account_ref"].(string); ok {
		v.AccountRef = s
	}
	if s, ok := updates["default_model"].(string); ok {
		v.DefaultModel = s
	}
	if s, ok := updates["provider"].(string); ok {
		v.Provider = s
	}
	if s, ok := updates["cli_command"].(string); ok {
		v.CLI.Command = s
	}
	if s, ok := updates["output_format"].(string); ok {
		v.CLI.OutputFormat = s
	}
	if s, ok := updates["default_args"].(string); ok {
		v.CLI.DefaultArgs = splitArgs(s)
	}
	if ss, ok := updates["default_args"].([]string); ok {
		v.CLI.DefaultArgs = ss
	}
	if s, ok := updates["effort"].(string); ok {
		v.CLI.Effort = s
	}
	if n, ok := toInt(updates["max_prompt_tokens"]); ok {
		v.ContextBudget.MaxPromptTokens = n
	}
	if n, ok := toInt(updates["max_context_tokens"]); ok {
		v.ContextBudget.MaxContextTokens = n
	}
	if n, ok := toInt(updates["max_messages"]); ok {
		v.ContextBudget.MaxMessages = n
	}
	if b2, ok := updates["mcp_support"].(bool); ok {
		v.MCPSupport = b2
	}
	if s, ok := updates["session_chain"].(string); ok {
		v.SessionChain = s
	}
	if b2, ok := updates["session_chain"].(bool); ok {
		v.SessionChain = boolToStr(b2)
	}
	if s, ok := updates["strategy"].(string); ok {
		v.Strategy = s
	}
	if b2, ok := updates["enabled"].(bool); ok {
		b.Enabled = b2
	}
}

func splitArgs(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Fields(s)
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return ""
}
