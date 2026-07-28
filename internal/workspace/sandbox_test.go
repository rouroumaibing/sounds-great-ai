package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSandboxManagerCleanSandbox(t *testing.T) {
	workDir := t.TempDir()
	baseRepo := t.TempDir()

	mgr := NewSandboxManager(baseRepo, workDir)

	// Create a sandbox directory
	sandboxPath := filepath.Join(workDir, "task-123")
	os.MkdirAll(sandboxPath, 0755)
	os.WriteFile(filepath.Join(sandboxPath, "file.txt"), []byte("test"), 0644)

	// Clean it
	ctx := context.Background()
	if err := mgr.CleanSandbox(ctx, "task-123", false); err != nil {
		t.Fatalf("CleanSandbox failed: %v", err)
	}

	if _, err := os.Stat(sandboxPath); !os.IsNotExist(err) {
		t.Error("sandbox directory should have been removed")
	}
}

func TestSandboxManagerGCTicker(t *testing.T) {
	workDir := t.TempDir()
	baseRepo := t.TempDir()

	mgr := NewSandboxManager(baseRepo, workDir)

	// Create an old sandbox directory (>24h)
	oldPath := filepath.Join(workDir, "old-task")
	os.MkdirAll(oldPath, 0755)
	// Set modification time to 25 hours ago
	oldTime := time.Now().Add(-25 * time.Hour)
	os.Chtimes(oldPath, oldTime, oldTime)

	// Create a recent sandbox directory
	newPath := filepath.Join(workDir, "new-task")
	os.MkdirAll(newPath, 0755)

	// Run GC once (manual trigger)
	mgr.runGCOnce(24)

	// Old should be removed, new should remain
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old sandbox should have been GC'd")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Error("new sandbox should still exist")
	}
}
