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
