package memory

import "context"

// 可检索的内容单元
type Chunk struct {
	DocID   string
	ChunkID string
	Score   float64
	Content string
}

// 混合检索接口
type Retriever interface {
	Search(ctx context.Context, query string, topK int) ([]Chunk, error)
}

// 开发用模拟检索器
type MockRetriever struct{}

// 实现 Retriever 接口
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
