package index

import "context"

type SearchTuning struct {
	TopK                 int
	VectorEFSearch       int
	RetrieverExtraFactor int
	RetrieverFuseFactor  int
	MinVectorCandidates  int
	MinFusionCandidates  int
}

type TunedRetriever interface {
	SearchTuned(ctx context.Context, query string, topK int, tuning SearchTuning) ([]Chunk, error)
}

type TunedVectorSearcher interface {
	SearchVectorTuned(ctx context.Context, queryEmbedding []float32, topK int, opts VectorSearchOptions) ([]Chunk, error)
}

type VectorSearchOptions struct {
	EFSearch int
}

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
