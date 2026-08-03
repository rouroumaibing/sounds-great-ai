package component

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cloudwego/eino/components/embedding"
)

// EmbedConfig configures the OpenAI embedding model.
type EmbedConfig struct {
	APIKey  string
	BaseURL string
	Model   string // default text-embedding-3-small
	Dim     int    // default 1536
}

// OpenAIEmbedder generates embeddings via OpenAI /v1/embeddings API.
// Implements eino's embedding.Embedder interface.
type OpenAIEmbedder struct {
	apiKey  string
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

// Ensure OpenAIEmbedder implements embedding.Embedder.
var _ embedding.Embedder = (*OpenAIEmbedder)(nil)

func NewOpenAIEmbedder(ctx context.Context, cfg EmbedConfig) (*OpenAIEmbedder, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("embedding init: API key required")
	}
	if cfg.Model == "" {
		cfg.Model = "text-embedding-3-small"
	}
	if cfg.Dim <= 0 {
		cfg.Dim = 1536
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	return &OpenAIEmbedder{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		dim:     cfg.Dim,
		client:  &http.Client{},
	}, nil
}

// EmbedStrings calls OpenAI embedding API in batches of 64.
func (e *OpenAIEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	const batchSize = 64
	results := make([][]float64, 0, len(texts))
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		vecs, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}
		results = append(results, vecs...)
	}
	return results, nil
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (e *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	reqBody, err := json.Marshal(embeddingRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/v1/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai embeddings: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if embResp.Error != nil {
		return nil, fmt.Errorf("openai error: %s", embResp.Error.Message)
	}
	if len(embResp.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: want %d, got %d", len(texts), len(embResp.Data))
	}
	vecs := make([][]float64, len(embResp.Data))
	for i, d := range embResp.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}
