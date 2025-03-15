package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"norma/pkg/apiserver"
)

func main() {
	addr := flag.String("addr", ":8090", "http listen address")
	dbPath := flag.String("db", "resource/data/norma.db", "sqlite db path")
	modelDir := flag.String("model-dir", "resource/model", "live2d model directory")
	configPath := flag.String("config", "config.yaml", "runtime config path")
	flag.Parse()

	srv, err := apiserver.New(apiserver.Options{
		Addr:       *addr,
		DBPath:     *dbPath,
		ModelDir:   *modelDir,
		ConfigPath: *configPath,
		DebugDir:   "web/debug",
		StartedAt:  time.Now(),
	})
	if err != nil {
		log.Fatalf("init api server: %v", err)
	}
	if err := srv.Start(); err != nil {
		log.Fatalf("start api server: %v", err)
	}
	log.Printf("api server listening on %s", srv.Addr())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Close(shutdownCtx); err != nil {
		log.Printf("close api server: %v", err)
	}
}
