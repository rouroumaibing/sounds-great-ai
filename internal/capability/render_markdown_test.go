package capability

import (
	"context"
	"strings"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestRenderMarkdown_Name(t *testing.T) {
	c := NewRenderMarkdown()
	if c.Name() != "render_markdown" || c.Version() != "v1" {
		t.Fatalf("name/version: %q/%q", c.Name(), c.Version())
	}
}

func TestRenderMarkdown_Run_FromFormatted(t *testing.T) {
	c := NewRenderMarkdown()
	input := &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"format": {
				Data: map[string]any{
					"formatted": FormattedOutput{
						Sections: map[string]string{
							"step1": "content one",
							"step2": "content two",
						},
						Combined: "",
					},
				},
			},
		},
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	md, ok := out.Data["markdown"].(string)
	if !ok {
		t.Fatalf("markdown wrong type: %T", out.Data["markdown"])
	}
	if !strings.Contains(md, "## step1") {
		t.Fatalf("expected '## step1' heading, got: %q", md)
	}
	if !strings.Contains(md, "content one") {
		t.Fatalf("expected 'content one' in markdown, got: %q", md)
	}
}

func TestRenderMarkdown_Run_MissingFormat(t *testing.T) {
	c := NewRenderMarkdown()
	input := &pack.TaskInput{Previous: map[string]*pack.TaskOutput{}}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	md, ok := out.Data["markdown"].(string)
	if !ok || md == "" {
		t.Fatalf("expected non-empty markdown, got: %v", out.Data["markdown"])
	}
}
