package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathValid(t *testing.T) {
	root := t.TempDir()
	wm := NewWorkspaceManager(root)
	tests := []string{"main.go", "src/app.go", "deep/nested/dir/file.txt"}
	for _, rel := range tests {
		resolved, err := wm.ResolvePath(rel)
		if err != nil {
			t.Errorf("ResolvePath(%q) failed: %v", rel, err)
		}
		expected := filepath.Join(root, rel)
		if resolved != expected {
			t.Errorf("ResolvePath(%q) = %q, want %q", rel, resolved, expected)
		}
	}
}

func TestResolvePathTraversalBlocked(t *testing.T) {
	root := t.TempDir()
	wm := NewWorkspaceManager(root)
	tests := []string{"../../../etc/passwd", "../../etc/shadow", "..", "../outside"}
	for _, rel := range tests {
		_, err := wm.ResolvePath(rel)
		if err == nil {
			t.Errorf("ResolvePath(%q) should have been blocked", rel)
		}
	}
}

func TestResolvePathSymlinkEscapeBlocked(t *testing.T) {
	root := t.TempDir()
	wm := NewWorkspaceManager(root)
	outside := t.TempDir()
	targetFile := filepath.Join(outside, "secret.txt")
	os.WriteFile(targetFile, []byte("secret"), 0644)
	linkPath := filepath.Join(root, "escape_link")
	os.Symlink(outside, linkPath)
	_, err := wm.ResolvePath("escape_link/secret.txt")
	if err == nil {
		t.Error("symlink escape should have been blocked")
	}
}

func TestIsPathAllowed(t *testing.T) {
	root := t.TempDir()
	wm := NewWorkspaceManager(root)
	if !wm.IsPathAllowed(filepath.Join(root, "file.go")) {
		t.Error("path inside root should be allowed")
	}
	if wm.IsPathAllowed(filepath.Join(root, "..", "outside")) {
		t.Error("path outside root should not be allowed")
	}
}
