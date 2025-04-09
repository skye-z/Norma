package memory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// RetrieverPostProcessOptions controls optional post-process stages after fusion.
type RetrieverPostProcessOptions struct {
	EnableDedup         bool
	EnableAdjacentMerge bool
	EnableRerank        bool
	MaxMergeGap         int
	RerankWeight        float64
}

// RetrieverHybrid implements vector+fulltext hybrid retrieval.
type RetrieverHybrid struct {
	vector VectorSearcher
	text   FullTextSearcher
	fuser  RankFuser
	opts   RetrieverPostProcessOptions
}

func NewHybridRetriever(vector VectorSearcher, text FullTextSearcher, fuser RankFuser) *RetrieverHybrid {
	if fuser == nil {
		fuser = NewRRFFuser(60)
	}
	return &RetrieverHybrid{
		vector: vector,
		text:   text,
		fuser:  fuser,
		opts:   defaultRetrieverPostProcessOptions(),
	}
}

func (r *RetrieverHybrid) SetPostProcessOptions(opts RetrieverPostProcessOptions) {
	r.opts = normalizeRetrieverPostProcessOptions(opts)
}

func (r *RetrieverHybrid) Search(ctx context.Context, query string, topK int) ([]Chunk, error) {
	return r.SearchTuned(ctx, query, topK, DefaultSearchTuning(topK))
}

func (r *RetrieverHybrid) SearchTuned(ctx context.Context, query string, topK int, tuning SearchTuning) ([]Chunk, error) {
	tuning = normalizeSearchTuning(tuning, topK)
	topK = tuning.TopK

	var vHits []Chunk
	var tHits []Chunk
	var err error

	if r.vector != nil {
		emb := embedF32(query)
		if tuned, ok := r.vector.(TunedVectorSearcher); ok {
			vHits, err = tuned.SearchVectorTuned(ctx, emb, candidateCount(topK, tuning.RetrieverExtraFactor, tuning.MinVectorCandidates), VectorSearchOptions{EFSearch: tuning.VectorEFSearch})
		} else {
			vHits, err = r.vector.SearchVector(ctx, emb, candidateCount(topK, tuning.RetrieverExtraFactor, tuning.MinVectorCandidates))
		}
		if err != nil {
			return nil, err
		}
	}
	if r.text != nil {
		tHits, err = r.text.SearchText(ctx, query, candidateCount(topK, tuning.RetrieverExtraFactor, tuning.MinVectorCandidates))
		if err != nil {
			return nil, err
		}
	}

	fused := r.fuser.Fuse(vHits, tHits, candidateCount(topK, tuning.RetrieverFuseFactor, tuning.MinFusionCandidates))
	out := optimizeRetrievedChunks(query, fused, r.opts)
	if len(out) > topK {
		out = out[:topK]
	}
	return out, nil
}

func normalizeSearchTuning(tuning SearchTuning, topK int) SearchTuning {
	defaults := DefaultSearchTuning(topK)
	if tuning.TopK <= 0 {
		tuning.TopK = defaults.TopK
	}
	if tuning.RetrieverExtraFactor <= 0 {
		tuning.RetrieverExtraFactor = defaults.RetrieverExtraFactor
	}
	if tuning.RetrieverFuseFactor <= 0 {
		tuning.RetrieverFuseFactor = defaults.RetrieverFuseFactor
	}
	if tuning.MinVectorCandidates <= 0 {
		tuning.MinVectorCandidates = defaults.MinVectorCandidates
	}
	if tuning.MinFusionCandidates <= 0 {
		tuning.MinFusionCandidates = defaults.MinFusionCandidates
	}
	return tuning
}

func candidateCount(topK, factor, minimum int) int {
	count := topK * factor
	if count < minimum {
		count = minimum
	}
	if count < topK {
		count = topK
	}
	return count
}

func (r *RetrieverHybrid) Components() (VectorSearcher, FullTextSearcher, RankFuser) {
	return r.vector, r.text, r.fuser
}

func defaultRetrieverPostProcessOptions() RetrieverPostProcessOptions {
	return RetrieverPostProcessOptions{
		EnableDedup:         true,
		EnableAdjacentMerge: true,
		EnableRerank:        true,
		MaxMergeGap:         1,
		RerankWeight:        0.15,
	}
}

func normalizeRetrieverPostProcessOptions(opts RetrieverPostProcessOptions) RetrieverPostProcessOptions {
	if opts.MaxMergeGap < 0 {
		opts.MaxMergeGap = 0
	}
	if opts.RerankWeight <= 0 {
		opts.RerankWeight = 0.15
	}
	return opts
}

func optimizeRetrievedChunks(query string, chunks []Chunk, opts RetrieverPostProcessOptions) []Chunk {
	out := append([]Chunk(nil), chunks...)
	if len(out) == 0 {
		return out
	}

	opts = normalizeRetrieverPostProcessOptions(opts)

	if opts.EnableDedup {
		out = dedupeChunks(out)
	}
	if opts.EnableAdjacentMerge {
		out = mergeAdjacentChunks(out, opts.MaxMergeGap)
	}
	if opts.EnableRerank {
		rerankByQueryOverlap(query, out, opts.RerankWeight)
	} else {
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].Score > out[j].Score
		})
	}

	return out
}

func dedupeChunks(chunks []Chunk) []Chunk {
	if len(chunks) == 0 {
		return nil
	}

	type seenRef struct {
		idx int
	}

	byID := map[string]seenRef{}
	byContent := map[string]seenRef{}
	out := make([]Chunk, 0, len(chunks))

	choose := func(old Chunk, cur Chunk) Chunk {
		if cur.Score > old.Score {
			return cur
		}
		if cur.Score == old.Score && len(cur.Content) > len(old.Content) {
			return cur
		}
		return old
	}

	for _, c := range chunks {
		if id := strings.TrimSpace(c.ChunkID); id != "" {
			if ref, ok := byID[id]; ok {
				out[ref.idx] = choose(out[ref.idx], c)
				continue
			}
		}

		ck := normalizedContentKey(c.Content)
		if ck != "" {
			if ref, ok := byContent[ck]; ok {
				out[ref.idx] = choose(out[ref.idx], c)
				continue
			}
		}

		out = append(out, c)
		idx := len(out) - 1
		if id := strings.TrimSpace(c.ChunkID); id != "" {
			byID[id] = seenRef{idx: idx}
		}
		if ck != "" {
			byContent[ck] = seenRef{idx: idx}
		}
	}

	return out
}

func mergeAdjacentChunks(chunks []Chunk, maxGap int) []Chunk {
	if len(chunks) < 2 {
		return chunks
	}
	if maxGap < 0 {
		maxGap = 0
	}

	type indexedChunk struct {
		chunk Chunk
		doc   string
		seq   int
	}

	groups := map[string][]indexedChunk{}
	passthrough := make([]Chunk, 0)

	for _, c := range chunks {
		doc, seq, ok := parseChunkSeq(c.DocID, c.ChunkID)
		if !ok {
			passthrough = append(passthrough, c)
			continue
		}
		groups[doc] = append(groups[doc], indexedChunk{chunk: c, doc: doc, seq: seq})
	}

	merged := make([]Chunk, 0, len(chunks))
	merged = append(merged, passthrough...)

	for doc, arr := range groups {
		sort.Slice(arr, func(i, j int) bool { return arr[i].seq < arr[j].seq })

		run := []indexedChunk{arr[0]}
		flushRun := func() {
			if len(run) == 0 {
				return
			}
			if len(run) == 1 {
				merged = append(merged, run[0].chunk)
				run = run[:0]
				return
			}

			contentParts := make([]string, 0, len(run))
			seenContent := map[string]struct{}{}
			best := run[0].chunk
			start := run[0].seq
			end := run[0].seq
			for _, rc := range run {
				if rc.chunk.Score > best.Score {
					best = rc.chunk
				}
				if rc.seq < start {
					start = rc.seq
				}
				if rc.seq > end {
					end = rc.seq
				}
				part := strings.TrimSpace(rc.chunk.Content)
				if part == "" {
					continue
				}
				k := normalizedContentKey(part)
				if _, ok := seenContent[k]; ok {
					continue
				}
				seenContent[k] = struct{}{}
				contentParts = append(contentParts, part)
			}

			best.DocID = doc
			best.ChunkID = fmt.Sprintf("%s#%d-%d", doc, start, end)
			best.Content = strings.TrimSpace(strings.Join(contentParts, "\n\n"))
			merged = append(merged, best)
			run = run[:0]
		}

		for i := 1; i < len(arr); i++ {
			prev := run[len(run)-1]
			cur := arr[i]
			if cur.seq-prev.seq <= maxGap+1 {
				run = append(run, cur)
				continue
			}
			flushRun()
			run = append(run, cur)
		}
		flushRun()
	}

	return merged
}

func parseChunkSeq(docID, chunkID string) (string, int, bool) {
	id := strings.TrimSpace(chunkID)
	if id == "" {
		return "", 0, false
	}

	i := strings.LastIndex(id, "#")
	if i < 0 || i+1 >= len(id) {
		return "", 0, false
	}
	seq, err := strconv.Atoi(id[i+1:])
	if err != nil {
		return "", 0, false
	}
	if strings.TrimSpace(docID) == "" {
		docID = id[:i]
	}
	if strings.TrimSpace(docID) == "" {
		return "", 0, false
	}
	return docID, seq, true
}

func rerankByQueryOverlap(query string, chunks []Chunk, weight float64) {
	if len(chunks) == 0 {
		return
	}
	qTokens := tokenizeText(query)
	type scored struct {
		idx   int
		chunk Chunk
		score float64
	}
	arr := make([]scored, 0, len(chunks))
	for i, c := range chunks {
		overlap := lexicalOverlapScore(qTokens, tokenizeText(c.Content))
		rank := c.Score + weight*overlap
		c.Score = rank
		arr = append(arr, scored{idx: i, chunk: c, score: rank})
	}

	sort.SliceStable(arr, func(i, j int) bool {
		if arr[i].score == arr[j].score {
			return arr[i].idx < arr[j].idx
		}
		return arr[i].score > arr[j].score
	})

	for i := range arr {
		chunks[i] = arr[i].chunk
	}
}

func lexicalOverlapScore(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(b))
	for _, tok := range b {
		set[tok] = struct{}{}
	}
	matched := 0
	seen := map[string]struct{}{}
	for _, tok := range a {
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		if _, ok := set[tok]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(seen))
}

func tokenizeText(s string) []string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r))
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) < 2 {
			continue
		}
		out = append(out, p)
	}
	return out
}

func normalizedContentKey(s string) string {
	tokens := tokenizeText(s)
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) > 32 {
		tokens = tokens[:32]
	}
	return strings.Join(tokens, " ")
}
