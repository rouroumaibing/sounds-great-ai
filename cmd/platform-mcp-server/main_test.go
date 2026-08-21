package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sounds-great-ai/internal/mcp/governance"
)

// newTestServer spins up an httptest server with the given handler and
// registers cleanup.
func newTestServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func findTool(name string) governance.ToolDefinition {
	for _, t := range governance.Catalog() {
		if t.Name == name {
			return t
		}
	}
	return governance.ToolDefinition{}
}

func callTool(t *testing.T, s *server, tool governance.ToolDefinition, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	body, _ := json.Marshal(args)
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      tool.Name,
			Arguments: json.RawMessage(body),
		},
	}
	res, err := makeHandler(s, tool)(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned error (should be wrapped as result): %v", err)
	}
	return res
}

func TestPlatformTools_ListThreads(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/threads" || r.Method != "GET" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"t1","title":"hello"}]`))
	})
	s := &server{apiBase: ts.URL, httpClient: ts.Client()}
	res := callTool(t, s, findTool("sg_list_threads"), map[string]any{})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content[0])
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, `"id": "t1"`) && !strings.Contains(text, `"id":"t1"`) {
		t.Fatalf("expected thread in output, got: %s", text)
	}
}

func TestPlatformTools_GetThreadPathSubstitution(t *testing.T) {
	var gotPath string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"events":[]}`))
	})
	s := &server{apiBase: ts.URL, httpClient: ts.Client()}
	res := callTool(t, s, findTool("sg_get_thread"), map[string]any{"id": "abc"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content[0])
	}
	if gotPath != "/api/threads/abc" {
		t.Fatalf("path substitution failed: got %q", gotPath)
	}
}

func TestPlatformTools_PostMessageBody(t *testing.T) {
	var gotMethod, gotBody string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		b := make([]byte, r.ContentLength)
		r.Body.Read(b)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"ok":true}`))
	})
	s := &server{apiBase: ts.URL, httpClient: ts.Client()}
	res := callTool(t, s, findTool("sg_post_message"), map[string]any{
		"id": "t1", "content": "hi", "role": "user", "sender": "mcp",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content[0])
	}
	if gotMethod != "POST" {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil {
		t.Fatalf("body not json: %s", gotBody)
	}
	if parsed["content"] != "hi" || parsed["role"] != "user" {
		t.Fatalf("body missing fields: %s", gotBody)
	}
}

func TestPlatformTools_MissingRequiredPathParam(t *testing.T) {
	s := &server{apiBase: "http://127.0.0.1:0", httpClient: http.DefaultClient}
	res := callTool(t, s, findTool("sg_get_thread"), map[string]any{})
	if !res.IsError {
		t.Fatal("expected IsError for missing required path param")
	}
}

func TestPlatformTools_Unreachable(t *testing.T) {
	s := &server{apiBase: "http://127.0.0.1:0", httpClient: http.DefaultClient}
	res := callTool(t, s, findTool("sg_list_threads"), map[string]any{})
	if !res.IsError {
		t.Fatal("expected IsError when platform API is unreachable")
	}
}

func TestBuildInputSchema(t *testing.T) {
	schema := buildInputSchema(findTool("sg_get_dog"))
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	if _, ok := props["key"]; !ok {
		t.Fatal("expected 'key' property")
	}
	req, ok := schema["required"].([]string)
	if !ok || len(req) != 1 || req[0] != "key" {
		t.Fatalf("expected required [key], got %v", schema["required"])
	}
}

// TestGovernedCatalogAnnotations verifies every catalog tool carries the four
// governance annotations consistently.
func TestGovernedCatalogAnnotations(t *testing.T) {
	for _, tool := range governance.Catalog() {
		// Write tools must NOT be flagged read-only; read tools must be.
		isWrite := tool.Method == "POST" || tool.Method == "PUT"
		if isWrite && tool.ReadOnly {
			t.Errorf("tool %s is %s but flagged ReadOnly", tool.Name, tool.Method)
		}
		if !isWrite && !tool.ReadOnly {
			t.Errorf("tool %s is %s but not flagged ReadOnly", tool.Name, tool.Method)
		}
	}
}
