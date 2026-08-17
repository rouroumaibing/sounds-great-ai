package a2aadapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"
	a2aprotocol "sounds-great-ai/pkg/a2a"
	"sounds-great-ai/internal/adapter/unified"
)

func TestAdapter_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %q", r.Header.Get("Content-Type"))
		}
		var req a2aprotocol.JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method != a2aprotocol.MethodTasksSend {
			t.Errorf("expected method %q, got %q", a2aprotocol.MethodTasksSend, req.Method)
		}
		resp := a2aprotocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: &a2aprotocol.Task{
				ID:     "task-1",
				Status: a2aprotocol.TaskStatusCompleted,
				Artifacts: []a2aprotocol.Artifact{
					{Name: "out", Parts: []a2aprotocol.Part{{Type: "text", Text: "hello from remote"}}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	adapter := New(nil, "a2a")
	adapter.SetEndpoint("a2a", srv.URL, "")

	ch, err := adapter.Execute(context.Background(), unified.ExecuteRequest{
		ClientID: "a2a",
		Messages: []*schema.Message{{Role: schema.User, Content: "ping"}},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var gotText, gotDone bool
	for ev := range ch {
		if ev.Type == "text" {
			gotText = gotText || ev.Content == "hello from remote"
		}
		if ev.Type == "done" {
			gotDone = true
		}
	}
	if !gotText {
		t.Error("expected text event 'hello from remote'")
	}
	if !gotDone {
		t.Error("expected terminal done event")
	}
}

func TestAdapter_Execute_NoEndpoint(t *testing.T) {
	adapter := New(nil, "a2a")
	if _, err := adapter.Execute(context.Background(), unified.ExecuteRequest{ClientID: "a2a"}); err == nil {
		t.Error("expected error when no endpoint configured")
	}
}

func TestAdapter_Health(t *testing.T) {
	adapter := New(nil, "a2a")
	if err := adapter.Health(context.Background()); err == nil {
		t.Error("expected health error when endpoint missing")
	}
	adapter.SetEndpoint("a2a", "https://example.com/a2a", "")
	if err := adapter.Health(context.Background()); err != nil {
		t.Errorf("expected healthy, got %v", err)
	}
}

// Ensure the adapter's buildPrompt handles nil/empty gracefully (smoke).
func TestBuildPrompt_Empty(t *testing.T) {
	if p := buildPrompt(unified.ExecuteRequest{}); p != "(empty request)" {
		t.Errorf("expected empty marker, got %q", p)
	}
}
