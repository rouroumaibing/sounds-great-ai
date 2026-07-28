package capability

import (
	"context"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestCommandCheckNameVersion(t *testing.T) {
	c := NewCommandCheck()
	if c.Name() != "command_check" {
		t.Errorf("Name = %q, want %q", c.Name(), "command_check")
	}
	if c.Version() != "v1" {
		t.Errorf("Version = %q, want %q", c.Version(), "v1")
	}
}

func TestCommandCheckBlocksRmRf(t *testing.T) {
	c := NewCommandCheck()
	input := &pack.TaskInput{Command: "rm -rf /"}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Approved {
		t.Error("rm -rf / should be blocked, got Approved = true")
	}
	if out.Reason == "" {
		t.Error("Reason should not be empty for blocked command")
	}
}

func TestCommandCheckAllowsLs(t *testing.T) {
	c := NewCommandCheck()
	input := &pack.TaskInput{Command: "ls"}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !out.Approved {
		t.Errorf("ls should be allowed, got Approved = false, Reason = %q", out.Reason)
	}
}

func TestCommandCheckBlocksShellFeatures(t *testing.T) {
	c := NewCommandCheck()
	tests := []struct {
		name string
		cmd  string
	}{
		{"pipe", "ls | grep foo"},
		{"semicolon", "ls; rm file"},
		{"redirect", "ls > file"},
		{"var_expand", "echo $HOME"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &pack.TaskInput{Command: tt.cmd}
			out, err := c.Run(context.Background(), input)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if out.Approved {
				t.Errorf("%q should be blocked", tt.cmd)
			}
		})
	}
}

func TestCommandCheckInitHealthClose(t *testing.T) {
	c := NewCommandCheck()
	if err := c.Init(context.Background()); err != nil {
		t.Errorf("Init: %v", err)
	}
	if err := c.Health(); err != nil {
		t.Errorf("Health: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
