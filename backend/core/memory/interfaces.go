package memory

import "context"

// Chunk 可检索的内容单元
// 包含文档ID、片段ID、相似度分数和实际内容
type Chunk struct {
	DocID   string
	ChunkID string
	Score   float64
	Content string
}

// Retriever 定义混合检索的行为接口
// 用于根据查询字符串返回最相关的记忆片段
type Retriever interface {
	Search(ctx context.Context, query string, topK int) ([]Chunk, error)
}

// MockRetriever 开发阶段使用的模拟检索器
// 用于在没有实际后端存储时进行开发和测试
type MockRetriever struct{}

// Search 实现 Retriever 接口
// 返回指定数量的模拟检索结果，模拟真实的检索行为
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
