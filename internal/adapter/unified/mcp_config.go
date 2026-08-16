package unified

import (
	"encoding/json"
	"os"
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

// WriteMCPConfigFile writes the MCP server configuration to an ephemeral temp
// file (outside the project tree) and returns the absolute path. The CLI reads
// it via --mcp-config <path>; the caller is responsible for removing it once
// the process exits (see SpawnHandle.OnExit). Writing to a temp dir — rather
// than workDir/.mcp-config.json — avoids leaking MCP server addresses/tokens
// into the repo (and a stray tracked file). Returns empty string if config is
// nil or has no servers.
func WriteMCPConfigFile(mcp *MCPConfig, _ string) (string, error) {
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

	tmp, err := os.CreateTemp("", "sg-mcp-*.json")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}
