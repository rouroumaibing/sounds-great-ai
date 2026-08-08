// internal/ragstore/indexer_test.go
package ragstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestIndexBuilder_Rebuild_Basic(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0, 0.0}}
	store := NewMemoryStore(emb, "")

	// Create temp docs directory
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "doc1.md"), "# Title\n\nHello world content")
	writeFile(t, filepath.Join(root, "doc2.md"), "# Another\n\nFoo bar baz")

	indexer := NewIndexBuilder(store, emb, root)
	if err := indexer.Rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	// Verify documents were indexed
	docs, err := store.ListAll(context.Background())
	if err != nil {
		t.Fatalf("listall: %v", err)
	}
	if len(docs) < 2 {
		t.Fatalf("want >=2 docs, got %d", len(docs))
	}
}

func TestIndexBuilder_Rebuild_Frontmatter(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store := NewMemoryStore(emb, "")

	root := t.TempDir()
	content := "---\ntitle: My Doc\ncategory: guide\ntags: a, b\n---\nBody text here"
	writeFile(t, filepath.Join(root, "doc.md"), content)

	indexer := NewIndexBuilder(store, emb, root)
	if err := indexer.Rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	docs, _ := store.ListAll(context.Background())
	if len(docs) == 0 {
		t.Fatal("no docs indexed")
	}
	title, _ := docs[0].MetaData["title"].(string)
	if title != "My Doc" {
		t.Fatalf("title: want 'My Doc', got %q", title)
	}
	category, _ := docs[0].MetaData["category"].(string)
	if category != "guide" {
		t.Fatalf("category: want 'guide', got %q", category)
	}
}

func TestIndexBuilder_Rebuild_Chunking(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store := NewMemoryStore(emb, "")

	root := t.TempDir()
	// 2500 chars → 3 chunks of 1000
	longContent := string(repeatRune('x', 2500))
	writeFile(t, filepath.Join(root, "big.md"), longContent)

	indexer := NewIndexBuilder(store, emb, root)
	if err := indexer.Rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	docs, _ := store.ListAll(context.Background())
	if len(docs) != 3 {
		t.Fatalf("chunking: want 3 chunks, got %d", len(docs))
	}
}

func TestIndexBuilder_Rebuild_Subdirectories(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store := NewMemoryStore(emb, "")

	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "sub"), 0755)
	writeFile(t, filepath.Join(root, "top.md"), "top content")
	writeFile(t, filepath.Join(root, "sub", "nested.md"), "nested content")

	indexer := NewIndexBuilder(store, emb, root)
	if err := indexer.Rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	docs, _ := store.ListAll(context.Background())
	if len(docs) < 2 {
		t.Fatalf("subdirs: want >=2 docs, got %d", len(docs))
	}
}

func TestIndexBuilder_Rebuild_SkipNonMarkdown(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store := NewMemoryStore(emb, "")

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "doc.md"), "markdown")
	writeFile(t, filepath.Join(root, "data.json"), `{"key": "value"}`)
	writeFile(t, filepath.Join(root, "script.sh"), "echo hi")

	indexer := NewIndexBuilder(store, emb, root)
	if err := indexer.Rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	docs, _ := store.ListAll(context.Background())
	if len(docs) != 1 {
		t.Fatalf("skip non-md: want 1 doc, got %d", len(docs))
	}
}

func TestIndexBuilder_Rebuild_EmptyDir(t *testing.T) {
	emb := &stubEmbedder{vec: []float64{1.0}}
	store := NewMemoryStore(emb, "")
	root := t.TempDir()

	indexer := NewIndexBuilder(store, emb, root)
	if err := indexer.Rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	docs, _ := store.ListAll(context.Background())
	if len(docs) != 0 {
		t.Fatalf("empty dir: want 0 docs, got %d", len(docs))
	}
}

func TestParseFrontmatter(t *testing.T) {
	content := "---\ntitle: Test\ncategory: docs\ntags: x, y\n---\nBody"
	meta := parseFrontmatter(&content)
	if meta["title"] != "Test" {
		t.Fatalf("title: got %v", meta["title"])
	}
	if meta["category"] != "docs" {
		t.Fatalf("category: got %v", meta["category"])
	}
	if content != "Body" {
		t.Fatalf("content after strip: got %q", content)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "Just regular content"
	meta := parseFrontmatter(&content)
	if len(meta) != 0 {
		t.Fatalf("want empty meta, got %v", meta)
	}
	if content != "Just regular content" {
		t.Fatalf("content changed: %q", content)
	}
}

func TestChunkContent(t *testing.T) {
	chunks := chunkContent("abcdef", 2)
	if len(chunks) != 3 {
		t.Fatalf("want 3 chunks, got %d", len(chunks))
	}
	if chunks[0] != "ab" || chunks[1] != "cd" || chunks[2] != "ef" {
		t.Fatalf("chunks: %v", chunks)
	}
}

func TestChunkContent_RuneSafe(t *testing.T) {
	// CJK characters: each is 3 bytes but 1 rune
	content := string(repeatRune('中', 5))
	chunks := chunkContent(content, 2)
	if len(chunks) != 3 {
		t.Fatalf("rune-safe: want 3 chunks, got %d", len(chunks))
	}
}

func TestDocID_Stable(t *testing.T) {
	id1 := docID("path/to/file.md", 0)
	id2 := docID("path/to/file.md", 0)
	if id1 != id2 {
		t.Fatal("docID not stable")
	}
	id3 := docID("path/to/file.md", 1)
	if id1 == id3 {
		t.Fatal("docID collision across chunks")
	}
}

// repeatRune returns a slice of n copies of r.
func repeatRune(r rune, n int) []rune {
	result := make([]rune, n)
	for i := range result {
		result[i] = r
	}
	return result
}

// writeFile writes content to path, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// Ensure schema.Document is used (avoid unused import in edge cases)
var _ = schema.Document{}
