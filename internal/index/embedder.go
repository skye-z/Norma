package index

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
