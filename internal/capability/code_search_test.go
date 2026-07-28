package capability

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestCodeSearchBasic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte("package foo\nfunc Foo() { Bark() }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bar.go"), []byte("package bar\nfunc Bar() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCodeSearch(dir)
	out, err := c.Run(context.Background(), &pack.TaskInput{Query: "Bark"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	count, ok := out.Data["count"].(int)
	if !ok {
		t.Fatalf("count is not int: %T", out.Data["count"])
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	matches, ok := out.Data["matches"].([]SearchMatch)
	if !ok {
		t.Fatalf("matches is not []SearchMatch: %T", out.Data["matches"])
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(matches))
	}
	if matches[0].File != "foo.go" {
		t.Errorf("File = %q, want %q", matches[0].File, "foo.go")
	}
	if matches[0].Line != 2 {
		t.Errorf("Line = %d, want 2", matches[0].Line)
	}
}

func TestCodeSearchEmptyPattern(t *testing.T) {
	dir := t.TempDir()
	c := NewCodeSearch(dir)
	_, err := c.Run(context.Background(), &pack.TaskInput{})
	if err == nil {
		t.Error("expected error for empty pattern, got nil")
	}
}

func TestCodeSearchSkipDirs(t *testing.T) {
	dir := t.TempDir()

	// Create a .git directory with a .go file containing the pattern.
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config.go"), []byte("Bark\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a regular file with the same pattern.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("Bark\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCodeSearch(dir)
	out, err := c.Run(context.Background(), &pack.TaskInput{Query: "Bark"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	count := out.Data["count"].(int)
	if count != 1 {
		t.Errorf("count = %d, want 1 (.git should be skipped)", count)
	}

	matches := out.Data["matches"].([]SearchMatch)
	if matches[0].File != "main.go" {
		t.Errorf("File = %q, want %q", matches[0].File, "main.go")
	}
}

func TestCodeSearchIgnoreCase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte("BARK\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCodeSearch(dir)
	out, err := c.Run(context.Background(), &pack.TaskInput{
		Query: "bark",
		CapabilityConfig: map[string]any{
			"ignore_case": true,
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	count := out.Data["count"].(int)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestCodeSearchIgnoreCaseAsString(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte("BARK\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCodeSearch(dir)
	out, err := c.Run(context.Background(), &pack.TaskInput{
		Query: "bark",
		CapabilityConfig: map[string]any{
			"ignore_case": "true",
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	count := out.Data["count"].(int)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestCodeSearchLongLine(t *testing.T) {
	dir := t.TempDir()

	// Build a line exceeding 100KB with the pattern embedded.
	longLine := strings.Repeat("a", 200000) + "needle"
	content := "package main\n" + longLine + "\n"
	if err := os.WriteFile(filepath.Join(dir, "big.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCodeSearch(dir)
	out, err := c.Run(context.Background(), &pack.TaskInput{Query: "needle"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	count := out.Data["count"].(int)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	matches := out.Data["matches"].([]SearchMatch)
	if !strings.Contains(matches[0].Content, "needle") {
		t.Error("match content does not contain 'needle'")
	}
}

func TestCodeSearchCrossPlatformPath(t *testing.T) {
	dir := t.TempDir()

	// Create a nested directory structure.
	nestedDir := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "file.go"), []byte("target\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCodeSearch(dir)
	out, err := c.Run(context.Background(), &pack.TaskInput{Query: "target"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	count := out.Data["count"].(int)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	matches := out.Data["matches"].([]SearchMatch)
	wantPath := "sub/deep/file.go"
	if matches[0].File != wantPath {
		t.Errorf("File = %q, want %q", matches[0].File, wantPath)
	}
	if strings.Contains(matches[0].File, "\\") {
		t.Errorf("File path contains backslash: %q", matches[0].File)
	}
}
