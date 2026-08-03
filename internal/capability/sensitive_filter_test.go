package capability

import (
	"context"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestSensitiveFilter_Name(t *testing.T) {
	c, err := NewSensitiveFilter()
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	if c.Name() != "sensitive_filter" || c.Version() != "v1" {
		t.Fatalf("name/version: %q/%q", c.Name(), c.Version())
	}
}

func TestSensitiveFilter_KeywordBlock(t *testing.T) {
	c, _ := NewSensitiveFilter()
	input := &pack.TaskInput{
		Query: "my password is secret123",
		CapabilityConfig: map[string]any{
			"keywords": []any{"password"},
		},
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	fr, ok := out.Data["filter"].(FilterResult)
	if !ok {
		t.Fatalf("filter wrong type: %T", out.Data["filter"])
	}
	if !fr.Blocked {
		t.Fatal("expected blocked=true")
	}
	if fr.Reason != "keyword" {
		t.Fatalf("reason: %q", fr.Reason)
	}
	if !contains(fr.CleanedText, "***") {
		t.Fatalf("expected *** in cleaned text, got %q", fr.CleanedText)
	}
}

func TestSensitiveFilter_BuiltInSecretPattern(t *testing.T) {
	c, _ := NewSensitiveFilter()
	input := &pack.TaskInput{
		Query: "config: api_key=ABCDEF1234567890",
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	fr, ok := out.Data["filter"].(FilterResult)
	if !ok {
		t.Fatalf("filter wrong type: %T", out.Data["filter"])
	}
	if len(fr.FlaggedPatterns) == 0 {
		t.Fatal("expected flagged patterns (secret pattern)")
	}
}

func TestSensitiveFilter_BuiltInAWSKeyPattern(t *testing.T) {
	c, _ := NewSensitiveFilter()
	input := &pack.TaskInput{
		Query: "deployed with key AKIAIOSFODNN7EXAMPLE",
	}
	out, _ := c.Run(context.Background(), input)
	fr, _ := out.Data["filter"].(FilterResult)
	if len(fr.FlaggedPatterns) == 0 {
		t.Fatal("expected AWS key flagged")
	}
}

func TestSensitiveFilter_UserPatternCompileError(t *testing.T) {
	c, _ := NewSensitiveFilter()
	input := &pack.TaskInput{
		Query: "harmless text",
		CapabilityConfig: map[string]any{
			"patterns": []any{"(unclosed["}, // invalid regex
		},
	}
	_, err := c.Run(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestSensitiveFilter_DCL_NoDuplicateRecompile(t *testing.T) {
	// Run twice with same config — second call should hit fast path (no recompile).
	// Hard to assert directly, but verify no error + correct result.
	c, _ := NewSensitiveFilter()
	input := &pack.TaskInput{
		Query: "hello world",
		CapabilityConfig: map[string]any{
			"patterns": []any{"hello"},
		},
	}
	for i := 0; i < 2; i++ {
		out, err := c.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("run %d: err: %v", i, err)
		}
		fr, _ := out.Data["filter"].(FilterResult)
		if len(fr.FlaggedPatterns) == 0 {
			t.Fatalf("run %d: expected 'hello' flagged", i)
		}
	}
}
