package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"norma/internal/api"
	"norma/internal/db"
	"norma/internal/memory"
	"norma/internal/metrics"
	"norma/internal/runtime"
	"xorm.io/xorm"
)

type Options struct {
	Addr       string
	DBPath     string
	ModelDir   string
	ConfigPath string
	DebugDir   string
	StartedAt  time.Time
}

type Server struct {
	addr       string
	startedAt  time.Time
	httpServer *http.Server
	engine     *xorm.Engine
}

func New(opts Options) (*Server, error) {
	addr := firstNonEmpty(opts.Addr, ":8090")
	dbPath := firstNonEmpty(opts.DBPath, "data/norma.db")
	modelDir := firstNonEmpty(opts.ModelDir, "model")
	configPath := firstNonEmpty(opts.ConfigPath, "configs/config.yaml")
	debugDir := firstNonEmpty(opts.DebugDir, "web/debug")
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	engine, err := db.NewEngine(dbPath, false)
	if err != nil {
		return nil, fmt.Errorf("init db: %w", err)
	}

	svc := memory.NewService(engine)
	mh := api.NewMemoryHandler(svc)
	rt := runtime.New()
	tracker := metrics.NewTracker()
	quality := runtime.NewQualityController(true, 1200, 4)
	runtime.AttachLatencyObserver(rt, tracker, quality)
	rh := api.NewRuntimeHandler(rt.Bus, rt.Inspector)
	sh := api.NewSettingsHandler(configPath, rt.Bus)
	me := api.NewMetricsHandler(tracker)
	ph := api.NewPrivacyHandler(configPath)
	th := api.NewToolPolicyHandler(configPath)
	pr := api.NewProviderHandler(configPath)
	se := api.NewSetupHandler(configPath, modelDir)

	mux := http.NewServeMux()
	api.RegisterHealth(mux, startedAt)
	mh.Register(mux)
	rh.Register(mux)
	sh.Register(mux)
	me.Register(mux)
	ph.Register(mux)
	th.Register(mux)
	pr.Register(mux)
	se.Register(mux)
	registerStaticModel(mux, modelDir)
	registerStaticDebug(mux, debugDir)

	return &Server{
		addr:      addr,
		startedAt: startedAt,
		engine:    engine,
		httpServer: &http.Server{
			Addr:    addr,
			Handler: withCORS(mux),
		},
	}, nil
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("api server stopped: %v", err)
		}
	}()
	return nil
}

func (s *Server) Close(ctx context.Context) error {
	var shutdownErr error
	if s.httpServer != nil {
		shutdownErr = s.httpServer.Shutdown(ctx)
	}
	if s.engine != nil {
		if err := s.engine.Close(); err != nil && shutdownErr == nil {
			shutdownErr = err
		}
	}
	return shutdownErr
}

func (s *Server) Addr() string {
	return s.addr
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
