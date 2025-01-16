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
	vecPath := flag.String("vec", "data/vector_index.json", "vector index path")
	q := flag.String("q", "norma", "search query")
	k := flag.Int("k", 5, "top k")
	flag.Parse()

	engine, err := db.NewEngine(*dbPath, false)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer engine.Close()

	fts := index.NewSQLiteStore(engine)
	vec, err := index.NewDiskVectorStore(*vecPath)
	if err != nil {
		log.Fatalf("init vector store: %v", err)
	}
	fuser := index.NewRRFFuser(60)
	hybrid := index.NewHybridRetriever(vec, fts, fuser)

	hits, err := hybrid.Search(context.Background(), *q, *k)
	if err != nil {
		log.Fatalf("search: %v", err)
	}

	for i, h := range hits {
		fmt.Printf("%d. %s (score=%.4f)\n", i+1, h.ChunkID, h.Score)
		fmt.Printf("   %s\n", h.Content)
	}
}
