package index

import (
	"context"
	"testing"
)

type tunedVectorStub struct {
	topK int
	ef   int
	hits []Chunk
}

func (v *tunedVectorStub) SearchVector(ctx context.Context, e []float32, k int) ([]Chunk, error) {
	v.topK = k
	return v.hits, nil
}

func (v *tunedVectorStub) SearchVectorTuned(ctx context.Context, e []float32, k int, opts VectorSearchOptions) ([]Chunk, error) {
	v.topK = k
	v.ef = opts.EFSearch
	return v.hits, nil
}

func TestHybridRetrieverSearchTuned(t *testing.T) {
	vector := &tunedVectorStub{hits: []Chunk{{ChunkID: "v1", Content: "alpha"}}}
	text := textStub{hits: []Chunk{{ChunkID: "t1", Content: "beta"}}}
	h := NewHybridRetriever(vector, text, NewRRFFuser(60))

	out, err := h.SearchTuned(context.Background(), "norma", 6, SearchTuning{
		TopK:                 4,
		VectorEFSearch:       20,
		RetrieverExtraFactor: 1,
		RetrieverFuseFactor:  2,
		MinVectorCandidates:  4,
		MinFusionCandidates:  6,
	})
	if err != nil {
		t.Fatalf("search tuned: %v", err)
	}
	if vector.topK != 4 {
		t.Fatalf("expected vector topK 4, got %d", vector.topK)
	}
	if vector.ef != 20 {
		t.Fatalf("expected ef 20, got %d", vector.ef)
	}
	if len(out) == 0 {
		t.Fatalf("expected output")
	}
}
