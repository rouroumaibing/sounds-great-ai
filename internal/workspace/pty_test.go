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
