package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sounds-great-ai/internal/ragstore"
)

// mockStore implements ragstore.VectorStore for testing.
type mockStore struct {
	docs []*schema.Document
	err  error
}

func (m *mockStore) Upsert(ctx context.Context, docs []*schema.Document) error { return nil }
func (m *mockStore) Search(ctx context.Context, query string, opts ragstore.SearchOpts) ([]*schema.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.docs, nil
}
func (m *mockStore) Delete(ctx context.Context, ids []string) error            { return nil }
func (m *mockStore) Close() error                                               { return nil }
func (m *mockStore) ListAll(ctx context.Context) ([]*schema.Document, error)    { return nil, nil }
func (m *mockStore) GetByID(ctx context.Context, id string) (*schema.Document, error) { return nil, nil }
func (m *mockStore) DropAll(ctx context.Context) error                          { return nil }

func TestHandleSearch_EmptyQuery(t *testing.T) {
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "search_knowledge",
			Arguments: json.RawMessage(`{"query":""}`),
		},
	}
	_, err := handleSearch(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestHandleSearch_NoResults(t *testing.T) {
	store := &mockStore{docs: nil}
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "search_knowledge",
			Arguments: json.RawMessage(`{"query":"nonexistent","top_k":5}`),
		},
	}
	result, err := handleSearch(context.Background(), req, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if tc.Text != "No relevant documents found." {
		t.Fatalf("expected 'No relevant documents found.', got %s", tc.Text)
	}
}

func TestHandleSearch_WithResults(t *testing.T) {
	store := &mockStore{
		docs: []*schema.Document{
			{ID: "doc1", Content: "RAG configuration guide"},
			{ID: "doc2", Content: "Embedding model setup"},
		},
	}
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "search_knowledge",
			Arguments: json.RawMessage(`{"query":"how to configure RAG"}`),
		},
	}
	result, err := handleSearch(context.Background(), req, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &parsed); err != nil {
		t.Fatalf("invalid JSON in result: %v", err)
	}
	results, ok := parsed["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("expected 2 results, got %v", parsed)
	}
}
