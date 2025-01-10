package index

import "context"

// Chunk is a retrievable content unit.
type Chunk struct {
    DocID   string
    ChunkID string
    Score   float64
    Content string
}

// Retriever defines hybrid retrieval behavior.
type Retriever interface {
    Search(ctx context.Context, query string, topK int) ([]Chunk, error)
}

// MockRetriever is a development-only retriever.
type MockRetriever struct{}

func (m MockRetriever) Search(ctx context.Context, query string, topK int) ([]Chunk, error) {
    _ = ctx
    if topK <= 0 {
        topK = 3
    }
    out := make([]Chunk, 0, topK)
    for i := 0; i < topK; i++ {
        out = append(out, Chunk{
            DocID:   "mock-doc",
            ChunkID: "chunk-" + string(rune('A'+i)),
            Score:   1.0 - float64(i)*0.1,
            Content: "mock retrieval result for query: " + query,
        })
    }
    return out, nil
}
