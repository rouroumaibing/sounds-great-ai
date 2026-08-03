package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"sounds-great-ai/pkg/pack/orchestrator"
)

// Type aliases — preserve existing call sites that reference these types
// by their unqualified names within this package.
type SubTask = orchestrator.SubTask
type DispatchEntry = orchestrator.DispatchEntry
type DispatchPlan = orchestrator.DispatchPlan

// MergeResult — result_merge output
type MergeResult struct {
	Summary  string         `json:"summary"`
	Sections []MergeSection `json:"sections"`
}

// MergeSection — a single breed's merged result section
type MergeSection struct {
	BreedID string `json:"breed_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// decodeData converts an any (from TaskOutput.Data) into a target typed slice/struct
// via json.Marshal → json.Unmarshal round-trip.
func decodeData(src any, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("decodeData: marshal failed: %w", err)
	}
	return json.Unmarshal(b, dst)
}

// sanitizeJSONResponse strips Markdown wrappers or extraneous text to extract pure JSON.
func sanitizeJSONResponse(s string) string {
	s = strings.TrimSpace(s)
	// If wrapped in markdown codeblocks
	if start := strings.Index(s, "```"); start >= 0 {
		if nl := strings.Index(s[start:], "\n"); nl >= 0 {
			content := s[start+nl+1:]
			if end := strings.LastIndex(content, "```"); end >= 0 {
				s = content[:end]
			}
		}
	}
	s = strings.TrimSpace(s)
	// Fallback: extract substring between first '['/'{' and last ']'/'}'
	firstArray := strings.Index(s, "[")
	firstObject := strings.Index(s, "{")
	first := -1
	if firstArray >= 0 && (firstObject < 0 || firstArray < firstObject) {
		first = firstArray
	} else if firstObject >= 0 {
		first = firstObject
	}
	lastArray := strings.LastIndex(s, "]")
	lastObject := strings.LastIndex(s, "}")
	last := -1
	if lastArray >= 0 && lastArray > lastObject {
		last = lastArray
	} else if lastObject >= 0 {
		last = lastObject
	}
	if first >= 0 && last > first {
		return s[first : last+1]
	}
	return s
}

// balancedTruncate truncates each breed's output to an equal share of the total budget.
// Uses []rune for UTF-8 safe character level truncation.
func balancedTruncate(results map[string]string, totalBudget int) map[string]string {
	n := len(results)
	if n == 0 {
		return results
	}
	perBreed := totalBudget / n
	truncated := make(map[string]string, n)
	for breed, content := range results {
		runes := []rune(content)
		if len(runes) > perBreed {
			truncated[breed] = string(runes[:perBreed]) + "\n... [truncated]"
		} else {
			truncated[breed] = content
		}
	}
	return truncated
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
