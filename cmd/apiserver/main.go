package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"norma/internal/api"
	"norma/internal/db"
	"norma/internal/memory"
	"norma/internal/metrics"
	"norma/internal/runtime"
)

func main() {
	addr := flag.String("addr", ":8090", "http listen address")
	dbPath := flag.String("db", "data/norma.db", "sqlite db path")
	modelDir := flag.String("model-dir", "model", "live2d model directory")
	configPath := flag.String("config", "configs/config.yaml", "runtime config path")
	flag.Parse()

	engine, err := db.NewEngine(*dbPath, false)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer engine.Close()

	svc := memory.NewService(engine)
	mh := api.NewMemoryHandler(svc)
	rt := runtime.New()
	tracker := metrics.NewTracker()
	quality := runtime.NewQualityController(true, 1200, 4)
	runtime.AttachLatencyObserver(rt, tracker, quality)
	rh := api.NewRuntimeHandler(rt.Bus, rt.Inspector)
	sh := api.NewSettingsHandler(*configPath, rt.Bus)
	me := api.NewMetricsHandler(tracker)
	ph := api.NewPrivacyHandler(*configPath)
	th := api.NewToolPolicyHandler(*configPath)

	mux := http.NewServeMux()
	api.RegisterHealth(mux, time.Now())
	mh.Register(mux)
	rh.Register(mux)
	sh.Register(mux)
	me.Register(mux)
	ph.Register(mux)
	th.Register(mux)
	registerStaticModel(mux, *modelDir)
	registerStaticDebug(mux, "web/debug")

	log.Printf("api server listening on %s", *addr)
	if err := http.ListenAndServe(*addr, withCORS(mux)); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func registerStaticModel(mux *http.ServeMux, modelDir string) {
	absDir, err := filepath.Abs(modelDir)
	if err != nil {
		log.Printf("skip static model route: resolve %q failed: %v", modelDir, err)
		return
	}
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		log.Printf("skip static model route: invalid dir %q", absDir)
		return
	}
	fs := http.FileServer(http.Dir(absDir))
	mux.Handle("/static/model/", http.StripPrefix("/static/model/", fs))
	log.Printf("static model route: /static/model/ -> %s", absDir)
}

func registerStaticDebug(mux *http.ServeMux, debugDir string) {
	absDir, err := filepath.Abs(debugDir)
	if err != nil {
		log.Printf("skip static debug route: resolve %q failed: %v", debugDir, err)
		return
	}
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		log.Printf("skip static debug route: invalid dir %q", absDir)
		return
	}
	fs := http.FileServer(http.Dir(absDir))
	mux.Handle("/static/debug/", http.StripPrefix("/static/debug/", fs))
	log.Printf("static debug route: /static/debug/ -> %s", absDir)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
