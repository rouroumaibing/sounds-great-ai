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

func TestSandboxManagerCleanSandboxNonexistentTask(t *testing.T) {
	workDir := t.TempDir()
	baseRepo := t.TempDir()
	mgr := NewSandboxManager(baseRepo, workDir)
	ctx := context.Background()
	if err := mgr.CleanSandbox(ctx, "nonexistent-task", false); err != nil {
		t.Errorf("CleanSandbox on nonexistent task failed: %v", err)
	}
}

func TestSandboxManagerCleanSandboxWithBranchDelete(t *testing.T) {
	workDir := t.TempDir()
	baseRepo := t.TempDir()
	mgr := NewSandboxManager(baseRepo, workDir)
	sandboxPath := filepath.Join(workDir, "task-branch")
	os.MkdirAll(sandboxPath, 0755)
	os.WriteFile(filepath.Join(sandboxPath, "file.txt"), []byte("test"), 0644)
	ctx := context.Background()
	if err := mgr.CleanSandbox(ctx, "task-branch", true); err != nil {
		t.Fatalf("CleanSandbox with branch delete failed: %v", err)
	}
	if _, err := os.Stat(sandboxPath); !os.IsNotExist(err) {
		t.Error("sandbox directory should have been removed")
	}
}

func TestSandboxManagerNewWithEmptyPaths(t *testing.T) {
	mgr := NewSandboxManager("", "")
	if mgr.BaseRepo != "" {
		t.Errorf("expected empty BaseRepo, got %q", mgr.BaseRepo)
	}
	if mgr.WorkDir != "" {
		t.Errorf("expected empty WorkDir, got %q", mgr.WorkDir)
	}
}

func TestSandboxManagerRunGCWithNonexistentWorkDir(t *testing.T) {
	mgr := NewSandboxManager("", "/nonexistent/path/xyz_abc")
	mgr.runGCOnce(24)
}

func TestSandboxManagerRunGCWithEmptyWorkDir(t *testing.T) {
	workDir := t.TempDir()
	mgr := NewSandboxManager("", workDir)
	mgr.runGCOnce(24)
}
