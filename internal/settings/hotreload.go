package settings

import (
	"os"
	"sync"
	"time"
)

// HotReloader watches a set of files and invokes onReload after a debounce
// period (default 30s) of inactivity following a detected modification.
//
// This powers the original "config hot-load" intent: when a settings file is
// edited externally (or by another process), the in-memory cache is refreshed
// automatically ~30s later without a restart. The debounce avoids thrashing
// while a file is being written incrementally.
type HotReloader struct {
	paths      []string
	interval   time.Duration
	debounce   time.Duration
	onReload   func()
	stop       chan struct{}
	mu         sync.Mutex
	mtimes     map[string]time.Time
	pendingAt  time.Time
	running    bool
}

// NewHotReloader creates a watcher for the given paths. onReload is invoked
// (on a background goroutine) once the debounce window elapses after the last
// detected change. Call Start to begin polling.
func NewHotReloader(paths []string, onReload func()) *HotReloader {
	return &HotReloader{
		paths:    paths,
		interval: 3 * time.Second,
		debounce: 30 * time.Second,
		onReload: onReload,
		stop:     make(chan struct{}),
		mtimes:   make(map[string]time.Time),
	}
}

// Start begins the background polling loop. Safe to call once.
func (h *HotReloader) Start() {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()
	go h.loop()
}

// Stop terminates the polling loop.
func (h *HotReloader) Stop() {
	select {
	case <-h.stop:
		// already closed
	default:
		close(h.stop)
	}
}

func (h *HotReloader) loop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			changed := false
			for _, p := range h.paths {
				mt := fileModTime(p)
				if !mt.Equal(h.mtimes[p]) {
					h.mtimes[p] = mt
					changed = true
				}
			}
			if changed {
				h.mu.Lock()
				h.pendingAt = time.Now().Add(h.debounce)
				h.mu.Unlock()
			}
			h.mu.Lock()
			due := h.pendingAt
			h.mu.Unlock()
			if !due.IsZero() && !time.Now().Before(due) {
				if h.onReload != nil {
					h.onReload()
				}
				h.mu.Lock()
				h.pendingAt = time.Time{}
				h.mu.Unlock()
			}
		}
	}
}

// fileModTime returns the modification time of a file, or the zero time if the
// file does not exist (so a freshly-created file is treated as a change).
func fileModTime(p string) time.Time {
	info, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
