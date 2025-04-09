package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Embedder converts text into vector embedding.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// HashEmbedder is a local deterministic fallback embedder.
type HashEmbedder struct{}

func (h HashEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	return embedF32(text), nil
}

// HTTPEmbedder calls an embedding HTTP service.
type HTTPEmbedder struct {
	Endpoint string
	Client   *http.Client
}

type embedReq struct {
	Text string `json:"text"`
}

type embedResp struct {
	Vector []float32 `json:"vector"`
}

func NewHTTPEmbedder(endpoint string, timeout time.Duration) *HTTPEmbedder {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &HTTPEmbedder{
		Endpoint: strings.TrimRight(endpoint, "/"),
		Client:   &http.Client{Timeout: timeout},
	}
}

func (e *HTTPEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if strings.TrimSpace(e.Endpoint) == "" {
		return nil, fmt.Errorf("embed endpoint empty")
	}
	b, _ := json.Marshal(embedReq{Text: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Endpoint+"/embed", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embed status=%d", resp.StatusCode)
	}

	var out embedResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Vector) == 0 {
		return nil, fmt.Errorf("empty vector")
	}
	return out.Vector, nil
}

// OllamaEmbedder calls Ollama's /api/embed endpoint.
type OllamaEmbedder struct {
	Endpoint string
	Model    string
	Client   *http.Client
}

type ollamaEmbedReq struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Keep  string `json:"keep_alive,omitempty"`
}

type ollamaEmbedResp struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func NewOllamaEmbedder(endpoint, model string, timeout time.Duration) *OllamaEmbedder {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434"
	}
	return &OllamaEmbedder{
		Endpoint: strings.TrimRight(endpoint, "/"),
		Model:    strings.TrimSpace(model),
		Client:   &http.Client{Timeout: timeout},
	}
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.Model == "" {
		return nil, fmt.Errorf("ollama embed model empty")
	}
	body, _ := json.Marshal(ollamaEmbedReq{Model: e.Model, Input: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Endpoint+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embed status=%d", resp.StatusCode)
	}

	var out ollamaEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Embeddings) == 0 || len(out.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("ollama embed returned empty vector")
	}
	return out.Embeddings[0], nil
}
