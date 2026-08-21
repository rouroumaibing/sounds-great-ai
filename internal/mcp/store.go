package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// MCPSettingsFileName is the on-disk JSON file (under ConfigRoot) that holds
// operator-managed MCP server entries. This mirrors the existing settings
// persistence layout (accounts.json / dog-catalog.json) so MCP servers live
// alongside the rest of the durable, operator-owned configuration.
const MCPSettingsFileName = "mcp-servers.json"

// maxBackups is the number of timestamped .bak snapshots kept per MCP config
// file (most recent wins), matching the settings store's retention policy.
const maxBackups = 5

// FileStore is a persistent, operator-managed registry of MCP servers. It is
// the single source of truth for user-configured servers and keeps the
// in-memory MCPRegistry (used by BuildMCPConfig) in sync on every mutation.
type FileStore struct {
	path string
	mu   sync.RWMutex
	reg  *MCPRegistry

	// items holds the persisted server configs keyed by Name. Builtin servers
	// (seeded by the platform) also live here so they survive restarts.
	items map[string]*MCPServerConfig
}

// NewFileStore loads (or initializes) the persisted MCP server store and syncs
// the in-memory registry. A missing file is treated as an empty store; the
// platform seeds builtin servers separately via SeedKnowledge.
func NewFileStore(configRoot string, reg *MCPRegistry) *FileStore {
	if reg == nil {
		reg = NewRegistry()
	}
	fs := &FileStore{
		path:  filepath.Join(configRoot, MCPSettingsFileName),
		reg:   reg,
		items: make(map[string]*MCPServerConfig),
	}
	if dir := filepath.Dir(fs.path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	fs.load()
	fs.sync()
	return fs
}

// load reads the JSON file into items. Corrupt or missing files are treated as
// empty (fail-soft, never fatal) so the platform can still boot.
func (s *FileStore) load() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return // absent → empty store
	}
	var list []MCPServerConfig
	if err := json.Unmarshal(raw, &list); err != nil {
		// Corrupt file: log and start empty rather than crashing the platform.
		// (We do not auto-backup on load; backup happens on the next write.)
		fmt.Printf("WARN: MCP store %s corrupt; starting empty: %v\n", s.path, err)
		return
	}
	for i := range list {
		c := list[i]
		if c.Name == "" {
			continue
		}
		cp := c
		s.items[c.Name] = &cp
	}
}

// sync rebuilds the in-memory registry from items. Call after any mutation.
func (s *FileStore) sync() {
	s.reg.Reset()
	for _, cfg := range s.items {
		cp := *cfg
		s.reg.Register(cp.Name, &cp)
	}
}

// save atomically writes items to disk with a timestamped backup.
func (s *FileStore) save() error {
	list := make([]MCPServerConfig, 0, len(s.items))
	for _, cfg := range s.items {
		list = append(list, *cfg)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	// Backup existing file before overwriting (edit-time only).
	if _, err := os.Stat(s.path); err == nil {
		if prev, rerr := os.ReadFile(s.path); rerr == nil {
			ts := time.Now().Format("20060102-150405")
			_ = os.WriteFile(fmt.Sprintf("%s.bak-%s", s.path, ts), prev, 0o600)
			pruneBackups(s.path, maxBackups)
		}
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

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

// List returns a sorted, deep-copied snapshot of all servers (including
// disabled ones), suitable for API responses.
func (s *FileStore) List() []MCPServerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MCPServerConfig, 0, len(s.items))
	for _, cfg := range s.items {
		out = append(out, *cfg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns a deep copy of the named server, if present.
func (s *FileStore) Get(name string) (MCPServerConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.items[name]
	if !ok {
		return MCPServerConfig{}, false
	}
	return *cfg, true
}

// Add inserts a new server. Returns an error if the name is empty, neither a
// command (stdio) nor a URL (remote) is provided, both are provided, or a
// server with that name already exists.
func (s *FileStore) Add(cfg MCPServerConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("mcp server name is required")
	}
	if cfg.Command == "" && cfg.URL == "" {
		return fmt.Errorf("mcp server requires either command (stdio) or url (remote)")
	}
	if cfg.Command != "" && cfg.URL != "" {
		return fmt.Errorf("mcp server must specify command OR url, not both")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[cfg.Name]; exists {
		return fmt.Errorf("mcp server %q already exists", cfg.Name)
	}
	cp := cfg
	cp.Builtin = false
	s.items[cp.Name] = &cp
	if err := s.save(); err != nil {
		return err
	}
	s.sync()
	return nil
}

// Update patches mutable fields of an existing server. The name is the lookup
// key and cannot be changed here (use Remove + Add for a rename). Builtin
// servers are read-only except for their Enabled flag.
func (s *FileStore) Update(name string, patch MCPServerConfig) (*MCPServerConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.items[name]
	if !ok {
		return nil, fmt.Errorf("mcp server %q not found", name)
	}
	if cur.Builtin && (patch.Command != "" || patch.Args != nil || patch.Env != nil || patch.Breeds != nil || patch.DisplayName != "") {
		return nil, fmt.Errorf("mcp server %q is builtin and read-only", name)
	}
	if patch.Command != "" {
		cur.Command = patch.Command
	}
	if patch.Args != nil {
		cur.Args = patch.Args
	}
	if patch.Env != nil {
		// Merge provided env keys over the existing map (PATCH semantics) so a
		// client that only knows masked "***" values can omit them without
		// wiping the real secret. An empty value deletes the key.
		if cur.Env == nil {
			cur.Env = make(map[string]string)
		}
		for k, v := range patch.Env {
			if v == "" {
				delete(cur.Env, k)
			} else {
				cur.Env[k] = v
			}
		}
	}
	if patch.URL != "" {
		cur.URL = patch.URL
	}
	if patch.Headers != nil {
		// PATCH semantics: empty value deletes the header (so a masked "***"
		// value can be omitted without clobbering the real secret).
		if cur.Headers == nil {
			cur.Headers = make(map[string]string)
		}
		for k, v := range patch.Headers {
			if v == "" {
				delete(cur.Headers, k)
			} else {
				cur.Headers[k] = v
			}
		}
	}
	if patch.CallbackURL != "" {
		cur.CallbackURL = patch.CallbackURL
	}
	if patch.Breeds != nil {
		cur.Breeds = patch.Breeds
	}
	if patch.DisplayName != "" {
		cur.DisplayName = patch.DisplayName
	}
	cur.Enabled = patch.Enabled
	if err := s.save(); err != nil {
		return nil, err
	}
	s.sync()
	cp := *cur
	return &cp, nil
}

// SetEnabled toggles a server's Enabled flag.
func (s *FileStore) SetEnabled(name string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.items[name]
	if !ok {
		return fmt.Errorf("mcp server %q not found", name)
	}
	cur.Enabled = enabled
	if err := s.save(); err != nil {
		return err
	}
	s.sync()
	return nil
}

// Remove deletes a user-managed server. Builtin servers cannot be removed
// (they are re-seeded by the platform); return an error instead.
func (s *FileStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.items[name]
	if !ok {
		return fmt.Errorf("mcp server %q not found", name)
	}
	if cur.Builtin {
		return fmt.Errorf("mcp server %q is builtin and cannot be removed", name)
	}
	delete(s.items, name)
	if err := s.save(); err != nil {
		return err
	}
	s.sync()
	return nil
}

// seedBuiltin ensures a builtin (platform-seeded) server exists, updating its
// transport configuration in place when already present. A pre-existing entry
// keeps its current Enabled flag so an operator who disabled the builtin stays
// disabled across restarts; only brand-new entries default to enabled. Persist
// only when the store file did not already exist, to avoid clobbering
// user-managed state on subsequent boots.
func (s *FileStore) seedBuiltin(name, displayName, command string, args []string, env, headers map[string]string, callbackURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.items[name]
	if !ok {
		cur = &MCPServerConfig{Name: name, Builtin: true, Enabled: true}
		s.items[name] = cur
	}
	cur.DisplayName = displayName
	cur.Command = command
	cur.Args = args
	cur.Env = env
	cur.Headers = headers
	cur.CallbackURL = callbackURL
	cur.Builtin = true
	if !ok {
		cur.Enabled = true
	}
	if _, err := os.Stat(s.path); err != nil {
		_ = s.save()
	}
	s.sync()
}

// SeedKnowledge ensures the builtin RAG "knowledge" server exists, updating its
// command/args in place when already present. Called by the server only when
// RAG is initialized (mirrors the prior hardcoded registration gate).
func (s *FileStore) SeedKnowledge(command string, args []string) {
	s.seedBuiltin("knowledge", "Knowledge (RAG)", command, args, nil, nil, "")
}

// SeedPlatform ensures the builtin "platform" MCP server exists. This is the
// platform-as-MCP-server surface: an MCP server (stdio by default, optionally
// Streamable HTTP via the same binary's --transport flag) that proxies the SG
// REST API and exposes collab/memory/people/roster/breeds capabilities to CLI
// agents. The apiBase/token env and CallbackURL mirror the SG server's own
// listen address and auth token at startup (read-only configuration, not
// runtime mutation). callbackURL is the HTTP fallback the agent uses when the
// MCP transport is unavailable — it points at the SG REST API itself.
func (s *FileStore) SeedPlatform(command string, args []string, env, headers map[string]string, callbackURL string) {
	s.seedBuiltin("platform", "Platform (MCP Tools)", command, args, env, headers, callbackURL)
}
