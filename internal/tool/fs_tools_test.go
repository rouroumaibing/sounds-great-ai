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

func TestReadFileToolNonexistentFile(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	tool := NewReadFileTool(wm)
	input := ReadFileInput{Path: "nonexistent.txt"}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadFileToolEmptyPath(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	tool := NewReadFileTool(wm)
	input := ReadFileInput{Path: ""}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err == nil {
		t.Error("expected error when reading directory as file")
	}
}

func TestReadFileToolDirectoryPath(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	os.Mkdir(filepath.Join(root, "subdir"), 0755)
	tool := NewReadFileTool(wm)
	input := ReadFileInput{Path: "subdir"}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err == nil {
		t.Error("expected error when reading directory as file")
	}
}

func TestReadFileToolTruncation(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	largeContent := strings.Repeat("a", maxFileSize+100)
	os.WriteFile(filepath.Join(root, "large.txt"), []byte(largeContent), 0644)
	tool := NewReadFileTool(wm)
	input := ReadFileInput{Path: "large.txt"}
	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(result, "[truncated]") {
		t.Error("expected truncation marker in result")
	}
}

func TestEditFileToolPathTraversal(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	tool := NewEditFileTool(wm)
	input := EditFileInput{Path: "../../../etc/passwd", Content: "malicious"}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err == nil {
		t.Error("expected path traversal to be blocked")
	}
}

func TestEditFileToolNestedDirectoryCreation(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	tool := NewEditFileTool(wm)
	input := EditFileInput{Path: "a/b/c/file.go", Content: "package c"}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("EditFile with nested dirs failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "a/b/c/file.go"))
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if string(data) != "package c" {
		t.Errorf("expected 'package c', got '%s'", string(data))
	}
}

func TestEditFileToolEmptyContent(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	tool := NewEditFileTool(wm)
	input := EditFileInput{Path: "empty.txt", Content: ""}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("EditFile with empty content failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "empty.txt"))
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestListDirToolNonexistentDir(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	tool := NewListDirTool(wm)
	input := ListDirInput{Path: "nonexistent"}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestListDirToolEmptyDir(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	os.Mkdir(filepath.Join(root, "emptydir"), 0755)
	tool := NewListDirTool(wm)
	input := ListDirInput{Path: "emptydir"}
	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("ListDir on empty dir failed: %v", err)
	}
	if strings.Contains(result, "file:") || strings.Contains(result, "dir:") {
		t.Errorf("expected no entries in empty dir, got %q", result)
	}
}

func TestListDirToolPathTraversal(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	tool := NewListDirTool(wm)
	input := ListDirInput{Path: "../../../etc"}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err == nil {
		t.Error("expected path traversal to be blocked")
	}
}

func TestListDirToolFilePath(t *testing.T) {
	root := t.TempDir()
	wm := workspace.NewWorkspaceManager(root)
	os.WriteFile(filepath.Join(root, "afile.txt"), []byte("content"), 0644)
	tool := NewListDirTool(wm)
	input := ListDirInput{Path: "afile.txt"}
	inputJSON, _ := json.Marshal(input)
	_, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err == nil {
		t.Error("expected error when listing a file as directory")
	}
}
