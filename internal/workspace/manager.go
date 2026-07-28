package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
)

type WorkspaceManager struct {
	rootDir string
}

func NewWorkspaceManager(rootDir string) *WorkspaceManager {
	abs, _ := filepath.Abs(rootDir)
	return &WorkspaceManager{rootDir: abs}
}

func (wm *WorkspaceManager) ResolvePath(relPath string) (string, error) {
	fullPath := filepath.Join(wm.rootDir, relPath)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("path resolution failed: %w", err)
	}
	if !wm.IsPathAllowed(absPath) {
		return "", fmt.Errorf("path %q escapes workspace root", relPath)
	}
	if realPath, err := filepath.EvalSymlinks(absPath); err == nil {
		realRoot, rerr := filepath.EvalSymlinks(wm.rootDir)
		if rerr != nil {
			realRoot = wm.rootDir
		}
		if !isPathWithin(realPath, realRoot) {
			return "", fmt.Errorf("symlink path %q escapes workspace root", relPath)
		}
		return realPath, nil
	}
	return absPath, nil
}

func (wm *WorkspaceManager) IsPathAllowed(fullPath string) bool {
	return isPathWithin(fullPath, wm.rootDir)
}

func isPathWithin(fullPath, root string) bool {
	abs, err := filepath.Abs(fullPath)
	if err != nil {
		return false
	}
	rootWithSep := root
	if !strings.HasSuffix(rootWithSep, string(filepath.Separator)) {
		rootWithSep += string(filepath.Separator)
	}
	if abs == root {
		return true
	}
	return strings.HasPrefix(abs, rootWithSep)
}

func (wm *WorkspaceManager) RootDir() string {
	return wm.rootDir
}
