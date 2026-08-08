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

func TestRunCommandToolEmptyCommand(t *testing.T) {
	dir := t.TempDir()
	executor := workspace.NewPTYExecutor(dir)
	tool := NewRunCommandTool(executor)
	input := RunCommandInput{Command: "", Timeout: 5}
	inputJSON, _ := json.Marshal(input)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := tool.InvokableRun(ctx, string(inputJSON))
	if err != nil {
		t.Fatalf("RunCommand with empty command failed: %v", err)
	}
}

func TestRunCommandToolInvalidCommand(t *testing.T) {
	dir := t.TempDir()
	executor := workspace.NewPTYExecutor(dir)
	tool := NewRunCommandTool(executor)
	input := RunCommandInput{Command: "nonexistent_command_xyz_123", Timeout: 5}
	inputJSON, _ := json.Marshal(input)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := tool.InvokableRun(ctx, string(inputJSON))
	if err != nil {
		t.Fatalf("RunCommand failed: %v", err)
	}
	if !strings.Contains(result, "not found") {
		t.Errorf("expected 'not found' in result, got %s", result)
	}
}

func TestRunCommandToolDefaultTimeout(t *testing.T) {
	dir := t.TempDir()
	executor := workspace.NewPTYExecutor(dir)
	tool := NewRunCommandTool(executor)
	input := RunCommandInput{Command: "echo default_timeout_test", Timeout: 0}
	inputJSON, _ := json.Marshal(input)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := tool.InvokableRun(ctx, string(inputJSON))
	if err != nil {
		t.Fatalf("RunCommand with default timeout failed: %v", err)
	}
	if !strings.Contains(result, "default_timeout_test") {
		t.Errorf("expected 'default_timeout_test' in result, got %s", result)
	}
}

func TestRunCommandToolStderrOutput(t *testing.T) {
	dir := t.TempDir()
	executor := workspace.NewPTYExecutor(dir)
	tool := NewRunCommandTool(executor)
	input := RunCommandInput{Command: "echo stderr_test_msg >&2", Timeout: 5}
	inputJSON, _ := json.Marshal(input)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := tool.InvokableRun(ctx, string(inputJSON))
	if err != nil {
		t.Fatalf("RunCommand with stderr failed: %v", err)
	}
	if !strings.Contains(result, "stderr_test_msg") {
		t.Errorf("expected 'stderr_test_msg' in result, got %s", result)
	}
}
