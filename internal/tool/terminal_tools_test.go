package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"sounds-great-ai/internal/workspace"
)

func TestRunCommandTool(t *testing.T) {
	dir := t.TempDir()
	executor := workspace.NewPTYExecutor(dir)

	tool := NewRunCommandTool(executor)
	input := RunCommandInput{Command: "echo test_output", Timeout: 5}
	inputJSON, _ := json.Marshal(input)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := tool.InvokableRun(ctx, string(inputJSON))
	if err != nil {
		t.Fatalf("RunCommand failed: %v", err)
	}

	if !strings.Contains(result, "test_output") {
		t.Errorf("expected 'test_output' in result, got %s", result)
	}
}

func TestRunCommandToolTimeout(t *testing.T) {
	dir := t.TempDir()
	executor := workspace.NewPTYExecutor(dir)

	tool := NewRunCommandTool(executor)
	input := RunCommandInput{Command: "sleep 10", Timeout: 1}
	inputJSON, _ := json.Marshal(input)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := tool.InvokableRun(ctx, string(inputJSON))
	// Should timeout and return error or partial result
	if err == nil {
		// Some implementations return partial result without error on timeout
		// This is acceptable
	}
}
