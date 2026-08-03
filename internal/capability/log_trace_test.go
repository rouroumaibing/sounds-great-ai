package capability

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestLogTrace_Name(t *testing.T) {
	c := NewLogTrace()
	if c.Name() != "log_trace" || c.Version() != "v1" {
		t.Fatalf("name/version: %q/%q", c.Name(), c.Version())
	}
}

func TestLogTrace_Run_NoPath(t *testing.T) {
	c := NewLogTrace()
	input := &pack.TaskInput{Query: "panic"}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	tr, ok := out.Data["trace"].(TraceResult)
	if !ok {
		t.Fatalf("trace wrong type: %T", out.Data["trace"])
	}
	if len(tr.Matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(tr.Matches))
	}
}

func TestLogTrace_Run_FiltersByQuery(t *testing.T) {
	// Create temp log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "app.log")
	content := "INFO starting\nERROR panic: nil pointer at foo.go:42\nINFO done\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	c := NewLogTrace()
	input := &pack.TaskInput{
		Query: "panic",
		Path:  logPath,
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	tr, ok := out.Data["trace"].(TraceResult)
	if !ok {
		t.Fatalf("trace wrong type: %T", out.Data["trace"])
	}
	if len(tr.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(tr.Matches))
	}
	if tr.Matches[0].Line != 2 {
		t.Fatalf("expected line 2, got %d", tr.Matches[0].Line)
	}
	if tr.Matches[0].Level != "ERROR" {
		t.Fatalf("expected ERROR level, got %q", tr.Matches[0].Level)
	}
}

func TestLogTrace_Run_PathMissing(t *testing.T) {
	c := NewLogTrace()
	input := &pack.TaskInput{
		Query: "panic",
		Path:  "/nonexistent/path.log",
	}
	out, err := c.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("err should be nil on missing path: %v", err)
	}
	tr, ok := out.Data["trace"].(TraceResult)
	if !ok {
		t.Fatalf("trace wrong type: %T", out.Data["trace"])
	}
	if len(tr.Matches) != 0 {
		t.Fatalf("expected 0 matches on missing path, got %d", len(tr.Matches))
	}
}
