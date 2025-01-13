package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"norma/internal/index"
)

func main() {
	dir := flag.String("dir", "docs", "directory to watch")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	q := index.NewMemoryQueue(128)
	w, err := index.NewWatcher(q)
	if err != nil {
		log.Fatalf("new watcher: %v", err)
	}
	defer w.Close()

	if err := w.AddRecursive(*dir); err != nil {
		log.Fatalf("watch dir: %v", err)
	}

	worker := index.NewWorker(q, index.TextExtractor{})

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
