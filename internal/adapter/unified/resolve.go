package unified

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var (
	resolveCache   = make(map[string]string)
	resolveCacheMu sync.RWMutex
)

var knownCLIDirs = []string{
	".local/bin",
	".claude/bin",
	".claude/local/bin",
	".fnm/aliases/default/bin",
	".volta/bin",
	".nix-profile/bin",
}

func ResolveCLICommand(command string) (string, error) {
	resolveCacheMu.RLock()
	if cached, ok := resolveCache[command]; ok {
		if _, err := os.Stat(cached); err == nil {
			resolveCacheMu.RUnlock()
			return cached, nil
		}
	}
	resolveCacheMu.RUnlock()

	if path, err := exec.LookPath(command); err == nil {
		cacheResolved(command, path)
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err == nil {
		for _, dir := range knownCLIDirs {
			fullPath := filepath.Join(home, dir, command)
			if isExecutable(fullPath) {
				cacheResolved(command, fullPath)
				return fullPath, nil
			}
		}

		nvmDir := filepath.Join(home, ".nvm", "versions", "node")
		if entries, err := os.ReadDir(nvmDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				fullPath := filepath.Join(nvmDir, entry.Name(), "bin", command)
				if isExecutable(fullPath) {
					cacheResolved(command, fullPath)
					return fullPath, nil
				}
			}
		}
	}

	if brewPath, err := exec.LookPath("brew"); err == nil {
		brewDir := filepath.Dir(filepath.Dir(brewPath))
		fullPath := filepath.Join(brewDir, "bin", command)
		if isExecutable(fullPath) {
			cacheResolved(command, fullPath)
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("CLI command %q not found in PATH or known directories", command)
}

func InvalidateCLICache(command string) {
	resolveCacheMu.Lock()
	delete(resolveCache, command)
	resolveCacheMu.Unlock()
}

func cacheResolved(command, path string) {
	resolveCacheMu.Lock()
	resolveCache[command] = path
	resolveCacheMu.Unlock()
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
