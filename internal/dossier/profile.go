// Package dossier implements the dog capability dossier (FT-DS-001), the
// single source of truth for capability profiles — mirroring clowder-ai F208.
//
// The dossier lives at docs/team/dog-dossier.md (git-versioned). Each dog gets
// one section anchored by a `### … dog:{dogId}` header plus a fenced yaml
// block whose first line is `# structured-profile: dog:{dogId}`. The yaml
// block is the machine-readable projection consumed by the prompt builder and
// the dossier API; the prose around it carries the full six-field profile
// (原生峰值 / 被低估能力 / 坏直觉 / 召唤反信号 / 互补&反模式 / 翻车熔断信号).
//
// Constitutional constraints (clowder eval-philosophy E1/E5, adopted):
//   - No personality scores. The summary layer is peer/operator judged with
//     provenance, never algorithm-generated.
//   - Updates flow only through the distillation proposal state machine;
//     every change carries evidence references.
package dossier

import (
	"regexp"
	"strings"
)

// DossierProfile is the machine-readable projection of one dog's dossier
// section (the structured yaml block).
type DossierProfile struct {
	// DogID is the dog identity key (breed-level dog_id or variant dog_id),
	// e.g. "bianmu", "bianmu-sonnet".
	DogID string `json:"dogId"`
	// OneLiner is the single-sentence portrait used in the identity block.
	OneLiner string `json:"oneLiner,omitempty"`
	// L0RosterSummary is the compact strengths cell for the teammate roster
	// (keep ≤ 52 runes to fit the roster table).
	L0RosterSummary string `json:"l0RosterSummary,omitempty"`
	// L0RoutingNote is the compact route-critical boundary for the roster
	// (what NOT to route to this dog).
	L0RoutingNote string `json:"l0RoutingNote,omitempty"`
	// RoutingSignals carries peak capabilities (route toward) and anti-signals
	// (route away).
	RoutingSignals RoutingSignals `json:"routingSignals,omitempty"`
	// Provenance records where this profile came from and when.
	Provenance Provenance `json:"provenance,omitempty"`
}

// RoutingSignals are the route-toward / route-away lists.
type RoutingSignals struct {
	PeakCapabilities []string `json:"peakCapabilities,omitempty"`
	AntiSignals      []string `json:"antiSignals,omitempty"`
}

// Provenance anchors every summary claim to its source and date.
type Provenance struct {
	Version        string   `json:"version,omitempty"`
	Date           string   `json:"date,omitempty"`
	PrimarySources []string `json:"primarySources,omitempty"`
}

// yamlBlockPattern matches fenced ```yaml blocks.
var yamlBlockPattern = regexp.MustCompile("(?s)```yaml\n(.*?)```")

// structuredProfileMarker matches the first-line marker inside a yaml block.
var structuredProfileMarker = regexp.MustCompile("(?m)^# structured-profile:\\s*dog:(\\S+)\\s*$")

// ParseDossierProfiles extracts structured-profile yaml blocks from dossier
// markdown content, keyed by dogId. Blocks without the marker are ignored
// (they may be generic yaml snippets in the prose).
func ParseDossierProfiles(markdownContent string) map[string]DossierProfile {
	profiles := make(map[string]DossierProfile)
	if markdownContent == "" {
		return profiles
	}

	for _, match := range yamlBlockPattern.FindAllStringSubmatch(markdownContent, -1) {
		block := strings.TrimSpace(match[1])
		marker := structuredProfileMarker.FindStringSubmatch(block)
		if marker == nil {
			continue
		}
		dogID := strings.TrimSpace(marker[1])
		if dogID == "" {
			continue
		}
		if profile, ok := parseYamlBlock(block, dogID); ok {
			profiles[dogID] = profile
		}
	}
	return profiles
}

// parseYamlBlock parses one structured-profile block into a DossierProfile.
// The format is intentionally narrow (flat quoted string fields + nested
// list fields), so a purpose-built parser suffices — no yaml dependency.
func parseYamlBlock(block, dogID string) (DossierProfile, bool) {
	profile := DossierProfile{DogID: dogID}

	if v, ok := extractString(block, "entityId"); ok && v != "" {
		// entityId, when present, must agree with the marker — a mismatch
		// means a copy-paste error; fail the block rather than index it
		// under the wrong dog.
		if v != dogID {
			return DossierProfile{}, false
		}
	}

	profile.OneLiner, _ = extractString(block, "oneLiner")
	profile.L0RosterSummary, _ = extractString(block, "l0RosterSummary")
	profile.L0RoutingNote, _ = extractString(block, "l0RoutingNote")
	profile.RoutingSignals = RoutingSignals{
		PeakCapabilities: extractList(block, "peakCapabilities"),
		AntiSignals:      extractList(block, "antiSignals"),
	}
	profile.Provenance = parseProvenance(block)

	return profile, true
}

// stringFieldPattern extracts `field: "value"` with an optional trailing
// YAML comment. Leading whitespace allows nested placement.
func stringFieldPattern(field string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*` + field + `:\s*"([^"]*)"\s*(?:#.*)?$`)
}

func extractString(block, field string) (string, bool) {
	m := stringFieldPattern(field).FindStringSubmatch(block)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// extractList extracts a list field either inline `["a", "b"]` or as
// multi-line `  - "item"` entries following the field line.
func extractList(block, field string) []string {
	lines := strings.Split(block, "\n")
	fieldRe := regexp.MustCompile(`^(\s*)` + field + `:\s*(.*)$`)

	for i, line := range lines {
		m := fieldRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent := len(m[1])
		rest := strings.TrimSpace(m[2])

		// Inline form: ["a", "b"]
		if strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") {
			inner := strings.TrimSpace(rest[1 : len(rest)-1])
			if inner == "" {
				return nil
			}
			items := make([]string, 0)
			for _, part := range strings.Split(inner, ",") {
				item := strings.Trim(strings.TrimSpace(part), `"`)
				if item != "" {
					items = append(items, item)
				}
			}
			return items
		}

		// Multi-line form: subsequent lines more indented, each `- "item"`.
		itemRe := regexp.MustCompile(`^\s+-\s+"(.*)"$`)
		items := make([]string, 0)
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimLeft(lines[j], " \t")
			if trimmed == "" {
				continue
			}
			lineIndent := len(lines[j]) - len(trimmed)
			if lineIndent <= indent {
				break
			}
			im := itemRe.FindStringSubmatch(lines[j])
			if im != nil && im[1] != "" {
				items = append(items, im[1])
			}
		}
		return items
	}
	return nil
}

func parseProvenance(block string) Provenance {
	p := Provenance{}
	if v, ok := extractString(block, "version"); ok {
		p.Version = v
	}
	if v, ok := extractString(block, "date"); ok {
		p.Date = v
	}
	// primarySources lives under the provenance: nesting; scope extraction
	// to that sub-block to avoid picking up same-named fields elsewhere.
	if body, ok := extractObjectBlock(block, "provenance"); ok {
		p.PrimarySources = extractList(body, "primarySources")
	}
	return p
}

// extractObjectBlock returns the indented body following a `field:` line
// (the field line itself has no value), bounding at the next line with
// indent ≤ the field's own indent.
func extractObjectBlock(block, field string) (string, bool) {
	lines := strings.Split(block, "\n")
	fieldRe := regexp.MustCompile(`^(\s*)` + field + `:\s*(?:#.*)?$`)

	for i, line := range lines {
		m := fieldRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent := len(m[1])
		var body []string
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimLeft(lines[j], " \t")
			if trimmed == "" {
				continue
			}
			lineIndent := len(lines[j]) - len(trimmed)
			if lineIndent <= indent {
				break
			}
			body = append(body, lines[j])
		}
		if len(body) > 0 {
			return strings.Join(body, "\n"), true
		}
		return "", false
	}
	return "", false
}
