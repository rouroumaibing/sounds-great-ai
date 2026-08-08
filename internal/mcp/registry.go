package mcp

import "sounds-great-ai/internal/config"

type MCPServerConfig struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
	Enabled bool
}

type MCPRegistry struct {
	servers map[string]*MCPServerConfig
}

func NewRegistry() *MCPRegistry {
	return &MCPRegistry{servers: make(map[string]*MCPServerConfig)}
}

func (r *MCPRegistry) Register(name string, cfg *MCPServerConfig) {
	r.servers[name] = cfg
}

func (r *MCPRegistry) ForBreed(breed *config.BreedConfig, task string) []*MCPServerConfig {
	var result []*MCPServerConfig
	for _, s := range r.servers {
		if !s.Enabled {
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
