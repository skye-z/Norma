// Package memory 记忆服务
package memory

import (
	"container/heap"
	"math"
	"math/rand"
	"sort"
)

// 纯 Go 的 HNSW 近似最近邻索引，距离为 1 - 余弦相似度
type hnswIndex struct {
	m        int
	mMax0    int
	efBuild  int
	levelMul float64
	rng      *rand.Rand
	nodes    []*hnswNode
	entry    int
	maxLevel int
}

type hnswNode struct {
	id      int
	vec     []float32
	friends [][]int
}

// 创建索引
func newHNSW(m, efBuild int) *hnswIndex {
	if m < 4 {
		m = 4
	}
	if efBuild < m {
		efBuild = m
	}
	return &hnswIndex{
		m:        m,
		mMax0:    m * 2,
		efBuild:  efBuild,
		levelMul: 1 / math.Log(float64(m)),
		rng:      rand.New(rand.NewSource(42)),
		entry:    -1,
	}
}

func hnswDist(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 1
	}
	return 1 - dot/(math.Sqrt(na)*math.Sqrt(nb))
}

type distItem struct {
	idx  int
	dist float64
}

type minHeap []distItem

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h minHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x interface{}) { *h = append(*h, x.(distItem)) }
func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	it := old[n-1]
	*h = old[:n-1]
	return it
}

type maxHeap []distItem

func (h maxHeap) Len() int            { return len(h) }
func (h maxHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist }
func (h maxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *maxHeap) Push(x interface{}) { *h = append(*h, x.(distItem)) }
func (h *maxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	it := old[n-1]
	*h = old[:n-1]
	return it
}

// 在给定层上从入口集合出发做贪心扩展，返回最近的 ef 个节点
func (h *hnswIndex) searchLayer(q []float32, entries []distItem, ef, level int) []distItem {
	visited := make(map[int]struct{}, ef*4)
	cand := &minHeap{}
	res := &maxHeap{}
	for _, e := range entries {
		visited[e.idx] = struct{}{}
		heap.Push(cand, e)
		heap.Push(res, e)
	}
	for cand.Len() > 0 {
		c := heap.Pop(cand).(distItem)
		if res.Len() >= ef && c.dist > (*res)[0].dist {
			break
		}
		node := h.nodes[c.idx]
		if level >= len(node.friends) {
			continue
		}
		for _, f := range node.friends[level] {
			if _, ok := visited[f]; ok {
				continue
			}
			visited[f] = struct{}{}
			d := hnswDist(q, h.nodes[f].vec)
			if res.Len() < ef || d < (*res)[0].dist {
				heap.Push(cand, distItem{f, d})
				heap.Push(res, distItem{f, d})
				if res.Len() > ef {
					heap.Pop(res)
				}
			}
		}
	}
	out := make([]distItem, res.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(res).(distItem)
	}
	return out
}

// 添加向量，id 为调用方使用的整数编号
func (h *hnswIndex) Add(id int, vec []float32) {
	level := int(-math.Log(h.rng.Float64()+1e-12) * h.levelMul)
	node := &hnswNode{id: id, vec: vec, friends: make([][]int, level+1)}
	idx := len(h.nodes)
	h.nodes = append(h.nodes, node)
	if h.entry < 0 {
		h.entry, h.maxLevel = idx, level
		return
	}
	ep := []distItem{{h.entry, hnswDist(vec, h.nodes[h.entry].vec)}}
	for l := h.maxLevel; l > level; l-- {
		ep = h.searchLayer(vec, ep, 1, l)[:1]
	}
	top := level
	if h.maxLevel < top {
		top = h.maxLevel
	}
	for l := top; l >= 0; l-- {
		found := h.searchLayer(vec, ep, h.efBuild, l)
		limit := h.m
		if l == 0 {
			limit = h.mMax0
		}
		sel := found
		if len(sel) > h.m {
			sel = sel[:h.m]
		}
		for _, s := range sel {
			node.friends[l] = append(node.friends[l], s.idx)
			peer := h.nodes[s.idx]
			peer.friends[l] = append(peer.friends[l], idx)
			if len(peer.friends[l]) > limit {
				h.shrink(peer, l, limit)
			}
		}
		ep = found
	}
	if level > h.maxLevel {
		h.entry, h.maxLevel = idx, level
	}
}

func (h *hnswIndex) shrink(n *hnswNode, level, limit int) {
	items := make([]distItem, 0, len(n.friends[level]))
	for _, f := range n.friends[level] {
		items = append(items, distItem{f, hnswDist(n.vec, h.nodes[f].vec)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].dist < items[j].dist })
	items = items[:limit]
	n.friends[level] = n.friends[level][:0]
	for _, it := range items {
		n.friends[level] = append(n.friends[level], it.idx)
	}
}

type hnswHit struct {
	ID   int
	Dist float64
}

// 查询最相近的 k 个编号
func (h *hnswIndex) Search(q []float32, ef, k int) []hnswHit {
	if h.entry < 0 {
		return nil
	}
	if ef < k {
		ef = k
	}
	ep := []distItem{{h.entry, hnswDist(q, h.nodes[h.entry].vec)}}
	for l := h.maxLevel; l > 0; l-- {
		ep = h.searchLayer(q, ep, 1, l)[:1]
	}
	found := h.searchLayer(q, ep, ef, 0)
	if len(found) > k {
		found = found[:k]
	}
	out := make([]hnswHit, 0, len(found))
	for _, f := range found {
		out = append(out, hnswHit{ID: h.nodes[f.idx].id, Dist: f.dist})
	}
	return out
}
