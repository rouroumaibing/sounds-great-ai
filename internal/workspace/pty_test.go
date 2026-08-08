package workspace

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPTYExecuteEcho(t *testing.T) {
	dir := t.TempDir()
	executor := NewPTYExecutor(dir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	outputCh, err := executor.Execute(ctx, "echo hello")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var outputs []PTYOutput
	for out := range outputCh {
		outputs = append(outputs, out)
	}

	var combined strings.Builder
	for _, o := range outputs {
		combined.WriteString(o.Data)
	}
	if !strings.Contains(combined.String(), "hello") {
		t.Errorf("expected 'hello' in output, got %q", combined.String())
	}
}

func TestPTYExecuteTimeout(t *testing.T) {
	dir := t.TempDir()
	executor := NewPTYExecutor(dir)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	outputCh, err := executor.Execute(ctx, "sleep 100")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should be cancelled by context timeout
	for range outputCh {
		// drain channel
	}
	// If we reach here, the command was cancelled (good)
}

func TestRunIsolatedCmd(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	out, err := RunIsolatedCmd(ctx, dir, "echo isolated")
	if err != nil {
		t.Fatalf("RunIsolatedCmd failed: %v", err)
	}
	if !strings.Contains(out, "isolated") {
		t.Errorf("expected 'isolated' in output, got %q", out)
	}
}

func TestPTYExecuteInvalidCommand(t *testing.T) {
	dir := t.TempDir()
	executor := NewPTYExecutor(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	outputCh, err := executor.Execute(ctx, "nonexistent_command_xyz_123")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	var combined strings.Builder
	for out := range outputCh {
		combined.WriteString(out.Data)
	}
	if !strings.Contains(combined.String(), "not found") {
		t.Errorf("expected 'not found' in output, got %q", combined.String())
	}
}

func TestPTYExecuteEmptyCommand(t *testing.T) {
	dir := t.TempDir()
	executor := NewPTYExecutor(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	outputCh, err := executor.Execute(ctx, "")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	for range outputCh {
		// drain channel
	}
}

func TestPTYExecuteStderr(t *testing.T) {
	dir := t.TempDir()
	executor := NewPTYExecutor(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	outputCh, err := executor.Execute(ctx, "echo stderr_test_output >&2")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	var combined strings.Builder
	for out := range outputCh {
		combined.WriteString(out.Data)
	}
	if !strings.Contains(combined.String(), "stderr_test_output") {
		t.Errorf("expected 'stderr_test_output' in output, got %q", combined.String())
	}
}

func TestRunIsolatedCmdError(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	out, err := RunIsolatedCmd(ctx, dir, "nonexistent_command_xyz_123")
	if err == nil {
		t.Error("expected error for invalid command")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' in output, got %q", out)
	}
}

func TestRunIsolatedCmdEmptyCommand(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	_, err := RunIsolatedCmd(ctx, dir, "")
	if err != nil {
		t.Errorf("empty command should succeed: %v", err)
	}
}
