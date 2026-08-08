package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesHandler_GetTree(t *testing.T) {
	root := t.TempDir()
	// GetTree always walks the "internal" subdirectory of rootDir.
	internalDir := filepath.Join(root, "internal", "foo")
	if err := os.MkdirAll(internalDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(internalDir, "bar.go"), []byte("package foo"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	h := NewFilesHandler(root)
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/files/tree", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var nodes []FileNode
	if err := json.NewDecoder(rec.Body).Decode(&nodes); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected non-empty file tree")
	}
	// Should contain the "foo" folder and "main.go" file.
	var foundFolder, foundFile bool
	for _, n := range nodes {
		if n.Name == "foo" && n.Type == "folder" {
			foundFolder = true
		}
		if n.Name == "main.go" && n.Type == "file" {
			foundFile = true
		}
	}
	if !foundFolder {
		t.Error("expected to find folder 'foo'")
	}
	if !foundFile {
		t.Error("expected to find file 'main.go'")
	}
}

