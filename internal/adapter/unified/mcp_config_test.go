package unified

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteMCPConfigFile_Nil(t *testing.T) {
	path, err := WriteMCPConfigFile(nil, "/tmp")
	if err != nil {
		t.Fatalf("nil config should not error: %v", err)
	}
	if path != "" {
		t.Fatalf("nil config should return empty path, got %s", path)
	}
}

func TestWriteMCPConfigFile_EmptyServers(t *testing.T) {
	cfg := &MCPConfig{Servers: []MCPServer{}}
	path, err := WriteMCPConfigFile(cfg, "/tmp")
	if err != nil {
		t.Fatalf("empty servers should not error: %v", err)
	}
	if path != "" {
		t.Fatalf("empty servers should return empty path, got %s", path)
	}
}

func TestWriteMCPConfigFile_WithServers(t *testing.T) {
	cfg := &MCPConfig{
		Servers: []MCPServer{
			{Name: "knowledge", Command: "sounds-great-mcp-server", Args: []string{"--db", "/tmp/test.db"}},
		},
	}
	tmpDir, _ := os.MkdirTemp("", "mcp-test")
	defer os.RemoveAll(tmpDir)

	path, err := WriteMCPConfigFile(cfg, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	dir := filepath.Dir(path)
	if dir != tmpDir {
		t.Fatalf("expected file in %s, got %s", tmpDir, dir)
	}
}
