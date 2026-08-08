package capability

import (
	"context"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestIntegrationCommandCheckBlocksDangerousCommand(t *testing.T) {
	cc := NewCommandCheck()
	input := &pack.TaskInput{
		Command: "rm -rf /",
		Path:    "/workspace/src/main.go",
	}
	out, err := cc.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("CommandCheck.Run: %v", err)
	}
	if out.Approved {
		t.Error("CommandCheck should block 'rm -rf /', got Approved = true")
	}
	if out.Reason == "" {
		t.Error("CommandCheck should have a reason for blocking")
	}
}

func TestIntegrationCommandCheckAllowsSafeCommand(t *testing.T) {
	cc := NewCommandCheck()
	input := &pack.TaskInput{
		Command: "ls",
		Path:    "/workspace/src/main.go",
	}
	out, err := cc.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("CommandCheck.Run: %v", err)
	}
	if !out.Approved {
		t.Errorf("CommandCheck should allow 'ls', got Approved = false, Reason = %q", out.Reason)
	}
}

func TestIntegrationPathValidateBlocksConfigFileWrite(t *testing.T) {
	pv := NewPathValidate()
	input := &pack.TaskInput{
		Command: "ls",
		Path:    "go.mod",
	}
	out, err := pv.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("PathValidate.Run: %v", err)
	}
	if out.Approved {
		t.Error("PathValidate should block writing to go.mod, got Approved = true")
	}
}

func TestIntegrationPathValidateAllowsNormalPath(t *testing.T) {
	pv := NewPathValidate()
	input := &pack.TaskInput{
		Command: "ls",
		Path:    "/workspace/src/main.go",
	}
	out, err := pv.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("PathValidate.Run: %v", err)
	}
	if !out.Approved {
		t.Errorf("PathValidate should allow normal path, got Approved = false, Reason = %q", out.Reason)
	}
}
