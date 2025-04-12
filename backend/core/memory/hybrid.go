package memory

import "context"

// VectorSearcher 向量检索器接口
// 抽象了 ANN (近似最近邻) 或 HNSW 等向量检索方法的实现
type VectorSearcher interface {
	SearchVector(ctx context.Context, queryEmbedding []float32, topK int) ([]Chunk, error)
}

// FullTextSearcher 全文检索器接口
// 抽象了 FTS5 或 BM25 等全文检索方法的实现
type FullTextSearcher interface {
	SearchText(ctx context.Context, query string, topK int) ([]Chunk, error)
}

// RankFuser 排名融合器接口
// 负责将向量检索和全文检索的结果进行合并排序
type RankFuser interface {
	Fuse(vectorHits, textHits []Chunk, topK int) []Chunk
}

// HybridRetriever 混合检索器接口
// 组合了向量检索、全文检索和排名融合三个组件
type HybridRetriever interface {
	Retriever
	Components() (VectorSearcher, FullTextSearcher, RankFuser)
}
