package transport

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileNode represents a file or folder in the project tree.
type FileNode struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Type     string     `json:"type"` // "folder" | "file"
	Path     string     `json:"path,omitempty"`
	Expanded bool       `json:"expanded,omitempty"`
	Children []FileNode `json:"children,omitempty"`
}

// FilesHandler serves the project file tree.
type FilesHandler struct {
	rootDir string
}

func NewFilesHandler(rootDir string) *FilesHandler {
	return &FilesHandler{rootDir: rootDir}
}

func (h *FilesHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/files/tree", h.GetTree)
	return mux
}

func (h *FilesHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	tree := h.buildTree(h.rootDir, "internal", true)
	respondJSON(w, http.StatusOK, tree)
}

// buildTree walks a directory (maxDepth 4) and builds a FileNode tree.
func (h *FilesHandler) buildTree(base, rel string, expanded bool) []FileNode {
	full := filepath.Join(base, rel)
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir() // dirs first
		}
		return entries[i].Name() < entries[j].Name()
	})

	var nodes []FileNode
	for _, e := range entries {
		name := e.Name()
		// skip hidden / vendor / node_modules
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			continue
		}
		id := "f-" + filepath.Join(rel, name)
		if e.IsDir() {
			childRel := filepath.Join(rel, name)
			children := h.buildTree(base, childRel, false)
			// only include dirs that have children or are not empty
			nodes = append(nodes, FileNode{
				ID:       id,
				Name:     name,
				Type:     "folder",
				Expanded: false,
				Children: children,
			})
		} else {
			nodes = append(nodes, FileNode{
				ID:   id,
				Name: name,
				Type: "file",
				Path: filepath.Join(rel, name),
			})
		}
	}
	return nodes
}
