package index

import (
	"context"
	"path/filepath"
	"testing"

	"norma/internal/db"
)

func TestSQLiteStoreUpsertAndSearch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "idx.db")
	engine, err := db.NewEngine(dbPath, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	store := NewSQLiteStore(engine)
	if err := store.Upsert("docs/a.txt", []string{"hello world", "norma project"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	hits, err := store.Search(context.Background(), "hello", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected search hits")
	}
}

func TestSQLiteStoreSearchNormalizesPunctuation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "idx.db")
	engine, err := db.NewEngine(dbPath, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	store := NewSQLiteStore(engine)
	if err := store.Upsert("docs/a.txt", []string{"plan today development tasks"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	hits, err := store.Search(context.Background(), "plan today's development tasks.", 5)
	if err != nil {
		t.Fatalf("search with punctuation: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
}

func TestSQLiteStoreSearchIgnoresSymbolOnlyQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "idx.db")
	engine, err := db.NewEngine(dbPath, false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	store := NewSQLiteStore(engine)
	hits, err := store.Search(context.Background(), "'' \" ()", 5)
	if err != nil {
		t.Fatalf("symbol only query: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits, got %d", len(hits))
	}
}
