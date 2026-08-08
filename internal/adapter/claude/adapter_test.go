package claude

import (
	"context"
	"testing"
)

func TestAdapterCapabilities(t *testing.T) {
	a := New(nil)
	caps := a.Capabilities()
	if !caps.SupportsMCP {
		t.Error("Claude Code should support MCP")
	}
	if caps.OutputFormat != "stream-json" {
		t.Errorf("output format = %s, want stream-json", caps.OutputFormat)
	}
}

func TestAdapterHealthMissingBinary(t *testing.T) {
	a := New(nil)
	a.BinaryPath = "claude-not-installed"
	if err := a.Health(context.Background()); err == nil {
		t.Error("expected error for missing binary")
	}
}

func TestAdapterBuildArgs(t *testing.T) {
	a := New(nil)
	args := a.buildArgs("claude-opus-4-6", "/tmp/work", nil, "")
	found := false
	for _, arg := range args {
		if arg == "stream-json" {
			found = true
		}
	}
	if !found {
		t.Error("expected stream-json in args")
	}
}
