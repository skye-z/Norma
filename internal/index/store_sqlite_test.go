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
