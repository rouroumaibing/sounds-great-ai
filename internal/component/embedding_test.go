package component

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIEmbedder_EmbedStrings_Success(t *testing.T) {
	// Mock OpenAI /v1/embeddings endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req["model"] != "text-embedding-3-small" {
			t.Fatalf("model: want text-embedding-3-small, got %v", req["model"])
		}
		input, ok := req["input"].([]any)
		if !ok || len(input) != 2 {
			t.Fatalf("input: want 2 texts, got %v", req["input"])
		}
		// Return 2 fake embeddings (dim=3 for test simplicity)
		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": []float64{0.1, 0.2, 0.3}},
				{"embedding": []float64{0.4, 0.5, 0.6}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	emb, err := NewOpenAIEmbedder(context.Background(), EmbedConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "text-embedding-3-small",
		Dim:     3,
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	vecs, err := emb.EmbedStrings(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("want 2 vectors, got %d", len(vecs))
	}
	if len(vecs[0]) != 3 {
		t.Fatalf("dim: want 3, got %d", len(vecs[0]))
	}
}

func TestOpenAIEmbedder_EmbedStrings_BatchChunking(t *testing.T) {
	// Verify that >64 inputs are split into multiple requests
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		input, _ := req["input"].([]any)
		// Each response has matching number of embeddings
		data := make([]map[string]any, len(input))
		for i := range input {
			data[i] = map[string]any{"embedding": []float64{0.1, 0.2, 0.3}}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()

	emb, _ := NewOpenAIEmbedder(context.Background(), EmbedConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "text-embedding-3-small",
		Dim:     3,
	})

	// 100 texts → 2 batches (64 + 36)
	_, err := emb.EmbedStrings(context.Background(), make([]string, 100))
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("want 2 API calls (64+36), got %d", callCount)
	}
}

func TestOpenAIEmbedder_EmbedStrings_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	emb, _ := NewOpenAIEmbedder(context.Background(), EmbedConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "text-embedding-3-small",
		Dim:     3,
	})

	_, err := emb.EmbedStrings(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("want error on 500, got nil")
	}
}
