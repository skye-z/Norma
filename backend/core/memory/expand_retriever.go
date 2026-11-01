// Package memory 记忆服务
package memory

import (
	"context"
	"strings"
)

// 查询扩展器：把口语化的问题改写为若干检索关键词或同义表达
type QueryExpander interface {
	Expand(ctx context.Context, query string) ([]string, error)
}

// 查询扩展检索：原问题与扩展词分别做全文检索，再用 RRF 融合
// 没有本地嵌入模型时，由云端大模型补足语义层面的召回
type RetrieverExpanding struct {
	text     FullTextSearcher
	expander QueryExpander
	fuser    RankFuser
}

// 创建查询扩展检索器
func NewExpandingRetriever(text FullTextSearcher, expander QueryExpander) *RetrieverExpanding {
	return &RetrieverExpanding{text: text, expander: expander, fuser: NewRRFFuser(60)}
}

// 搜索；扩展失败时退化为对原问题的全文检索
func (r *RetrieverExpanding) Search(ctx context.Context, query string, topK int) ([]Chunk, error) {
	if topK <= 0 {
		topK = 4
	}
	cand := topK * 4
	base, err := r.text.SearchText(ctx, query, cand)
	if err != nil {
		return nil, err
	}
	if r.expander == nil {
		return trimChunks(base, topK), nil
	}
	terms, err := r.expander.Expand(ctx, query)
	if err != nil || len(terms) == 0 {
		return trimChunks(base, topK), nil
	}
	expanded, err := r.text.SearchText(ctx, strings.Join(terms, " "), cand)
	if err != nil {
		return trimChunks(base, topK), nil
	}
	return trimChunks(r.fuser.Fuse(base, expanded, cand), topK), nil
}

func trimChunks(c []Chunk, k int) []Chunk {
	if len(c) > k {
		return c[:k]
	}
	return c
}
