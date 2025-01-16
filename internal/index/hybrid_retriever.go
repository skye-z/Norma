package index

import "context"

// RetrieverHybrid implements vector+fulltext hybrid retrieval.
type RetrieverHybrid struct {
	vector VectorSearcher
	text   FullTextSearcher
	fuser  RankFuser
}

func NewHybridRetriever(vector VectorSearcher, text FullTextSearcher, fuser RankFuser) *RetrieverHybrid {
	if fuser == nil {
		fuser = NewRRFFuser(60)
	}
	return &RetrieverHybrid{vector: vector, text: text, fuser: fuser}
}

func (r *RetrieverHybrid) Search(ctx context.Context, query string, topK int) ([]Chunk, error) {
	if topK <= 0 {
		topK = 5
	}

	extra := topK * 2
	if extra < 8 {
		extra = 8
	}

	var vHits []Chunk
	var tHits []Chunk
	var err error

	if r.vector != nil {
		emb := embedF32(query)
		vHits, err = r.vector.SearchVector(ctx, emb, extra)
		if err != nil {
			return nil, err
		}
	}
	if r.text != nil {
		tHits, err = r.text.SearchText(ctx, query, extra)
		if err != nil {
			return nil, err
		}
	}

	return r.fuser.Fuse(vHits, tHits, topK), nil
}

func (r *RetrieverHybrid) Components() (VectorSearcher, FullTextSearcher, RankFuser) {
	return r.vector, r.text, r.fuser
}
