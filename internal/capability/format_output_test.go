package capability

import (
	"context"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestFormatOutput_Name(t *testing.T) {
	c := NewFormatOutput()
	if c.Name() != "format_output" || c.Version() != "v1" {
		t.Fatalf("name/version: %q/%q", c.Name(), c.Version())
	}
}

func TestFormatOutput_Run_FormatsPrevious(t *testing.T) {
	c := NewFormatOutput()
	input := &pack.TaskInput{
		Query: "summary",
		Previous: map[string]*pack.TaskOutput{
			"step1": {Data: map[string]any{"key": "value"}},
			"step2": {Data: map[string]any{"numbers": []int{1, 2, 3}}},
		},
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	fo, ok := out.Data["formatted"].(FormattedOutput)
	if !ok {
		t.Fatalf("formatted wrong type: %T", out.Data["formatted"])
	}
	if len(fo.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(fo.Sections))
	}
	if _, ok := fo.Sections["step1"]; !ok {
		t.Fatal("missing step1 section")
	}
	if fo.Combined == "" {
		t.Fatal("combined text empty")
	}
}

func TestFormatOutput_Run_EmptyPrevious(t *testing.T) {
	c := NewFormatOutput()
	input := &pack.TaskInput{Query: "x"}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	fo, ok := out.Data["formatted"].(FormattedOutput)
	if !ok {
		t.Fatalf("formatted wrong type: %T", out.Data["formatted"])
	}
	if len(fo.Sections) != 0 {
		t.Fatalf("expected 0 sections, got %d", len(fo.Sections))
	}
}
