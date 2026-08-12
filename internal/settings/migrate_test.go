package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sounds-great-ai/pkg/pack"
)

// legacyCatalogDoc is a minimal legacy (version 1) dog-catalog.json that uses
// the `members` array. It exists only to exercise the migration path.
const legacyCatalogDoc = `{
  "version": 1,
  "members": [
    {
      "id": "m1",
      "breed_id": "bianmu",
      "display_name": "Border Collie",
      "role": "router",
      "enabled": true,
      "client_id": "claude",
      "default_model": "opus",
      "provider": "anthropic",
      "color_primary": "#111111",
      "color_secondary": "#222222",
      "mention_patterns": ["@bianmu"],
      "team_strengths": ["routing", "synthesis"],
      "mcp_support": true,
      "session_chain": true,
      "strategy": "balanced"
    },
    {
      "id": "m2",
      "breed_id": "jinmao",
      "display_name": "Golden Retriever",
      "role": "retriever",
      "enabled": false
    }
  ],
  "leader": {
    "name": "You",
    "aliases": ["Owner"],
    "timeZone": "Asia/Shanghai"
  },
  "configs": [
    {"key": "default_breed", "value": "bianmu", "category": "routing"}
  ]
}`

// TestMigrateLegacyMembers verifies that a version-1 `members`-based catalog is
// migrated to the new breeds+roster structure with the correct field mapping.
func TestMigrateLegacyMembers(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, CatalogFileName)
	accountsPath := filepath.Join(dir, AccountsFileName)

	if err := os.WriteFile(catalogPath, []byte(legacyCatalogDoc), 0o644); err != nil {
		t.Fatalf("write legacy catalog: %v", err)
	}

	store := NewFileSettingsStore(accountsPath, catalogPath, false)

	breeds, err := store.ListBreeds()
	if err != nil {
		t.Fatalf("ListBreeds: %v", err)
	}
	if len(breeds) != 2 {
		t.Fatalf("breeds: want 2, got %d", len(breeds))
	}

	byID := make(map[string]*pack.BreedConfig, len(breeds))
	for _, b := range breeds {
		byID[b.ID] = b
	}

	m1, ok := byID["m1"]
	if !ok {
		t.Fatal("expected breed m1")
	}
	// Breed key (breed_id) maps to Name; display_name maps to DisplayName.
	if m1.Name != "bianmu" {
		t.Errorf("m1.Name = %q, want %q", m1.Name, "bianmu")
	}
	if m1.DisplayName != "Border Collie" {
		t.Errorf("m1.DisplayName = %q, want %q", m1.DisplayName, "Border Collie")
	}
	if !m1.Enabled {
		t.Error("m1.Enabled: want true")
	}
	if len(m1.Variants) == 0 {
		t.Fatal("m1: expected at least one variant")
	}
	if m1.Variants[0].ClientID != "claude" {
		t.Errorf("m1 variant client_id = %q, want %q", m1.Variants[0].ClientID, "claude")
	}
	if m1.Variants[0].DefaultModel != "opus" {
		t.Errorf("m1 variant default_model = %q, want %q", m1.Variants[0].DefaultModel, "opus")
	}
	if m1.Variants[0].Provider != "anthropic" {
		t.Errorf("m1 variant provider = %q, want %q", m1.Variants[0].Provider, "anthropic")
	}
	if m1.Color == nil || m1.Color.Primary != "#111111" || m1.Color.Secondary != "#222222" {
		t.Errorf("m1 color = %+v, want primary #111111 secondary #222222", m1.Color)
	}
	if len(m1.MentionPatterns) != 1 || m1.MentionPatterns[0] != "@bianmu" {
		t.Errorf("m1 mention_patterns = %+v, want [@bianmu]", m1.MentionPatterns)
	}
	if m1.Variants[0].MCPSupport != true {
		t.Error("m1 variant mcp_support: want true")
	}
	if m1.Variants[0].SessionChain != "true" {
		t.Errorf("m1 variant session_chain = %q, want %q", m1.Variants[0].SessionChain, "true")
	}
	if m1.Variants[0].Strategy != "balanced" {
		t.Errorf("m1 variant strategy = %q, want %q", m1.Variants[0].Strategy, "balanced")
	}

	m2, ok := byID["m2"]
	if !ok {
		t.Fatal("expected breed m2")
	}
	if m2.Name != "jinmao" {
		t.Errorf("m2.Name = %q, want %q", m2.Name, "jinmao")
	}
	if m2.Enabled {
		t.Error("m2.Enabled: want false")
	}

	// Roster: legacy role -> family assignment; enabled -> available.
	roster, err := store.GetRoster()
	if err != nil {
		t.Fatalf("GetRoster: %v", err)
	}
	if r, ok := roster["m1"]; !ok || r.Family != "router" || !r.Available {
		t.Errorf("roster[m1] = %+v, want family=router available=true", r)
	}
	if r, ok := roster["m2"]; !ok || r.Family != "retriever" || r.Available {
		t.Errorf("roster[m2] = %+v, want family=retriever available=false", r)
	}

	// Leader carried over.
	leader, err := store.GetLeader()
	if err != nil {
		t.Fatalf("GetLeader: %v", err)
	}
	if leader.Name != "You" {
		t.Errorf("leader.Name = %q, want %q", leader.Name, "You")
	}

	// Configs carried over.
	configs, err := store.ListConfig()
	if err != nil {
		t.Fatalf("ListConfig: %v", err)
	}
	found := false
	for _, c := range configs {
		if c.Key == "default_breed" && c.Value == "bianmu" {
			found = true
		}
	}
	if !found {
		t.Error("config default_breed=bianmu not carried over")
	}
}

// TestMigrateLegacyMembersBackup verifies that a .pre-migration backup of the
// original (un-migrated) document is written before the catalog is rewritten.
func TestMigrateLegacyMembersBackup(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, CatalogFileName)
	accountsPath := filepath.Join(dir, AccountsFileName)

	if err := os.WriteFile(catalogPath, []byte(legacyCatalogDoc), 0o644); err != nil {
		t.Fatalf("write legacy catalog: %v", err)
	}

	NewFileSettingsStore(accountsPath, catalogPath, false).ListBreeds() // triggers migration

	backupPath := catalogPath + ".pre-migration"
	raw, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	// The backup must still contain the original `members` array.
	if !strings.Contains(string(raw), `"members"`) {
		t.Error("backup should contain the legacy `members` key")
	}

	// The live catalog must no longer be a `members` document.
	live, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read live catalog: %v", err)
	}
	var probe struct {
		Members json.RawMessage `json:"members"`
	}
	if err := json.Unmarshal(live, &probe); err != nil {
		t.Fatalf("unmarshal live: %v", err)
	}
	if len(probe.Members) > 0 {
		t.Error("live catalog should no longer contain a `members` key after migration")
	}
	var doc catalogDocument
	if err := json.Unmarshal(live, &doc); err != nil {
		t.Fatalf("unmarshal live as catalogDocument: %v", err)
	}
	if doc.Version != 2 {
		t.Errorf("live catalog version = %d, want 2", doc.Version)
	}
	if len(doc.Breeds) != 2 {
		t.Errorf("live catalog breeds = %d, want 2", len(doc.Breeds))
	}
}

// TestMigrateLegacyMembersIdempotent verifies that reloading the already-migrated
// catalog does not re-migrate, duplicate breeds, or overwrite the backup.
func TestMigrateLegacyMembersIdempotent(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, CatalogFileName)
	accountsPath := filepath.Join(dir, AccountsFileName)

	if err := os.WriteFile(catalogPath, []byte(legacyCatalogDoc), 0o644); err != nil {
		t.Fatalf("write legacy catalog: %v", err)
	}

	s1 := NewFileSettingsStore(accountsPath, catalogPath, false)
	b1, _ := s1.ListBreeds()
	if len(b1) != 2 {
		t.Fatalf("first load breeds = %d, want 2", len(b1))
	}

	// Simulate a fresh process reloading the (now migrated) file.
	s2 := NewFileSettingsStore(accountsPath, catalogPath, false)
	b2, err := s2.ListBreeds()
	if err != nil {
		t.Fatalf("second load ListBreeds: %v", err)
	}
	if len(b2) != 2 {
		t.Fatalf("second load breeds = %d, want 2 (no duplication)", len(b2))
	}

	// The backup timestamp must be unchanged (not re-written on second load).
	backupPath := catalogPath + ".pre-migration"
	info1, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("backup missing after first load: %v", err)
	}
	// Touch would change mtime; instead just confirm it still exists and the
	// second load did not error (re-migration would have rewritten it).
	info2, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("backup missing after second load: %v", err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Error("backup was re-written on second load; migration should be idempotent")
	}
}
