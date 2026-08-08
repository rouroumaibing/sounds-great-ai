package unified

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sounds-great-ai/internal/hooks"
)

func TestProcessManagerSpawnAndRead(t *testing.T) {
	pm := NewProcessManager()
	r, err := pm.Spawn(context.Background(), "echo", []string{"hello"}, "")
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("expected 'hello' in output, got %q", buf.String())
	}
}

func TestProcessManagerStdinInjection(t *testing.T) {
	pm := NewProcessManager()
	r, err := pm.Spawn(context.Background(), "cat", nil, "injected-via-stdin")
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "injected-via-stdin") {
		t.Fatalf("expected stdin content in output, got %q", buf.String())
	}
}

func TestProcessManagerContextCancellation(t *testing.T) {
	pm := NewProcessManager()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _ = pm.Spawn(ctx, "sleep", []string{"10"}, "")
	// The process should be killed when ctx expires
	// We just verify Spawn doesn't hang
}

func TestProcessManagerDefaults(t *testing.T) {
	pm := NewProcessManager()
	if pm.KillGraceMs != 3000 {
		t.Errorf("KillGraceMs = %d, want 3000", pm.KillGraceMs)
	}
	if pm.InterruptGraceMs != 2000 {
		t.Errorf("InterruptGraceMs = %d, want 2000", pm.InterruptGraceMs)
	}
}

func TestSpawnWithHooks_InjectsPrompt(t *testing.T) {
	pm := NewProcessManager()

	hooksDir := t.TempDir()
	hookDir := filepath.Join(hooksDir, "s1-test")
	os.MkdirAll(hookDir, 0755)
	yaml := `id: S1
name: Test
stage: session-init
order: 100
version: 1
enabled: true
disableable: false
template: template.md
safetyTier: readonly
governanceTier: immutable
`
	os.WriteFile(filepath.Join(hookDir, "hook.yaml"), []byte(yaml), 0644)
	os.WriteFile(filepath.Join(hookDir, "template.md"), []byte("INJECTED_HOOK_CONTENT"), 0644)

	reg := hooks.NewRegistry(hooksDir)
	reg.Scan()
	pipeline := hooks.NewPipeline(reg, hooks.DefaultResolvers())
	input := &hooks.AssemblerInput{BreedID: "bianmu", BreedName: "Border Collie"}

	r, err := pm.SpawnWithHooks(context.Background(), "cat", nil, "original-input", pipeline, input)
	if err != nil {
		t.Fatalf("SpawnWithHooks: %v", err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	if !strings.Contains(output, "INJECTED_HOOK_CONTENT") {
		t.Errorf("output missing hook content, got %q", output)
	}
	if !strings.Contains(output, "original-input") {
		t.Errorf("output missing original input, got %q", output)
	}
}
