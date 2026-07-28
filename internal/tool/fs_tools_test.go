package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sounds-great-ai/internal/workspace"
)

func TestReadFileTool(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello world"), 0644)
	tool := NewReadFileTool(wm)
	input := ReadFileInput{Path: "test.txt"}
	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected 'hello world' in result, got %s", result)
	}
}

func TestReadFileToolPathTraversal(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	tool := NewReadFileTool(wm)
	input := ReadFileInput{Path: "../../../etc/passwd"}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err == nil {
		t.Error("expected path traversal to be blocked")
	}
}

func TestEditFileTool(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	tool := NewEditFileTool(wm)
	input := EditFileInput{Path: "new.go", Content: "package main"}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("EditFile failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "new.go"))
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if string(data) != "package main" {
		t.Errorf("expected 'package main', got '%s'", string(data))
	}
}

func TestListDirTool(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	os.Mkdir(filepath.Join(root, "sub"), 0755)
	os.WriteFile(filepath.Join(root, "a.go"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(root, "b.go"), []byte("b"), 0644)
	tool := NewListDirTool(wm)
	input := ListDirInput{Path: ""}
	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	if !strings.Contains(result, "a.go") || !strings.Contains(result, "b.go") {
		t.Errorf("expected listing to contain a.go and b.go, got %s", result)
	}
	if !strings.Contains(result, "sub") {
		t.Errorf("expected listing to contain sub dir, got %s", result)
	}
}
