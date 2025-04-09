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

	"norma/backend/api"
	"norma/backend/core/bootstrap"
	"norma/backend/core/db"
	memorySvc "norma/backend/core/memory"
	"norma/backend/utils"
	"norma/pkg/layout"
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
	dbPath := firstNonEmpty(opts.DBPath, layout.DBPath)
	modelDir := firstNonEmpty(opts.ModelDir, layout.ModelDir)
	configPath := firstNonEmpty(opts.ConfigPath, layout.ConfigFileName)
	debugDir := firstNonEmpty(opts.DebugDir, layout.DebugSourceDir)
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("prepare db dir: %w", err)
	}

	engine, err := db.NewEngine(dbPath, false)
	if err != nil {
		return nil, fmt.Errorf("init db: %w", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	stack, err := bootstrap.NewStack(cfg, engine)
	if err != nil {
		return nil, fmt.Errorf("init runtime stack: %w", err)
	}

	svc := memorySvc.NewService(engine)
	mh := api.NewMemoryHandler(svc)
	rh := api.NewRuntimeHandler(stack.Runtime.Bus, stack.Runtime.Inspector)
	sh := api.NewSettingsHandler(configPath, stack.Runtime.Bus)
	me := api.NewMetricsHandler(stack.Tracker)
	ph := api.NewPrivacyHandler(configPath)
	th := api.NewToolPolicyHandler(configPath)
	dh := api.NewDiagnosticsHandler(configPath, modelDir, stack.Runtime.Bus, stack.Runtime.Inspector, svc)
	ch := api.NewConversationHandler(configPath, stack.Runtime.Bus, stack.Runtime.Inspector, stack.ProviderManager)
	mh2 := api.NewModelHandler(configPath, modelDir)

	mux := http.NewServeMux()
	api.RegisterHealth(mux, startedAt)
	mh.Register(mux)
	rh.Register(mux)
	sh.Register(mux)
	me.Register(mux)
	ph.Register(mux)
	th.Register(mux)
	dh.Register(mux)
	ch.Register(mux)
	mh2.Register(mux)
	registerStaticModel(mux, modelDir)
	registerStaticDebug(mux, debugDir)
	registerLive2DCore(mux, debugDir, modelDir)

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
	mux.Handle("/static/console/", http.StripPrefix("/static/console/", fs))
	log.Printf("static console route: /static/console/ -> %s", absDir)
}

func registerLive2DCore(mux *http.ServeMux, debugDir, modelDir string) {
	candidates := []string{
		filepath.Join(filepath.Dir(debugDir), "live2dcubismcore.min.js"),
		filepath.Join(modelDir, "live2dcubismcore.min.js"),
		filepath.Join(modelDir, "hiyori", "runtime", "live2dcubismcore.min.js"),
	}
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		info, statErr := os.Stat(abs)
		if statErr != nil || info.IsDir() {
			continue
		}
		mux.HandleFunc("/live2dcubismcore.min.js", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, abs)
		})
		log.Printf("live2d core route: /live2dcubismcore.min.js -> %s", abs)
		return
	}
	log.Printf("skip live2d core route: no core file found")
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
