package index

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	hnsw "github.com/Bithack/go-hnsw"
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

// DiskVectorStore persists vector metadata and serves ANN search with HNSW.
type DiskVectorStore struct {
	mu           sync.RWMutex
	path         string
	docs         []vectorDoc
	embedder     Embedder
	hnswIndex    *hnsw.Hnsw
	hnswM        int
	hnswEFBuild  int
	hnswEFSearch int
}

func NewDiskVectorStore(path string) (*DiskVectorStore, error) {
	return NewDiskVectorStoreWithEmbedder(path, HashEmbedder{})
}

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
			v32 = embedF32(c)
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
	return s.SearchVector(ctx, q, topK)
}

func (s *DiskVectorStore) SearchVector(ctx context.Context, queryEmbedding []float32, topK int) ([]Chunk, error) {
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
		hits := s.searchHNSWLocked(queryEmbedding, topK)
		if len(hits) > 0 {
			return hits, nil
		}
	}

	return s.searchLinearLocked(queryEmbedding, topK), nil
}

func (s *DiskVectorStore) searchHNSWLocked(queryEmbedding []float32, topK int) []Chunk {
	if s.hnswIndex == nil {
		return nil
	}

	ef := s.hnswEFSearch
	if ef < topK*2 {
		ef = topK * 2
	}

	result := s.hnswIndex.Search(hnsw.Point(queryEmbedding), ef, topK)
	hits := make([]Chunk, 0, topK)
	for result.Len() > 0 {
		item := result.Pop()
		if item == nil {
			break
		}
		docIdx := int(item.ID) - 1
		if docIdx < 0 || docIdx >= len(s.docs) {
			continue
		}
		doc := s.docs[docIdx]
		score := 1.0 / (1.0 + float64(item.D))
		hits = append(hits, Chunk{DocID: doc.Path, ChunkID: doc.ChunkID, Score: score, Content: doc.Content})
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

	zero := make(hnsw.Point, dim)
	idx := hnsw.New(s.hnswM, s.hnswEFBuild, zero)
	idx.Grow(len(s.docs) + 1)

	for i, d := range s.docs {
		idx.Add(to32(d.Vec, dim), uint32(i+1))
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

	hnswPath := s.hnswPath()
	if _, err := os.Stat(hnswPath); err == nil {
		if idx, _, loadErr := hnsw.Load(hnswPath); loadErr == nil {
			s.hnswIndex = idx
			return nil
		}
	}

	return s.rebuildHNSWLocked()
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

	if s.hnswIndex != nil {
		if err := os.MkdirAll(filepath.Dir(s.hnswPath()), 0o755); err != nil {
			return err
		}
		if err := s.hnswIndex.Save(s.hnswPath()); err != nil {
			return err
		}
	} else {
		_ = os.Remove(s.hnswPath())
	}

	return nil
}

func (s *DiskVectorStore) hnswPath() string {
	if s.path == "" {
		return ""
	}
	return s.path + ".hnsw"
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
	dot := 0.0
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
	}
	return dot
}
