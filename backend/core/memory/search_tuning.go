// Package memory 记忆服务
package memory

import "context"

// 搜索调优参数
type SearchTuning struct {
	TopK                 int
	VectorEFSearch       int
	RetrieverExtraFactor int
	RetrieverFuseFactor  int
	MinVectorCandidates  int
	MinFusionCandidates  int
}

// 调优检索器接口
type TunedRetriever interface {
	SearchTuned(ctx context.Context, query string, topK int, tuning SearchTuning) ([]Chunk, error)
}

// 调优向量搜索器接口
type TunedVectorSearcher interface {
	SearchVectorTuned(ctx context.Context, queryEmbedding []float32, topK int, opts VectorSearchOptions) ([]Chunk, error)
}

// 向量搜索选项
type VectorSearchOptions struct {
	EFSearch int
}

// 默认搜索调优
func DefaultSearchTuning(topK int) SearchTuning {
	if topK <= 0 {
		topK = 5
	}
	return SearchTuning{
		TopK:                 topK,
		VectorEFSearch:       0,
		RetrieverExtraFactor: 2,
		RetrieverFuseFactor:  3,
		MinVectorCandidates:  8,
		MinFusionCandidates:  12,
	}
}
