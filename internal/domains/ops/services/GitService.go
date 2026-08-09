package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	opsPorts "sounds-great-ai/internal/domains/ops/ports"
)

// GitService provides git repository operations.
type GitService struct {
	repoPath string
}

// NewGitService creates a new GitService.
func NewGitService(repoPath string) *GitService {
	return &GitService{repoPath: repoPath}
}

// Status returns the current git repository status.
func (s *GitService) Status(ctx context.Context) (opsPorts.GitStatus, error) {
	branch := ""
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}

	ahead, behind := 0, 0
	if out, err := exec.Command("git", "rev-list", "--left-right", "--count", "HEAD...@{u}").Output(); err == nil {
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) >= 2 {
			fmt.Sscanf(parts[0], "%d", &ahead)
			fmt.Sscanf(parts[1], "%d", &behind)
		}
	}

	untracked, modified := 0, 0
	if out, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "??") {
				untracked++
			} else {
				modified++
			}
		}
	}

	return opsPorts.GitStatus{
		Branch: branch,
		Clean:  modified == 0 && untracked == 0,
		Ahead:  ahead,
		Behind: behind,
	}, nil
}

// DiagnosticsService provides system diagnostics.
type DiagnosticsService struct {
	sessionCountFn func() int
	logBufLenFn   func() int
	ragAvailable  func() bool
}

// NewDiagnosticsService creates a new DiagnosticsService.
func NewDiagnosticsService(
	sessionCountFn func() int,
	logBufLenFn func() int,
	ragAvailable func() bool,
) *DiagnosticsService {
	return &DiagnosticsService{
		sessionCountFn: sessionCountFn,
		logBufLenFn:    logBufLenFn,
		ragAvailable:   ragAvailable,
	}
}

// Diagnostics returns current system diagnostics.
func (s *DiagnosticsService) Diagnostics(ctx context.Context) (opsPorts.Diagnostics, error) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	pool := map[string]any{
		"goroutines": runtime.NumGoroutine(),
		"memory": map[string]any{
			"alloc":        mem.Alloc,
			"total_alloc":  mem.TotalAlloc,
			"sys":          mem.Sys,
			"heap_alloc":   mem.HeapAlloc,
			"heap_sys":     mem.HeapSys,
			"heap_objects": mem.HeapObjects,
			"num_gc":       mem.NumGC,
			"gc_pause_ns":  mem.PauseTotalNs,
		},
	}
	if s.sessionCountFn != nil {
		pool["rate_monitor"] = map[string]any{"tracked_sessions": s.sessionCountFn()}
	}
	if s.logBufLenFn != nil {
		pool["log_buffer"] = map[string]any{"entries": s.logBufLenFn()}
	}
	if s.ragAvailable != nil {
		pool["rag_cache"] = map[string]any{"available": s.ragAvailable()}
	}

	return opsPorts.Diagnostics{
		Pool:      pool,
		Memory:    map[string]any{"alloc": mem.Alloc, "sys": mem.Sys},
		Goroutines: runtime.NumGoroutine(),
	}, nil
}

// LogService provides log access.
type LogService struct {
	recentFn func(n int) []json.RawMessage
}

// NewLogService creates a new LogService.
func NewLogService(recentFn func(n int) []json.RawMessage) *LogService {
	return &LogService{recentFn: recentFn}
}

// Recent returns the last n log entries.
func (s *LogService) Recent(n int) []json.RawMessage {
	if s.recentFn == nil {
		return nil
	}
	return s.recentFn(n)
}
