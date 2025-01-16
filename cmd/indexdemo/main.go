package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"norma/internal/db"
	"norma/internal/index"
)

func main() {
	dir := flag.String("dir", "docs", "directory to watch")
	dbPath := flag.String("db", "data/norma.db", "sqlite db path")
	vecPath := flag.String("vec", "data/vector_index.json", "vector index path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	engine, err := db.NewEngine(*dbPath, false)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer engine.Close()

	q := index.NewMemoryQueue(128)
	w, err := index.NewWatcher(q)
	if err != nil {
		log.Fatalf("new watcher: %v", err)
	}
	defer w.Close()

	if err := w.AddRecursive(*dir); err != nil {
		log.Fatalf("watch dir: %v", err)
	}

	fts := index.NewSQLiteStore(engine)
	vec, err := index.NewDiskVectorStore(*vecPath)
	if err != nil {
		log.Fatalf("init vector store: %v", err)
	}
	sink := index.NewCompositeSink(fts, vec)
	worker := index.NewWorkerWithSink(q, index.TextExtractor{}, sink)

	go func() {
		if err := worker.Run(ctx); err != nil {
			log.Printf("worker stopped: %v", err)
		}
	}()

	log.Printf("index demo started, watching: %s", *dir)
	if err := w.Run(ctx); err != nil {
		log.Printf("watcher stopped: %v", err)
	}
}
