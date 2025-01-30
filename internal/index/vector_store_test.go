package index

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDiskVectorStoreSearch(t *testing.T) {
	s, err := NewDiskVectorStore(filepath.Join(t.TempDir(), "v.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := s.Upsert("docs/a.txt", []string{"norma local deployment", "other"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	hits, err := s.Search(context.Background(), "local deployment", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected hits")
	}
}
