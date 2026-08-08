package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sounds-great-ai/internal/component"
	"sounds-great-ai/internal/ragstore"
)

func main() {
	dbPath := flag.String("db", "", "RAG SQLite database path")
	embedAPIKey := flag.String("embed-api-key", os.Getenv("MODEL_API_KEY"), "Embedding API key")
	embedBaseURL := flag.String("embed-base-url", os.Getenv("MODEL_BASE_URL"), "Embedding API base URL")
	embedModel := flag.String("embed-model", "text-embedding-3-small", "Embedding model name")
	embedDim := flag.Int("embed-dim", 1536, "Embedding dimension")
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("--db flag is required")
	}

	ctx := context.Background()

	// Initialize embedder
	embedder, err := component.NewOpenAIEmbedder(ctx, component.EmbedConfig{
		APIKey:  *embedAPIKey,
		BaseURL: *embedBaseURL,
		Model:   *embedModel,
		Dim:     *embedDim,
	})
	if err != nil {
		log.Fatalf("embedder init: %v", err)
	}

	// Open RAG SQLite store directly
	store, err := ragstore.NewStore(ragstore.StoreConfig{
		Backend:    ragstore.BackendSQLite,
		Embedder:   embedder,
		SQLitePath: *dbPath,
	})
	if err != nil {
		log.Fatalf("store init: %v", err)
	}
	defer store.Close()

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "sounds-great-rag",
		Version: "1.0.0",
	}, nil)

	// Register search_knowledge tool
	server.AddTool(&mcp.Tool{
		Name:        "search_knowledge",
		Description: "Search the knowledge base for relevant documents. Use this when you need to look up information, documentation, or context from the project's knowledge store.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query — natural language describing what you want to find.",
				},
				"top_k": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return (default 5).",
					"default":     5,
				},
			},
			"required": []string{"query"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleSearch(ctx, req, store)
	})

	// Run server over stdio
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// searchArgs is the input schema for search_knowledge tool.
type searchArgs struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k,omitempty"`
}

// handleSearch handles the search_knowledge tool call.
func handleSearch(ctx context.Context, req *mcp.CallToolRequest, store ragstore.VectorStore) (*mcp.CallToolResult, error) {
	var args searchArgs
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Query == "" {
		return nil, fmt.Errorf("query parameter is required and must be a non-empty string")
	}

	topK := args.TopK
	if topK <= 0 {
		topK = 5
	}

	docs, err := store.Search(ctx, args.Query, ragstore.SearchOpts{TopK: topK, Threshold: 0.3})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(docs) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "No relevant documents found."}},
		}, nil
	}

	// Format results as JSON
	results := make([]searchResult, 0, len(docs))
	for _, doc := range docs {
		r := searchResult{
			ID:      doc.ID,
			Content: doc.Content,
		}
		if doc.MetaData != nil {
			if source, ok := doc.MetaData["source"].(string); ok {
				r.Source = source
			}
		}
		results = append(results, r)
	}

	jsonBytes, err := json.MarshalIndent(map[string]any{"results": results}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(jsonBytes)}},
	}, nil
}

type searchResult struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Source  string `json:"source,omitempty"`
}
