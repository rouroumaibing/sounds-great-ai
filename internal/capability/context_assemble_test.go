package capability

import (
	"context"
	"strings"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestContextAssemble_NameVersion(t *testing.T) {
	c := NewContextAssemble()
	if c.Name() != "context_assemble" {
		t.Fatalf("name: want context_assemble, got %s", c.Name())
	}
}

func TestContextAssemble_Run_WithMatches(t *testing.T) {
	c := NewContextAssemble()
	out, err := c.Run(context.Background(), &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"search": {
				Data: map[string]any{
					"matches": []any{
						map[string]any{
							"id":      "d1",
							"content": "hello world",
							"score":   0.95,
							"source":  "test",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !out.Approved {
		t.Fatal("want Approved=true")
	}
	context, _ := out.Data["context"].(string)
	if !strings.Contains(context, "hello world") {
		t.Fatalf("context missing content: %q", context)
	}
	if !strings.Contains(context, "0.950") {
		t.Fatalf("context missing score: %q", context)
	}
}

func TestContextAssemble_Run_EmptyPrevious(t *testing.T) {
	c := NewContextAssemble()
	out, err := c.Run(context.Background(), &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.Approved {
		t.Fatal("want Approved=false for empty")
	}
	context, _ := out.Data["context"].(string)
	if context == "" {
		t.Fatal("context should have fallback message")
	}
}

func TestContextAssemble_Run_Dedup(t *testing.T) {
	c := NewContextAssemble()
	out, _ := c.Run(context.Background(), &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"search1": {
				Data: map[string]any{
					"matches": []any{
						map[string]any{"id": "d1", "content": "a", "score": 0.9, "source": "s1"},
					},
				},
			},
			"search2": {
				Data: map[string]any{
					"matches": []any{
						map[string]any{"id": "d1", "content": "a", "score": 0.8, "source": "s1"}, // dup
						map[string]any{"id": "d2", "content": "b", "score": 0.7, "source": "s2"},
					},
				},
			},
		},
	})
	count, _ := out.Data["match_count"].(int)
	if count != 2 {
		t.Fatalf("dedup: want 2 unique, got %d", count)
	}
}

func TestContextAssemble_Run_Truncation(t *testing.T) {
	c := NewContextAssemble()
	longContent := strings.Repeat("x", 10000)
	out, _ := c.Run(context.Background(), &pack.TaskInput{
		CapabilityConfig: map[string]any{"max_chars": 100},
		Previous: map[string]*pack.TaskOutput{
			"search": {
				Data: map[string]any{
					"matches": []any{
						map[string]any{"id": "d1", "content": longContent, "score": 0.9, "source": "s"},
					},
				},
			},
		},
	})
	context, _ := out.Data["context"].(string)
	if len([]rune(context)) > 100 {
		t.Fatalf("truncation: context too long: %d runes", len([]rune(context)))
	}
	truncated, _ := out.Data["truncated"].(bool)
	if !truncated {
		t.Fatal("want truncated=true")
	}
}

func TestContextAssemble_Run_DynamicPreviousScan(t *testing.T) {
	// Verify it scans all Previous entries, not just "search" key
	c := NewContextAssemble()
	out, _ := c.Run(context.Background(), &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"custom_step_name": {
				Data: map[string]any{
					"matches": []any{
						map[string]any{"id": "d1", "content": "found", "score": 0.9, "source": "s"},
					},
				},
			},
		},
	})
	count, _ := out.Data["match_count"].(int)
	if count != 1 {
		t.Fatalf("dynamic scan: want 1, got %d", count)
	}
}
