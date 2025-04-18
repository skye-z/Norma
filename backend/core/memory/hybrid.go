package memory

import "context"

// 向量检索器
type VectorSearcher interface {
	SearchVector(ctx context.Context, queryEmbedding []float32, topK int) ([]Chunk, error)
}

// 全文检索器
type FullTextSearcher interface {
	SearchText(ctx context.Context, query string, topK int) ([]Chunk, error)
}

// 排名融合器
type RankFuser interface {
	Fuse(vectorHits, textHits []Chunk, topK int) []Chunk
}

// 混合检索器
type HybridRetriever interface {
	Retriever
	Components() (VectorSearcher, FullTextSearcher, RankFuser)
}
