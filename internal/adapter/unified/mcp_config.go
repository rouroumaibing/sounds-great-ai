package unified

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// mcpConfigFile is the JSON structure CLI agents expect for --mcp-config.
type mcpConfigFile struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// WriteMCPConfigFile writes the MCP server configuration to a JSON file
// in workDir and returns the file path. Returns empty string if config
// is nil or has no servers.
func WriteMCPConfigFile(mcp *MCPConfig, workDir string) (string, error) {
	if mcp == nil || len(mcp.Servers) == 0 {
		return "", nil
	}

	file := mcpConfigFile{MCPServers: make(map[string]mcpServerEntry)}
	for _, s := range mcp.Servers {
		file.MCPServers[s.Name] = mcpServerEntry{
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
		}
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", err
	}

	path := filepath.Join(workDir, ".mcp-config.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}
