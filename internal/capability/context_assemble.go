package capability

import (
	"context"
	"fmt"
	"strings"

	"sounds-great-ai/pkg/pack"
)

// ContextAssemble is the context_assemble capability adapter.
// It collects matches from all Previous outputs, deduplicates by document ID,
// and assembles a context string for downstream LLM consumption.
type ContextAssemble struct{}

func NewContextAssemble() *ContextAssemble {
	return &ContextAssemble{}
}

func (c *ContextAssemble) Name() string                   { return "context_assemble" }
func (c *ContextAssemble) Version() string                { return "v1" }
func (c *ContextAssemble) Init(ctx context.Context) error { return nil }
func (c *ContextAssemble) Health() error                  { return nil }
func (c *ContextAssemble) Close() error                   { return nil }

func (c *ContextAssemble) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	// 1. Dynamically scan all Previous entries for "matches" field
	//    (don't hardcode stepID="search")
	var allMatches []map[string]any
	for _, prevOut := range input.Previous {
		if prevOut == nil || prevOut.Data == nil {
			continue
		}
		matches, ok := prevOut.Data["matches"].([]any)
		if !ok {
			continue
		}
		for _, m := range matches {
			if mm, ok := m.(map[string]any); ok {
				allMatches = append(allMatches, mm)
			}
		}
	}

	// 2. Dedup by document ID
	seen := make(map[string]bool)
	var unique []map[string]any
	for _, m := range allMatches {
		id, _ := m["id"].(string)
		if id != "" && seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, m)
	}

	// 3. Assemble context text: [rank] source\ncontent\n(score: X)
	var sb strings.Builder
	for i, m := range unique {
		content, _ := m["content"].(string)
		source, _ := m["source"].(string)
		score, _ := m["score"].(float64)
		sb.WriteString(fmt.Sprintf("[%d] %s\n%s\n(score: %.3f)\n\n",
			i+1, source, content, score))
	}

	// 4. Rune-safe truncation
	maxChars := getIntConfig(input.CapabilityConfig, "max_chars", 8000)
	context := truncateRunes(sb.String(), maxChars)

	// 5. Empty fallback message
	if context == "" {
		context = "未找到相关参考文档。"
	}

	return &pack.TaskOutput{
		Approved: len(unique) > 0,
		Reason:   fmt.Sprintf("assembled %d unique contexts", len(unique)),
		Data: map[string]any{
			"context":     context,
			"match_count": len(unique),
			"truncated":   len([]rune(sb.String())) > maxChars,
		},
	}, nil
}
