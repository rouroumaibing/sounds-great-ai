package codex

import (
	"context"
	"testing"
)

func TestAdapterCapabilities(t *testing.T) {
	a := New(nil)
	caps := a.Capabilities()
	if !caps.SupportsMCP {
		t.Error("Codex CLI should support MCP")
	}
	if caps.OutputFormat != "json" {
		t.Errorf("output format = %s, want json", caps.OutputFormat)
	}
}

func TestAdapterHealthMissingBinary(t *testing.T) {
	a := New(nil)
	a.BinaryPath = "codex-not-installed"
	if err := a.Health(context.Background()); err == nil {
		t.Error("expected error for missing binary")
	}
}
