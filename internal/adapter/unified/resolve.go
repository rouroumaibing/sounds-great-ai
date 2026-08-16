package unified

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		// Cached path is stale (binary moved/removed): drop it so we re-resolve
		// from PATH/known dirs instead of repeatedly hitting a dead path.
		resolveCacheMu.RUnlock()
		InvalidateCLICache(command)
	} else {
		resolveCacheMu.RUnlock()
	}

	if path, err := exec.LookPath(command); err == nil {
		cacheResolved(command, path)
		return path, nil
	}

		home, err := os.UserHomeDir()
	if err == nil {
		for _, dir := range knownCLIDirs {
			for _, name := range candidateNames(command) {
				fullPath := filepath.Join(home, dir, name)
				if isExecutable(fullPath) {
					cacheResolved(command, fullPath)
					return fullPath, nil
				}
			}
		}

		nvmDir := filepath.Join(home, ".nvm", "versions", "node")
		if entries, err := os.ReadDir(nvmDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				for _, name := range candidateNames(command) {
					fullPath := filepath.Join(nvmDir, entry.Name(), "bin", name)
					if isExecutable(fullPath) {
						cacheResolved(command, fullPath)
						return fullPath, nil
					}
				}
			}
		}
	}

	if brewPath, err := exec.LookPath("brew"); err == nil {
		brewDir := filepath.Dir(filepath.Dir(brewPath))
		for _, name := range candidateNames(command) {
			fullPath := filepath.Join(brewDir, "bin", name)
			if isExecutable(fullPath) {
				cacheResolved(command, fullPath)
				return fullPath, nil
			}
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

// candidateNames returns the base command plus platform-specific executable
// extensions (Windows) so known-directory resolution can find e.g. claude.cmd
// / claude.exe (R9). On non-Windows it returns just the command.
func candidateNames(command string) []string {
	names := []string{command}
	if runtime.GOOS == "windows" {
		for _, ext := range []string{".exe", ".cmd", ".bat", ".ps1"} {
			names = append(names, command+ext)
		}
	}
	return names
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		// Windows determines executability by extension (PATHEXT), not Unix
		// permission bits — which Go reports as 0 there.
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}
