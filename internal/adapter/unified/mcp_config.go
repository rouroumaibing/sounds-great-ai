package unified

import (
	"encoding/json"
	"os"
)

// mcpConfigFile is the JSON structure CLI agents expect for --mcp-config.
type mcpConfigFile struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

// mcpServerEntry mirrors the CLI-native MCP server schema. Local servers use
// command/args/env; remote servers use type="http"/"sse" with url + optional
// headers. type is omitted for local servers (defaults to stdio).
type mcpServerEntry struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
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
			Type:    s.Type,
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
			URL:     s.URL,
			Headers: s.Headers,
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
