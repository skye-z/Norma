package index

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaEmbedder(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[[0.1,0.2,0.3]]}`))
	}))
	defer ts.Close()

	embedder := NewOllamaEmbedder(ts.URL, "bge-m3:latest", 0)
	vec, err := embedder.Embed(context.Background(), "norma")
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vec) != 3 {
		t.Fatalf("expected 3 dims, got %d", len(vec))
	}
}
