// internal/ragstore/indexer.go
package ragstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// chunkSize is the character count per indexing chunk.
const chunkSize = 1000

// IndexBuilder scans a directory for markdown files, chunks them, and upserts
// into a VectorStore. This is the document ingestion pipeline for RAG.
type IndexBuilder struct {
	store    VectorStore
	embedder embedding.Embedder
	rootDir  string
}

// NewIndexBuilder constructs an IndexBuilder that indexes .md files under rootDir.
func NewIndexBuilder(store VectorStore, embedder embedding.Embedder, rootDir string) *IndexBuilder {
	return &IndexBuilder{store: store, embedder: embedder, rootDir: rootDir}
}

// Rebuild walks rootDir for *.md files, parses frontmatter, chunks content,
// and upserts each chunk into the store. Files that fail are skipped (logged);
// the returned error is non-nil only if the walk itself fails.
func (b *IndexBuilder) Rebuild() error {
	errCount := 0
	fileCount := 0

	err := filepath.Walk(b.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errCount++
			log.Printf("indexer: walk error %s: %v", path, err)
			return nil // skip
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		fileCount++
		if e := b.indexFile(path); e != nil {
			errCount++
			log.Printf("indexer: skip %s: %v", path, e)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("indexer walk: %w", err)
	}

	log.Printf("indexer: processed %d files, %d errors", fileCount, errCount)
	return nil
}

// indexFile reads a markdown file, parses frontmatter, chunks content, and upserts.
func (b *IndexBuilder) indexFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	content := string(data)
	meta := parseFrontmatter(&content)
	meta["source"] = path
	meta["namespace"] = "docs"

	chunks := chunkContent(content, chunkSize)
	docs := make([]*schema.Document, 0, len(chunks))
	relPath, _ := filepath.Rel(b.rootDir, path)
	for i, chunk := range chunks {
		id := docID(relPath, i)
		docs = append(docs, &schema.Document{
			ID:       id,
			Content:  chunk,
			MetaData: copyMeta(meta),
		})
	}

	if len(docs) == 0 {
		return nil
	}
	return b.store.Upsert(context.Background(), docs)
}

// parseFrontmatter extracts title, tags, category from YAML-like frontmatter.
// Strips the frontmatter block from content if present.
// Does not use a YAML library — simple line-by-line key: value parsing.
func parseFrontmatter(content *string) map[string]any {
	meta := make(map[string]any)
	if !strings.HasPrefix(*content, "---\n") {
		return meta
	}
	end := strings.Index((*content)[4:], "\n---\n")
	if end < 0 {
		return meta
	}
	fm := (*content)[4 : 4+end]
	*content = (*content)[4+end+5:]

	for _, line := range strings.Split(fm, "\n") {
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		switch key {
		case "title":
			meta["title"] = val
		case "category":
			meta["category"] = val
		case "tags":
			meta["tags"] = strings.Split(val, ",")
		}
	}
	return meta
}

// chunkContent splits content into chunks of at most maxChars characters.
// Rune-safe: splits on rune boundaries.
func chunkContent(content string, maxChars int) []string {
	if maxChars <= 0 {
		return []string{content}
	}
	runes := []rune(content)
	var chunks []string
	for i := 0; i < len(runes); i += maxChars {
		end := i + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	if len(chunks) == 0 {
		return nil
	}
	return chunks
}

// docID generates a stable document ID from file path and chunk index.
func docID(relPath string, chunkIdx int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", relPath, chunkIdx)))
	return hex.EncodeToString(h[:16])
}

// copyMeta returns a shallow copy of the metadata map.
func copyMeta(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src)+1)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
