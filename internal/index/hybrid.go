package index

import "context"

// VectorSearcher abstracts ANN/HNSW retrieval.
type VectorSearcher interface {
	SearchVector(ctx context.Context, queryEmbedding []float32, topK int) ([]Chunk, error)
}

// FullTextSearcher abstracts FTS5/BM25 retrieval.
type FullTextSearcher interface {
	SearchText(ctx context.Context, query string, topK int) ([]Chunk, error)
}

// RankFuser merges vector and full-text recall results.
type RankFuser interface {
	Fuse(vectorHits, textHits []Chunk, topK int) []Chunk
}

// HybridRetriever combines ANN + full-text + fusion.
type HybridRetriever interface {
	Retriever
	Components() (VectorSearcher, FullTextSearcher, RankFuser)
}
