package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/pack/orchestrator"
)

// Type aliases — preserve existing call sites that reference these types
// by their unqualified names within this package.
type SubTask = orchestrator.SubTask
type DispatchEntry = orchestrator.DispatchEntry
type DispatchPlan = orchestrator.DispatchPlan

// decodeData converts an any (from TaskOutput.Data) into a target typed slice/struct
// via json.Marshal → json.Unmarshal round-trip.
func decodeData(src any, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("decodeData: marshal failed: %w", err)
	}
	return json.Unmarshal(b, dst)
}

// truncateRunes returns s truncated to at most max runes, UTF-8 safe.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// getIntConfig extracts an int value from a config map with a default fallback.
// Handles JSON float64 → int coercion.
func getIntConfig(cfg map[string]any, key string, defaultVal int) int {
	if cfg == nil {
		return defaultVal
	}
	v, ok := cfg[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case int64:
		return int(val)
	default:
		return defaultVal
	}
}

// dedupKey returns SHA256 hash of breed+title+description for dedup.
func dedupKey(st SubTask) string {
	h := sha256.Sum256([]byte(st.SuggestBreed + st.Title + st.Description))
	return hex.EncodeToString(h[:])
}

// maxPreviousContextLen is the maximum characters of previous-step context to include.
const maxPreviousContextLen = 12000

// formatPreviousOutputs serializes previous step outputs into a readable context string.
func formatPreviousOutputs(prev map[string]*pack.TaskOutput) string {
	var sb strings.Builder
	for id, out := range prev {
		fmt.Fprintf(&sb, "## Step: %s\n", id)
		if out != nil {
			fmt.Fprintf(&sb, "Reason: %s\n", out.Reason)
			if len(out.Data) > 0 {
				b, _ := json.MarshalIndent(out.Data, "", "  ")
				fmt.Fprintf(&sb, "```json\n%s\n```\n", string(b))
			} else if len(out.Results) > 0 {
				b, _ := json.MarshalIndent(out.Results, "", "  ")
				fmt.Fprintf(&sb, "```json\n%s\n```\n", string(b))
			}
		}
	}
	if sb.Len() > maxPreviousContextLen {
		return sb.String()[:maxPreviousContextLen] + "... [Truncated due to context limit]"
	}
	return sb.String()
}

// defaultAvailableBreeds is the fallback list of breed IDs.
const defaultAvailableBreeds = "bianmu,xigou,jinmao,demu,zangao,zhonghuatianyuanquan"

// getAvailableBreeds resolves the list of available breed IDs from config or env.
func getAvailableBreeds(input *pack.TaskInput) map[string]bool {
	// 1. Try CapabilityConfig
	if input.CapabilityConfig != nil {
		if v, ok := input.CapabilityConfig["available_breeds"]; ok {
			if breeds, ok := v.([]any); ok {
				m := make(map[string]bool, len(breeds))
				for _, b := range breeds {
					if s, ok := b.(string); ok {
						m[s] = true
					}
				}
				if len(m) > 0 {
					return m
				}
			}
		}
	}
	// 2. Try env var
	if envBreeds := os.Getenv("AVAILABLE_BREEDS"); envBreeds != "" {
		m := make(map[string]bool)
		for _, b := range strings.Split(envBreeds, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				m[b] = true
			}
		}
		if len(m) > 0 {
			return m
		}
	}
	// 3. Default
	m := make(map[string]bool)
	for _, b := range strings.Split(defaultAvailableBreeds, ",") {
		m[b] = true
	}
	return m
}
