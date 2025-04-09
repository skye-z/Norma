package memory

import "sort"

// RRFFuser merges two ranked lists using Reciprocal Rank Fusion.
type RRFFuser struct {
	K int
}

func NewRRFFuser(k int) *RRFFuser {
	if k <= 0 {
		k = 60
	}
	return &RRFFuser{K: k}
}

func (f *RRFFuser) Fuse(vectorHits, textHits []Chunk, topK int) []Chunk {
	if topK <= 0 {
		topK = 5
	}

	type scored struct {
		chunk Chunk
		score float64
	}
	byID := map[string]*scored{}

	acc := func(list []Chunk) {
		for i, h := range list {
			id := h.ChunkID
			if id == "" {
				id = h.DocID
			}
			if id == "" {
				continue
			}
			s, ok := byID[id]
			if !ok {
				s = &scored{chunk: h}
				byID[id] = s
			}
			s.score += 1.0 / float64(f.K+i+1)
		}
	}

	acc(vectorHits)
	acc(textHits)

	arr := make([]scored, 0, len(byID))
	for _, s := range byID {
		s.chunk.Score = s.score
		arr = append(arr, *s)
	}

	sort.Slice(arr, func(i, j int) bool { return arr[i].score > arr[j].score })
	if len(arr) > topK {
		arr = arr[:topK]
	}

	out := make([]Chunk, 0, len(arr))
	for _, s := range arr {
		out = append(out, s.chunk)
	}
	return out
}
