package memory

import (
	"context"
	"path/filepath"
	"testing"

	"norma/internal/db"
)

func TestMemoryServiceAddSearchDelete(t *testing.T) {
	engine, err := db.NewEngine(filepath.Join(t.TempDir(), "m.db"), false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	svc := NewService(engine)
	m, err := svc.Add(context.Background(), KindPreference, "喜欢简洁回复", 8)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if m == nil || m.ID == 0 {
		t.Fatalf("expected inserted memory")
	}

	hits, err := svc.Search(context.Background(), "简洁", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected search hits")
	}

	if err := svc.Delete(context.Background(), m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestMemoryCompact(t *testing.T) {
	engine, err := db.NewEngine(filepath.Join(t.TempDir(), "c.db"), false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	svc := NewService(engine)
	_, _ = svc.Add(context.Background(), KindFact, "同一事实", 1)
	_, _ = svc.Add(context.Background(), KindFact, "同一事实", 8)
	_, _ = svc.Add(context.Background(), KindEvent, "事件A", 5)

	deleted, err := svc.Compact(context.Background())
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if deleted == 0 {
		t.Fatalf("expected compact to delete duplicates")
	}

	hits, err := svc.Search(context.Background(), "同一事实", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 deduped row, got %d", len(hits))
	}
	if hits[0].Importance != 8 {
		t.Fatalf("expected higher importance kept, got %d", hits[0].Importance)
	}
}
