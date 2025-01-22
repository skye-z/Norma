package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"norma/internal/db"
	"norma/internal/memory"
)

func main() {
	dbPath := flag.String("db", "data/norma.db", "sqlite db path")
	action := flag.String("action", "search", "add|search|delete")
	kind := flag.String("kind", "fact", "fact|preference|event")
	text := flag.String("text", "", "memory content or query")
	importance := flag.Int("importance", 5, "0-10")
	id := flag.Int64("id", 0, "memory id for delete")
	limit := flag.Int("limit", 10, "search limit")
	flag.Parse()

	engine, err := db.NewEngine(*dbPath, false)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer engine.Close()

	svc := memory.NewService(engine)
	ctx := context.Background()

	switch *action {
	case "add":
		m, err := svc.Add(ctx, *kind, *text, *importance)
		if err != nil {
			log.Fatalf("add: %v", err)
		}
		if m == nil {
			log.Fatalf("add: empty content")
		}
		fmt.Printf("added: id=%d kind=%s importance=%d content=%s\n", m.ID, m.Kind, m.Importance, m.Content)
	case "delete":
		if *id <= 0 {
			log.Fatalf("delete requires -id")
		}
		if err := svc.Delete(ctx, *id); err != nil {
			log.Fatalf("delete: %v", err)
		}
		fmt.Printf("deleted: id=%d\n", *id)
	default:
		hits, err := svc.Search(ctx, *text, *limit)
		if err != nil {
			log.Fatalf("search: %v", err)
		}
		for i, m := range hits {
			fmt.Printf("%d. id=%d kind=%s imp=%d content=%s\n", i+1, m.ID, m.Kind, m.Importance, m.Content)
		}
	}
}
