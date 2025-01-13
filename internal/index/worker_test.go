package index

import "testing"

func TestChunkText(t *testing.T) {
	in := "abcdefghijklmnopqrstuvwxyz"
	chunks := chunkText(in, 10)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0] != "abcdefghij" || chunks[1] != "klmnopqrst" || chunks[2] != "uvwxyz" {
		t.Fatalf("unexpected chunks: %#v", chunks)
	}
}
