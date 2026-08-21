package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func newTestStore(t *testing.T) (*FileStore, string) {
	t.Helper()
	dir := t.TempDir()
	store := NewFileStore(dir, NewRegistry())
	return store, dir
}

func TestFileStoreAddListRemove(t *testing.T) {
	store, _ := newTestStore(t)

	if err := store.Add(MCPServerConfig{Name: "fs", Command: "npx", Args: []string{"-y", "@x/fs"}, Enabled: true}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := store.Add(MCPServerConfig{Name: "fs", Command: "node"}); err == nil {
		t.Fatal("expected conflict on duplicate name")
	}
	if _, ok := store.Get("fs"); !ok {
		t.Fatal("fs should exist")
	}
	// Registry must reflect the added server.
	if got := store.reg.ForBreed(nil, ""); len(got) != 1 || got[0].Name != "fs" {
		t.Fatalf("registry not synced: %v", got)
	}
	// Disable then verify ForBreed filters it out.
	if err := store.SetEnabled("fs", false); err != nil {
		t.Fatalf("setenabled: %v", err)
	}
	if got := store.reg.ForBreed(nil, ""); len(got) != 0 {
		t.Fatalf("disabled server should not appear in ForBreed: %v", got)
	}
	if err := store.Remove("fs"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := store.Get("fs"); ok {
		t.Fatal("fs should be gone")
	}
}

func TestFileStoreBuiltinImmutable(t *testing.T) {
	store, _ := newTestStore(t)
	store.SeedKnowledge("/bin/rag", []string{"--db", "x.db"})
	if err := store.Remove("knowledge"); err == nil {
		t.Fatal("builtin server must not be removable")
	}
	// Editing command of a builtin must be rejected.
	if _, err := store.Update("knowledge", MCPServerConfig{Command: "hacked"}); err == nil {
		t.Fatal("builtin command must be read-only")
	}
	// But toggling Enabled is allowed.
	if err := store.SetEnabled("knowledge", false); err != nil {
		t.Fatalf("toggle builtin enabled: %v", err)
	}
}

func TestFileStorePersistence(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir, NewRegistry())
	if err := store.Add(MCPServerConfig{Name: "a", Command: "c1"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Re-open from the same path.
	reopened := NewFileStore(dir, NewRegistry())
	got, ok := reopened.Get("a")
	if !ok || got.Command != "c1" {
		t.Fatalf("persistence failed: %+v", got)
	}
	// File exists on disk.
	if _, err := os.Stat(filepath.Join(dir, MCPSettingsFileName)); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}

func TestForBreedScoping(t *testing.T) {
	reg := NewRegistry()
	reg.Register("all", &MCPServerConfig{Name: "all", Enabled: true})
	reg.Register("scoped", &MCPServerConfig{Name: "scoped", Enabled: true, Breeds: []string{"bianmu"}})
	// Empty allowlist → returned; scoped → only for bianmu.
	if got := reg.ForBreed(nil, ""); len(got) != 2 {
		t.Fatalf("expected 2 for nil breed, got %d", len(got))
	}
	// A bianmu breed should see both "all" and "scoped".
	if got := reg.ForBreed(&pack.BreedConfig{ID: "bianmu"}, ""); len(got) != 2 {
		t.Fatalf("expected 2 for bianmu, got %d", len(got))
	}
	// A non-matching breed should see only "all".
	if got := reg.ForBreed(&pack.BreedConfig{ID: "jinmao"}, ""); len(got) != 1 || got[0].Name != "all" {
		t.Fatalf("expected only 'all' for jinmao, got %v", got)
	}
}

func TestFileStoreRemoteServer(t *testing.T) {
	store, _ := newTestStore(t)

	// A remote (URL-only) server is valid.
	if err := store.Add(MCPServerConfig{Name: "remote", URL: "https://mcp.example.com/sse", Headers: map[string]string{"Authorization": "Bearer x"}}); err != nil {
		t.Fatalf("add remote: %v", err)
	}
	got, ok := store.Get("remote")
	if !ok || got.URL != "https://mcp.example.com/sse" {
		t.Fatalf("remote server not stored: %+v", got)
	}
	if got.Headers["Authorization"] != "Bearer x" {
		t.Fatalf("remote headers not stored: %+v", got.Headers)
	}
	// Local (Command) server is also valid.
	if err := store.Add(MCPServerConfig{Name: "local", Command: "npx", Args: []string{"-y", "x"}}); err != nil {
		t.Fatalf("add local: %v", err)
	}
	// Neither command nor URL is rejected.
	if err := store.Add(MCPServerConfig{Name: "bad"}); err == nil {
		t.Fatal("expected error when neither command nor url set")
	}
	// Both command and URL is rejected.
	if err := store.Add(MCPServerConfig{Name: "both", Command: "x", URL: "https://y"}); err == nil {
		t.Fatal("expected error when both command and url set")
	}
	// Update can change the URL (PATCH semantics for headers).
	if _, err := store.Update("remote", MCPServerConfig{URL: "https://mcp2.example.com"}); err != nil {
		t.Fatalf("update remote url: %v", err)
	}
	if got, _ := store.Get("remote"); got.URL != "https://mcp2.example.com" {
		t.Fatalf("remote url not updated: %+v", got)
	}
}

func TestSeedPlatformCallbackURL(t *testing.T) {
	store, _ := newTestStore(t)
	store.SeedPlatform("/bin/platform-mcp", []string{"--api-base", "http://localhost:8080"}, map[string]string{"SG_API_TOKEN": "t"}, map[string]string{"X-Extra": "1"}, "http://localhost:8080")
	got, ok := store.Get("platform")
	if !ok {
		t.Fatal("platform server not seeded")
	}
	if got.CallbackURL != "http://localhost:8080" {
		t.Fatalf("platform callback_url not seeded: %q", got.CallbackURL)
	}
	if got.Headers["X-Extra"] != "1" {
		t.Fatalf("platform headers not seeded: %+v", got.Headers)
	}
}
