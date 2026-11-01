// Package memory 记忆服务
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultHNSWM              = 16
	defaultHNSWEFConstruction = 200
	defaultHNSWEFSearch       = 64
)

type vectorDoc struct {
	ChunkID string    `json:"chunk_id"`
	Path    string    `json:"path"`
	Content string    `json:"content"`
	Vec     []float64 `json:"vec"`
}

// 持久化向量存储，使用 HNSW 进行近似最近邻搜索
type DiskVectorStore struct {
	mu           sync.RWMutex
	path         string
	docs         []vectorDoc
	embedder     Embedder
	hnswIndex    *hnswIndex
	hnswM        int
	hnswEFBuild  int
	hnswEFSearch int
}

// 创建向量存储
func NewDiskVectorStore(path string) (*DiskVectorStore, error) {
	return NewDiskVectorStoreWithEmbedder(path, HashEmbedder{})
}

// 创建带 Embedder 的向量存储
func NewDiskVectorStoreWithEmbedder(path string, embedder Embedder) (*DiskVectorStore, error) {
	if embedder == nil {
		embedder = HashEmbedder{}
	}
	s := &DiskVectorStore{
		path:         path,
		embedder:     embedder,
		hnswM:        defaultHNSWM,
		hnswEFBuild:  defaultHNSWEFConstruction,
		hnswEFSearch: defaultHNSWEFSearch,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *DiskVectorStore) Upsert(path string, chunks []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.docs[:0]
	for _, d := range s.docs {
		if d.Path != path {
			filtered = append(filtered, d)
		}
	}
	s.docs = filtered

	for i, c := range chunks {
		v32, err := s.embedder.Embed(context.Background(), c)
		if err != nil {
			return err
		}
		s.docs = append(s.docs, vectorDoc{
			ChunkID: chunkID(path, i),
			Path:    path,
			Content: c,
			Vec:     to64(v32),
		})
	}

	if err := s.rebuildHNSWLocked(); err != nil {
		return err
	}
	return s.save()
}

func (s *DiskVectorStore) Delete(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.docs[:0]
	for _, d := range s.docs {
		if d.Path != path {
			filtered = append(filtered, d)
		}
	}
	s.docs = filtered

	if err := s.rebuildHNSWLocked(); err != nil {
		return err
	}
	return s.save()
}

func (s *DiskVectorStore) Search(ctx context.Context, query string, topK int) ([]Chunk, error) {
	q, err := s.embedder.Embed(ctx, query)
	if err != nil {
		q = embedF32(query)
	}
	return s.SearchVectorTuned(ctx, q, topK, VectorSearchOptions{})
}

func (s *DiskVectorStore) SearchVector(ctx context.Context, queryEmbedding []float32, topK int) ([]Chunk, error) {
	return s.SearchVectorTuned(ctx, queryEmbedding, topK, VectorSearchOptions{})
}

func (s *DiskVectorStore) SearchVectorTuned(ctx context.Context, queryEmbedding []float32, topK int, opts VectorSearchOptions) ([]Chunk, error) {
	_ = ctx
	if topK <= 0 {
		topK = 5
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.docs) == 0 {
		return nil, nil
	}

	if s.hnswIndex != nil && len(queryEmbedding) > 0 {
		hits := s.searchHNSWLocked(queryEmbedding, topK, opts)
		if len(hits) > 0 {
			return hits, nil
		}
	}

	return s.searchLinearLocked(queryEmbedding, topK), nil
}

func (s *DiskVectorStore) searchHNSWLocked(queryEmbedding []float32, topK int, opts VectorSearchOptions) []Chunk {
	if s.hnswIndex == nil {
		return nil
	}

	ef := s.hnswEFSearch
	if opts.EFSearch > 0 {
		ef = opts.EFSearch
	}
	if ef < topK*2 {
		ef = topK * 2
	}

	hits := make([]Chunk, 0, topK)
	for _, item := range s.hnswIndex.Search(queryEmbedding, ef, topK) {
		if item.ID < 0 || item.ID >= len(s.docs) {
			continue
		}
		doc := s.docs[item.ID]
		hits = append(hits, Chunk{DocID: doc.Path, ChunkID: doc.ChunkID, Score: 1 - item.Dist, Content: doc.Content})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

func (s *DiskVectorStore) searchLinearLocked(queryEmbedding []float32, topK int) []Chunk {
	q := to64(queryEmbedding)

	type hit struct {
		doc   vectorDoc
		score float64
	}
	hits := make([]hit, 0, len(s.docs))
	for _, d := range s.docs {
		hits = append(hits, hit{doc: d, score: cosine(q, d.Vec)})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > topK {
		hits = hits[:topK]
	}

	out := make([]Chunk, 0, len(hits))
	for _, h := range hits {
		out = append(out, Chunk{DocID: h.doc.Path, ChunkID: h.doc.ChunkID, Score: h.score, Content: h.doc.Content})
	}
	return out
}

func (s *DiskVectorStore) rebuildHNSWLocked() error {
	if len(s.docs) == 0 {
		s.hnswIndex = nil
		return nil
	}

	dim := len(s.docs[0].Vec)
	if dim == 0 {
		s.hnswIndex = nil
		return nil
	}

	idx := newHNSW(s.hnswM, s.hnswEFBuild)
	for i, d := range s.docs {
		idx.Add(i, to32(d.Vec, dim))
	}

	s.hnswIndex = idx
	return nil
}

func (s *DiskVectorStore) load() error {
	if s.path == "" {
		return nil
	}

	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(b) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, &s.docs); err != nil {
		return err
	}
	// 嵌入器变更后旧向量不可再用，重新生成
	if id := embedderID(s.embedder); id != s.readEmbedderID() {
		for i := range s.docs {
			v, err := s.embedder.Embed(context.Background(), s.docs[i].Content)
			if err != nil {
				return err
			}
			s.docs[i].Vec = to64(v)
		}
		if err := s.save(); err != nil {
			return err
		}
	}

	return s.rebuildHNSWLocked()
}

// 嵌入器标识；未提供名称的嵌入器以类型名代替
func embedderID(e Embedder) string {
	if n, ok := e.(interface{ Name() string }); ok {
		return n.Name()
	}
	return fmt.Sprintf("%T", e)
}

func (s *DiskVectorStore) readEmbedderID() string {
	b, err := os.ReadFile(s.path + ".embedder")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (s *DiskVectorStore) save() error {
	if s.path == "" {
		return nil
	}

	b, err := json.MarshalIndent(s.docs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, b, 0o644); err != nil {
		return err
	}

	return os.WriteFile(s.path+".embedder", []byte(embedderID(s.embedder)), 0o644)
}

func chunkID(path string, i int) string {
	return path + "#" + strconv.Itoa(i)
}

func embedF32(text string) []float32 {
	vec64 := embed(text)
	out := make([]float32, len(vec64))
	for i, v := range vec64 {
		out[i] = float32(v)
	}
	return out
}

func embed(text string) []float64 {
	vec := make([]float64, 32)
	text = strings.ToLower(strings.TrimSpace(text))
	for i, r := range []rune(text) {
		vec[i%len(vec)] += float64((int(r)%97)+1) * 0.01
	}
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return vec
	}
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

func to64(v []float32) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = float64(x)
	}
	return out
}

func to32(v []float64, dim int) []float32 {
	if dim <= 0 {
		dim = len(v)
	}
	out := make([]float32, dim)
	for i := 0; i < dim && i < len(v); i++ {
		out[i] = float32(v[i])
	}
	return out
}

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	dot, na, nb := 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// 查询向量必须与入库向量使用同一个嵌入器
type QueryEmbedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

func (s *DiskVectorStore) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return s.embedder.Embed(ctx, text)
}
