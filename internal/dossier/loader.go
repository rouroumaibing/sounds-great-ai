package dossier

import (
	"os"
	"path/filepath"
	"sync"
)

// DossierRelativePath is the dossier location relative to the project root.
// It is a repo doc (git-versioned), NOT runtime config — updates land via the
// distillation pipeline as commits, so profile changes carry full history.
const DossierRelativePath = "docs/team/dog-dossier.md"

// cachedDossier holds one project root's parse result.
type cachedDossier struct {
	profiles map[string]DossierProfile
	// found records whether the dossier file existed and loaded. Missing
	// file (community scenario) is a silent fallback; a present-but-unreadable
	// file is a drift signal the caller may warn on.
	found bool
}

// Loader loads and caches dossier profiles per project root. Cache lifetime
// is process-scoped: the distillation pipeline invalidates it explicitly after
// an apply, and a server restart always gets a fresh read.
type Loader struct {
	mu    sync.RWMutex
	cache map[string]cachedDossier
}

// NewLoader creates a dossier loader.
func NewLoader() *Loader {
	return &Loader{cache: make(map[string]cachedDossier)}
}

// Load returns the dossier profiles for a project root, loading and caching
// on first access.
func (l *Loader) Load(projectRoot string) map[string]DossierProfile {
	c := l.loadCached(projectRoot)
	return c.profiles
}

// IsAvailable reports whether the dossier file was found and loaded for the
// given root. Distinguishes "community has no dossier" (silent fallback OK)
// from "dossier exists but failed to load" (drift signal).
func (l *Loader) IsAvailable(projectRoot string) bool {
	return l.loadCached(projectRoot).found
}

// HasEntry reports whether a specific dog has a structured-profile entry.
// Used to scope drift warnings: only tracked dogs (with an entry) missing
// l0RosterSummary deserve a warning; runtime dogs with no entry fall back
// silently.
func (l *Loader) HasEntry(projectRoot, dogID string) bool {
	_, ok := l.loadCached(projectRoot).profiles[dogID]
	return ok
}

// GetProfile returns the full profile for a dog.
func (l *Loader) GetProfile(projectRoot, dogID string) (DossierProfile, bool) {
	p, ok := l.loadCached(projectRoot).profiles[dogID]
	return p, ok
}

// GetOneLiner returns the identity one-liner for a dog.
func (l *Loader) GetOneLiner(projectRoot, dogID string) (string, bool) {
	p, ok := l.loadCached(projectRoot).profiles[dogID]
	if !ok || p.OneLiner == "" {
		return "", false
	}
	return p.OneLiner, true
}

// GetRosterSummary returns the roster strengths cell for a dog.
func (l *Loader) GetRosterSummary(projectRoot, dogID string) (string, bool) {
	p, ok := l.loadCached(projectRoot).profiles[dogID]
	if !ok || p.L0RosterSummary == "" {
		return "", false
	}
	return p.L0RosterSummary, true
}

// GetRoutingNote returns the route-critical boundary note for a dog.
func (l *Loader) GetRoutingNote(projectRoot, dogID string) (string, bool) {
	p, ok := l.loadCached(projectRoot).profiles[dogID]
	if !ok || p.L0RoutingNote == "" {
		return "", false
	}
	return p.L0RoutingNote, true
}

// Invalidate drops the cache for a project root so the next read re-parses
// the file. Called by the distillation apply path after writing the dossier.
func (l *Loader) Invalidate(projectRoot string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.cache, projectRoot)
}

// ResetForTest clears the entire cache.
func (l *Loader) ResetForTest() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cache = make(map[string]cachedDossier)
}

func (l *Loader) loadCached(projectRoot string) cachedDossier {
	l.mu.RLock()
	if c, ok := l.cache[projectRoot]; ok {
		l.mu.RUnlock()
		return c
	}
	l.mu.RUnlock()

	content, err := os.ReadFile(filepath.Join(projectRoot, DossierRelativePath))
	if err != nil {
		if os.IsNotExist(err) {
			// Community scenario: no dossier yet. Do NOT cache the miss —
			// the file may appear later (git pull, first apply) and every
			// getter is cheap enough to re-stat.
			return cachedDossier{profiles: make(map[string]DossierProfile)}
		}
		// Present but unreadable (permissions, encoding) — cache the drift
		// state (found, empty) so consumers can surface it.
		c := cachedDossier{profiles: make(map[string]DossierProfile), found: true}
		l.mu.Lock()
		l.cache[projectRoot] = c
		l.mu.Unlock()
		return c
	}

	c := cachedDossier{profiles: ParseDossierProfiles(string(content)), found: true}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cache[projectRoot] = c
	return c
}

// Reader is a project-root-bound projection of the Loader satisfying the
// prompt package's DossierReader interface (structural typing — no import
// cycle). One Reader is attached to the prompt Builder at assembly time.
type Reader struct {
	loader      *Loader
	projectRoot string
}

// NewReader creates a dossier reader bound to a project root.
func NewReader(loader *Loader, projectRoot string) *Reader {
	return &Reader{loader: loader, projectRoot: projectRoot}
}

// OneLiner returns the identity one-liner for a dog.
func (r *Reader) OneLiner(dogID string) (string, bool) {
	return r.loader.GetOneLiner(r.projectRoot, dogID)
}

// RosterSummary returns the roster strengths cell for a dog.
func (r *Reader) RosterSummary(dogID string) (string, bool) {
	return r.loader.GetRosterSummary(r.projectRoot, dogID)
}

// RoutingNote returns the route-critical boundary note for a dog.
func (r *Reader) RoutingNote(dogID string) (string, bool) {
	return r.loader.GetRoutingNote(r.projectRoot, dogID)
}
