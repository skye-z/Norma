// Package memory 记忆服务
package memory

import (
	"context"
	"math"
	"path/filepath"
	"sort"
	"time"
)

// DecayingRetriever 按 OpenClaw 的规则给检索结果加时间衰减：
// 每日笔记按半衰期衰减（默认 30 天），MEMORY.md 等长期文件不衰减。
// 衰减只改变排序，不会让条目消失
type DecayingRetriever struct {
	Inner        Retriever
	HalfLifeDays float64
	Now          func() time.Time
}

func NewDecayingRetriever(inner Retriever) *DecayingRetriever {
	return &DecayingRetriever{Inner: inner, HalfLifeDays: 30, Now: time.Now}
}

func (d *DecayingRetriever) Search(ctx context.Context, query string, topK int) ([]Chunk, error) {
	// 多取一些候选再重排，否则被衰减压下去的新条目没有机会上来
	chunks, err := d.Inner.Search(ctx, query, topK*2)
	if err != nil {
		return nil, err
	}
	now := d.Now()
	for i := range chunks {
		chunks[i].Score *= d.factor(chunks[i].DocID, now)
	}
	sort.SliceStable(chunks, func(i, j int) bool { return chunks[i].Score > chunks[j].Score })
	if topK > 0 && len(chunks) > topK {
		chunks = chunks[:topK]
	}
	return chunks, nil
}

func (d *DecayingRetriever) factor(docID string, now time.Time) float64 {
	m := dailyPathRe.FindStringSubmatch(filepath.ToSlash(docID))
	if m == nil || d.HalfLifeDays <= 0 {
		return 1
	}
	day, err := time.ParseInLocation("2006-01-02", m[1], now.Location())
	if err != nil {
		return 1
	}
	// 按日历日相差的天数计算：今天的笔记不衰减，不管现在是几点
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	age := today.Sub(day).Hours() / 24
	if age <= 0 {
		return 1
	}
	return math.Pow(0.5, age/d.HalfLifeDays)
}
