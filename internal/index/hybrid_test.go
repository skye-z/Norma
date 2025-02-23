package index

import (
	"context"
	"testing"
)

type vectorStub struct{ hits []Chunk }

func (v vectorStub) SearchVector(ctx context.Context, e []float32, k int) ([]Chunk, error) {
	_ = ctx
	_ = e
	_ = k
	return v.hits, nil
}

type textStub struct{ hits []Chunk }

func (t textStub) SearchText(ctx context.Context, q string, k int) ([]Chunk, error) {
	_ = ctx
	_ = q
	_ = k
	return t.hits, nil
}

func TestRRFFuser(t *testing.T) {
	f := NewRRFFuser(60)
	v := []Chunk{{ChunkID: "a"}, {ChunkID: "b"}}
	tx := []Chunk{{ChunkID: "b"}, {ChunkID: "c"}}
	out := f.Fuse(v, tx, 3)
	if len(out) != 3 {
		t.Fatalf("expected 3, got %d", len(out))
	}
	if out[0].ChunkID != "b" {
		t.Fatalf("expected b first, got %s", out[0].ChunkID)
	}
}

func TestHybridRetriever(t *testing.T) {
	h := NewHybridRetriever(
		vectorStub{hits: []Chunk{{ChunkID: "v1", Content: "a"}, {ChunkID: "shared", Content: "b"}}},
		textStub{hits: []Chunk{{ChunkID: "shared", Content: "b"}, {ChunkID: "t1", Content: "c"}}},
		NewRRFFuser(60),
	)

	out, err := h.Search(context.Background(), "norma", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected non-empty")
	}
	if out[0].ChunkID != "shared" {
		t.Fatalf("expected shared first, got %s", out[0].ChunkID)
	}
}

func TestHybridRetrieverPostProcessDedupeByContent(t *testing.T) {
	h := NewHybridRetriever(
		vectorStub{hits: []Chunk{{DocID: "doc1", ChunkID: "doc1#0", Score: 0.9, Content: "same content alpha"}}},
		textStub{hits: []Chunk{{DocID: "doc2", ChunkID: "doc2#8", Score: 0.8, Content: "same content alpha"}, {DocID: "doc3", ChunkID: "doc3#1", Score: 0.7, Content: "unique beta"}}},
		NewRRFFuser(60),
	)

	h.SetPostProcessOptions(RetrieverPostProcessOptions{
		EnableDedup:         true,
		EnableAdjacentMerge: false,
		EnableRerank:        false,
		MaxMergeGap:         1,
		RerankWeight:        0.15,
	})

	out, err := h.Search(context.Background(), "alpha", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected deduped size 2, got %d (%#v)", len(out), out)
	}
}

func TestHybridRetrieverPostProcessMergeAdjacent(t *testing.T) {
	chunks := []Chunk{
		{DocID: "doc", ChunkID: "doc#0", Score: 0.4, Content: "first"},
		{DocID: "doc", ChunkID: "doc#1", Score: 0.5, Content: "second"},
		{DocID: "doc", ChunkID: "doc#3", Score: 0.3, Content: "third"},
	}
	out := optimizeRetrievedChunks("", chunks, RetrieverPostProcessOptions{
		EnableDedup:         false,
		EnableAdjacentMerge: true,
		EnableRerank:        false,
		MaxMergeGap:         0,
		RerankWeight:        0.15,
	})

	if len(out) != 2 {
		t.Fatalf("expected 2 merged chunks, got %d (%#v)", len(out), out)
	}
	foundMerged := false
	for _, c := range out {
		if c.ChunkID == "doc#0-1" {
			foundMerged = true
			if c.Content != "first\n\nsecond" {
				t.Fatalf("unexpected merged content: %q", c.Content)
			}
		}
	}
	if !foundMerged {
		t.Fatalf("missing merged adjacent chunk in %#v", out)
	}
}

func TestHybridRetrieverPostProcessRerank(t *testing.T) {
	chunks := []Chunk{
		{ChunkID: "c1", Score: 0.5, Content: "nothing relevant"},
		{ChunkID: "c2", Score: 0.5, Content: "alpha feature details"},
	}
	out := optimizeRetrievedChunks("alpha", chunks, RetrieverPostProcessOptions{
		EnableDedup:         false,
		EnableAdjacentMerge: false,
		EnableRerank:        true,
		MaxMergeGap:         1,
		RerankWeight:        0.3,
	})
	if len(out) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(out))
	}
	if out[0].ChunkID != "c2" {
		t.Fatalf("expected query-related chunk first, got %s", out[0].ChunkID)
	}
}
