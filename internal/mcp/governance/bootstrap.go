package governance

import (
	"fmt"
	"sort"
	"strings"
)

// BootstrapAction is a single opt-in governance bootstrap action the operator
// can preview before applying. The governance surface is zero-write by default:
// nothing is mutated until Execute is called (F302: runtime-priority portable
// governance).
type BootstrapAction struct {
	Target string // e.g. "capability:sg_create_thread"
	Kind   string // "disable" | "override"
	Detail string
}

// BootstrapReport is a dry-run preview of governance bootstrap actions. Building
// a report never mutates any store (F302: zero-write).
type BootstrapReport struct{}

// NewBootstrapReport creates a zero-write bootstrap reporter.
func NewBootstrapReport() *BootstrapReport { return &BootstrapReport{} }

// Preview computes the actions Execute would apply for the given per-project
// overrides. Pure: no writes (F302).
func (r *BootstrapReport) Preview(overrides []ToolOverride) []BootstrapAction {
	acts := make([]BootstrapAction, 0, len(overrides))
	for _, o := range overrides {
		if o.Disabled {
			acts = append(acts, BootstrapAction{
				Target: "capability:" + o.Name,
				Kind:   "disable",
				Detail: "opt-in disable for project surface",
			})
		} else {
			acts = append(acts, BootstrapAction{
				Target: "capability:" + o.Name,
				Kind:   "override",
				Detail: "opt-in override for project surface",
			})
		}
	}
	sort.Slice(acts, func(i, j int) bool { return acts[i].Target < acts[j].Target })
	return acts
}

// Execute applies the overrides to the registry for a project. This is the only
// write path and is opt-in (the operator must have previewed first). Returns the
// number of overrides applied.
func (r *BootstrapReport) Execute(reg *CapabilityRegistry, project string, overrides []ToolOverride) int {
	applied := 0
	for _, o := range overrides {
		reg.ApplyOverride(project, o)
		applied++
	}
	return applied
}

// Undo reverts all bootstrap actions for a project (F302: reversible bootstrap).
func (r *BootstrapReport) Undo(reg *CapabilityRegistry, project string) {
	reg.ResetProject(project)
}

func stripCapabilityPrefix(s string) string {
	const p = "capability:"
	if strings.HasPrefix(s, p) {
		return s[len(p):]
	}
	return s
}

// FormatActions renders a preview for operator display.
func (r *BootstrapReport) FormatActions(acts []BootstrapAction) string {
	var b strings.Builder
	for _, a := range acts {
		fmt.Fprintf(&b, "- [%s] %s (%s)\n", a.Kind, a.Target, a.Detail)
	}
	return b.String()
}
