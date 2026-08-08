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

func TestResolvePathEmptyString(t *testing.T) {
	root := t.TempDir()
	wm := NewWorkspaceManager(root)
	resolved, err := wm.ResolvePath("")
	if err != nil {
		t.Errorf("ResolvePath(\"\") failed: %v", err)
	}
	expected, err := filepath.EvalSymlinks(root)
	if err != nil {
		expected = root
	}
	if resolved != expected {
		t.Errorf("ResolvePath(\"\") = %q, want %q", resolved, expected)
	}
}

func TestIsPathAllowedRootItself(t *testing.T) {
	root := t.TempDir()
	wm := NewWorkspaceManager(root)
	if !wm.IsPathAllowed(root) {
		t.Error("root itself should be allowed")
	}
}

func TestRootDir(t *testing.T) {
	root := t.TempDir()
	wm := NewWorkspaceManager(root)
	if wm.RootDir() != root {
		t.Errorf("RootDir() = %q, want %q", wm.RootDir(), root)
	}
}

func TestNewWorkspaceManagerRelativePath(t *testing.T) {
	wm := NewWorkspaceManager(".")
	if !filepath.IsAbs(wm.RootDir()) {
		t.Errorf("RootDir() should be absolute, got %q", wm.RootDir())
	}
}

func TestIsPathAllowedEmptyString(t *testing.T) {
	root := t.TempDir()
	wm := NewWorkspaceManager(root)
	if wm.IsPathAllowed("") {
		t.Error("empty string should not be allowed (resolves to CWD, not root)")
	}
}
