package mcp

import "testing"

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
