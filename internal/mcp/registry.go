package mcp

import (
	"sounds-great-ai/pkg/pack"
)

type MCPServerConfig struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name,omitempty"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Enabled     bool              `json:"enabled"`
	// Breeds is an optional allowlist of breed IDs/names that should receive
	// this server via BuildMCPConfig. Empty means "all breeds".
	Breeds []string `json:"breeds,omitempty"`
	// Builtin marks servers seeded by the platform (e.g. the RAG "knowledge"
	// server). Builtin servers are shown read-only in the UI and cannot be
	// deleted by the operator.
	Builtin bool `json:"builtin,omitempty"`

	// Remote (HTTP/SSE) MCP server support. When URL is set the server is
	// reached over the network instead of being spawned locally. Headers carry
	// auth (e.g. Authorization) and are masked in API responses / persisted
	// with the same 0600 file protection as Env.
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	// CallbackURL is the optional HTTP fallback: when the primary transport
	// (stdio or remote) is unreachable, tool calls can be routed here. For the
	// builtin "platform" server this is the SG REST API itself.
	CallbackURL string `json:"callback_url,omitempty"`
}

type MCPRegistry struct {
	servers map[string]*MCPServerConfig
}

func NewRegistry() *MCPRegistry {
	return &MCPRegistry{servers: make(map[string]*MCPServerConfig)}
}

// Reset clears all entries. Used by the persistent store when re-syncing the
// in-memory registry from disk.
func (r *MCPRegistry) Reset() {
	r.servers = make(map[string]*MCPServerConfig)
}

func (r *MCPRegistry) Register(name string, cfg *MCPServerConfig) {
	r.servers[name] = cfg
}

// breedMatches reports whether a server's Breeds allowlist permits the given
// breed. An empty allowlist permits all breeds.
func breedMatches(s *MCPServerConfig, breed *pack.BreedConfig) bool {
	if len(s.Breeds) == 0 || breed == nil {
		return true
	}
	for _, b := range s.Breeds {
		if b == breed.ID || b == breed.Name {
			return true
		}
	}
	return false
}

func (r *MCPRegistry) ForBreed(breed *pack.BreedConfig, task string) []*MCPServerConfig {
	var result []*MCPServerConfig
	for _, s := range r.servers {
		if !s.Enabled {
			continue
		}
		if !breedMatches(s, breed) {
			continue
		}
		result = append(result, s)
	}
	return result
}

// All returns all registered MCP server configs (including disabled ones).
func (r *MCPRegistry) All() []*MCPServerConfig {
	result := make([]*MCPServerConfig, 0, len(r.servers))
	for _, s := range r.servers {
		result = append(result, s)
	}
	return result
}
