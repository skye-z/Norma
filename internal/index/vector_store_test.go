package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskVectorStoreSearch(t *testing.T) {
	base := filepath.Join(t.TempDir(), "v.json")
	s, err := NewDiskVectorStore(base)
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

	if _, err := os.Stat(base + ".hnsw"); err != nil {
		t.Fatalf("expected hnsw index file: %v", err)
	}
}

func TestDiskVectorStoreDeleteRebuildsIndex(t *testing.T) {
	s, err := NewDiskVectorStore(filepath.Join(t.TempDir(), "v.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := s.Upsert("docs/a.txt", []string{"norma project"}); err != nil {
		t.Fatalf("upsert a: %v", err)
	}
	if err := s.Upsert("docs/b.txt", []string{"golang companion"}); err != nil {
		t.Fatalf("upsert b: %v", err)
	}

	if err := s.Delete("docs/a.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	hits, err := s.Search(context.Background(), "norma", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, h := range hits {
		if h.DocID == "docs/a.txt" {
			t.Fatalf("deleted doc should not appear in search results")
		}
	}
}
