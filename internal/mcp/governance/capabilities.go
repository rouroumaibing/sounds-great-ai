// Package governance additions for P1-D:
//   - CapabilityRegistry: single source of truth for the governed surface with
//     a global catalog + per-project overrides (F249) and a capability-surface
//     registry (F223).
//   - SurfaceLifecycle: resource-family atomic switch / dual-exposure / lazy
//     registration (F286).
//
// All access is goroutine-safe; overrides never mutate the global catalog.
package governance

import (
	"sort"
	"sync"
)

// ToolOverride patches a single global tool for a specific project's surface
// (F249: per-project override over the global single source).
type ToolOverride struct {
	Name        string
	Disabled    bool  // remove this tool from the project's surface
	ReadOnly    *bool // override the governance ReadOnly annotation
	DualExposure *bool // override dual-exposure for this tool (F286)
	Lazy        *bool // override lazy registration for this tool (F286)
}

// SurfaceLifecycle governs how a tool is exposed on the MCP surface (F286):
//   - ResourceFamily groups tools that switch together under AtomicSwitch.
//   - DualExposure exposes both the read-only and the write variant of a tool.
//   - Lazy keeps the tool out of the baseline digest until first used.
//   - AtomicSwitch makes enable/disable apply to the whole ResourceFamily.
type SurfaceLifecycle struct {
	ResourceFamily string
	DualExposure   bool
	Lazy           bool
	AtomicSwitch   bool
}

// CapabilityRegistry is the single source of truth for the governed surface
// (F223/F249): a global catalog (seeded from Catalog) plus per-project
// overrides, and per-tool surface lifecycle (F286).
type CapabilityRegistry struct {
	mu        sync.RWMutex
	global    []ToolDefinition
	overrides map[string][]ToolOverride // project id -> overrides
	lifecycle map[string]SurfaceLifecycle
	familyOn  map[string]bool // resource family -> enabled (AtomicSwitch)
}

// NewCapabilityRegistry seeds the registry from the global Catalog().
func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{
		global:    Catalog(),
		overrides: make(map[string][]ToolOverride),
		lifecycle: make(map[string]SurfaceLifecycle),
		familyOn:  make(map[string]bool),
	}
}

// Global returns the unmodified global catalog (single source).
func (r *CapabilityRegistry) Global() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneTools(r.global)
}

// ApplyOverride records a per-project override for a tool (F249).
func (r *CapabilityRegistry) ApplyOverride(project string, o ToolOverride) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.overrides[project] = append(r.overrides[project], o)
}

// ResetProject clears all overrides for a project (F249).
func (r *CapabilityRegistry) ResetProject(project string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.overrides, project)
}

// ForProject resolves the effective surface for a project: the global catalog
// with per-project overrides applied (disabled removed, ReadOnly patched). This
// is the single resolved source for that project (F249).
func (r *CapabilityRegistry) ForProject(project string) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if project == "" {
		return cloneTools(r.global)
	}
	ov := make(map[string]ToolOverride, len(r.overrides[project]))
	for _, o := range r.overrides[project] {
		ov[o.Name] = o
	}
	out := make([]ToolDefinition, 0, len(r.global))
	for _, t := range r.global {
		if o, ok := ov[t.Name]; ok {
			if o.Disabled {
				continue // per-project removal
			}
			if o.ReadOnly != nil {
				t.ReadOnly = *o.ReadOnly
			}
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// SetLifecycle configures the surface lifecycle for a tool (F286).
func (r *CapabilityRegistry) SetLifecycle(name string, lc SurfaceLifecycle) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lifecycle[name] = lc
}

// SurfaceLifecycle returns the lifecycle config for a tool (F286).
func (r *CapabilityRegistry) SurfaceLifecycle(name string) SurfaceLifecycle {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lifecycle[name]
}

// SetFamilyEnabled atomically toggles a whole resource family (F286
// AtomicSwitch). When disabled, every tool in the family is considered off the
// surface.
func (r *CapabilityRegistry) SetFamilyEnabled(family string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.familyOn[family] = enabled
}

// FamilyEnabled reports whether a resource family is currently enabled.
func (r *CapabilityRegistry) FamilyEnabled(family string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.familyOn[family]
}

// MarkUsed promotes a lazy tool into the baseline (F286 Lazy: first use).
func (r *CapabilityRegistry) MarkUsed(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if lc, ok := r.lifecycle[name]; ok {
		lc.Lazy = false
		r.lifecycle[name] = lc
	}
}

// InBaseline reports whether a tool is part of the baseline digest. A lazy tool
// that has not been MarkUsed is excluded (F286).
func (r *CapabilityRegistry) InBaseline(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if lc, ok := r.lifecycle[name]; ok {
		return !lc.Lazy
	}
	return true
}

func cloneTools(in []ToolDefinition) []ToolDefinition {
	out := make([]ToolDefinition, len(in))
	copy(out, in)
	return out
}
