package unified

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCLICommand_ExistingBinary(t *testing.T) {
	path, err := ResolveCLICommand("echo")
	if err != nil {
		t.Fatalf("ResolveCLICommand: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path for echo")
	}
}

func TestResolveCLICommand_NotFound(t *testing.T) {
	_, err := ResolveCLICommand("nonexistent-cli-xyz123")
	if err == nil {
		t.Error("expected error for nonexistent CLI")
	}
}

func TestResolveCLICommand_KnownDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	fakeDir := filepath.Join(home, ".local", "bin")
	os.MkdirAll(fakeDir, 0755)
	fakePath := filepath.Join(fakeDir, "fake-cli-test")
	os.WriteFile(fakePath, []byte("#!/bin/sh\necho ok\n"), 0755)
	defer os.Remove(fakePath)

	InvalidateCLICache("fake-cli-test")
	path, err := ResolveCLICommand("fake-cli-test")
	if err != nil {
		t.Fatalf("ResolveCLICommand: %v", err)
	}
	if path != fakePath && filepath.Base(path) != "fake-cli-test" {
		t.Errorf("path = %q, want %q or similar", path, fakePath)
	}
}

func TestResolveCLICommand_CacheInvalidation(t *testing.T) {
	ResolveCLICommand("echo")
	InvalidateCLICache("echo")
	path, err := ResolveCLICommand("echo")
	if err != nil || path == "" {
		t.Error("cache invalidation broke resolution")
	}
}
