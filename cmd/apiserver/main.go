package main

import (
	"flag"
	"log"
	"net/http"

	"norma/internal/api"
	"norma/internal/db"
	"norma/internal/memory"
	"norma/internal/runtime"
)

func main() {
	addr := flag.String("addr", ":8090", "http listen address")
	dbPath := flag.String("db", "data/norma.db", "sqlite db path")
	flag.Parse()

	engine, err := db.NewEngine(*dbPath, false)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer engine.Close()

	svc := memory.NewService(engine)
	mh := api.NewMemoryHandler(svc)
	rt := runtime.New()
	rh := api.NewRuntimeHandler(rt.Bus)

	mux := http.NewServeMux()
	mh.Register(mux)
	rh.Register(mux)

	log.Printf("api server listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
