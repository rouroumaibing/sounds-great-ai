package mcp

import (
	"sync"
	"testing"
)

func TestMCPRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register("filesystem", &MCPServerConfig{Name: "filesystem", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}, Enabled: true})
	servers := r.ForBreed(nil, "")
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Name != "filesystem" {
		t.Errorf("name = %s", servers[0].Name)
	}
}

func TestMCPRegistryDisabledNotReturned(t *testing.T) {
	r := NewRegistry()
	r.Register("disabled", &MCPServerConfig{Name: "disabled", Enabled: false})
	servers := r.ForBreed(nil, "")
	if len(servers) != 0 {
		t.Fatalf("expected 0 servers, got %d", len(servers))
	}
}

func TestMCPRegistryRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	r.Register("fs", &MCPServerConfig{Name: "fs", Command: "npx", Enabled: true})
	r.Register("fs", &MCPServerConfig{Name: "fs-v2", Command: "node", Enabled: true})
	servers := r.ForBreed(nil, "")
	if len(servers) != 1 {
		t.Fatalf("expected 1 server after duplicate register, got %d", len(servers))
	}
	if servers[0].Name != "fs-v2" {
		t.Errorf("expected overwritten name fs-v2, got %s", servers[0].Name)
	}
	if servers[0].Command != "node" {
		t.Errorf("expected overwritten command node, got %s", servers[0].Command)
	}
}

func TestMCPRegistryForBreedEmpty(t *testing.T) {
	r := NewRegistry()
	servers := r.ForBreed(nil, "")
	if servers != nil && len(servers) != 0 {
		t.Fatalf("expected nil or empty slice for empty registry, got %v", servers)
	}
}

func TestMCPRegistryAllMultiple(t *testing.T) {
	r := NewRegistry()
	r.Register("a", &MCPServerConfig{Name: "a", Enabled: true})
	r.Register("b", &MCPServerConfig{Name: "b", Enabled: false})
	r.Register("c", &MCPServerConfig{Name: "c", Enabled: true})
	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 servers from All(), got %d", len(all))
	}
}

func TestMCPRegistryAllIncludesDisabled(t *testing.T) {
	r := NewRegistry()
	r.Register("enabled", &MCPServerConfig{Name: "enabled", Enabled: true})
	r.Register("disabled", &MCPServerConfig{Name: "disabled", Enabled: false})
	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 servers from All() including disabled, got %d", len(all))
	}
}

func TestMCPRegistryAllEmpty(t *testing.T) {
	r := NewRegistry()
	all := r.All()
	if len(all) != 0 {
		t.Fatalf("expected 0 servers from empty registry All(), got %d", len(all))
	}
}

func TestMCPRegistryForBreedOnlyEnabled(t *testing.T) {
	r := NewRegistry()
	r.Register("a", &MCPServerConfig{Name: "a", Enabled: true})
	r.Register("b", &MCPServerConfig{Name: "b", Enabled: false})
	r.Register("c", &MCPServerConfig{Name: "c", Enabled: true})
	servers := r.ForBreed(nil, "")
	if len(servers) != 2 {
		t.Fatalf("expected 2 enabled servers, got %d", len(servers))
	}
}

func TestMCPRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	r.Register("server1", &MCPServerConfig{Name: "server1", Enabled: true})
	r.Register("server2", &MCPServerConfig{Name: "server2", Enabled: false})
	r.Register("server3", &MCPServerConfig{Name: "server3", Enabled: true})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = r.ForBreed(nil, "")
		}()
		go func() {
			defer wg.Done()
			_ = r.All()
		}()
	}
	wg.Wait()
	servers := r.ForBreed(nil, "")
	if len(servers) != 2 {
		t.Fatalf("expected 2 enabled servers after concurrent reads, got %d", len(servers))
	}
	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 total servers after concurrent reads, got %d", len(all))
	}
}

func TestMCPRegistryRegisterNilConfig(t *testing.T) {
	r := NewRegistry()
	r.Register("nil-cfg", nil)
	all := r.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 entry even with nil config, got %d", len(all))
	}
}
