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

func TestSplitStructuredSegments(t *testing.T) {
	in := "# Title\n\nfirst paragraph\nline2\n\n```go\nfmt.Println(\"hi\")\n```\n\nafter"
	segments := splitStructuredSegments(in)
	if len(segments) != 4 {
		t.Fatalf("expected 4 segments, got %d: %#v", len(segments), segments)
	}
	if segments[0] != "# Title" {
		t.Fatalf("unexpected heading segment: %q", segments[0])
	}
	if segments[1] != "first paragraph\nline2" {
		t.Fatalf("unexpected paragraph segment: %q", segments[1])
	}
	if segments[2] != "```go\nfmt.Println(\"hi\")\n```" {
		t.Fatalf("unexpected code segment: %q", segments[2])
	}
	if segments[3] != "after" {
		t.Fatalf("unexpected tail segment: %q", segments[3])
	}
}

func TestChunkStructuredWithOverlap(t *testing.T) {
	in := "abcdefghijklmnopqrstuvwxyz"
	chunks := chunkStructured(in, 10, 2)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d (%#v)", len(chunks), chunks)
	}
	if chunks[0] != "abcdefghij" || chunks[1] != "ijklmnopqr" || chunks[2] != "qrstuvwxyz" {
		t.Fatalf("unexpected chunks with overlap: %#v", chunks)
	}
}

func TestChunkStructuredKeepsCrossSegmentOverlap(t *testing.T) {
	in := "12345\n\n67890\n\nabcde"
	chunks := chunkStructured(in, 10, 3)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d (%#v)", len(chunks), chunks)
	}
	if chunks[0] != "12345" {
		t.Fatalf("unexpected chunk[0]: %q", chunks[0])
	}
	if chunks[1] != "345\n\n67890" {
		t.Fatalf("unexpected chunk[1]: %q", chunks[1])
	}
	if chunks[2] != "890\n\nabcde" {
		t.Fatalf("unexpected chunk[2]: %q", chunks[2])
	}
}
