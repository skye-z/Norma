package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"norma/internal/db"
	"norma/internal/index"
)

func main() {
	dbPath := flag.String("db", "data/norma.db", "sqlite db path")
	q := flag.String("q", "norma", "search query")
	k := flag.Int("k", 5, "top k")
	flag.Parse()

	engine, err := db.NewEngine(*dbPath, false)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer engine.Close()

	store := index.NewSQLiteStore(engine)
	hits, err := store.Search(context.Background(), *q, *k)
	if err != nil {
		log.Fatalf("search: %v", err)
	}

	for i, h := range hits {
		fmt.Printf("%d. %s\n", i+1, h.ChunkID)
		fmt.Printf("   %s\n", h.Content)
	}
}
