// Package apiserver 提供 Norma 应用程序的 HTTP API 服务器。
// 该服务器提供内存管理、运行时状态、设置、指标、隐私、工具策略、
// 诊断、对话和模型相关的REST API端点。
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

// Options 定义 API 服务器的启动选项
type Options struct {
	Addr       string    // 服务器监听地址，默认为 :8090
	DBPath     string    // 数据库文件路径
	ModelDir   string    // 模型文件目录
	ConfigPath string    // 配置文件路径
	DebugDir   string    // 调试文件目录
	StartedAt  time.Time // 服务器启动时间
}

// Server 是 API 服务器的结构体
type Server struct {
	addr       string       // 监听地址
	startedAt  time.Time    // 启动时间
	httpServer *http.Server // HTTP 服务器实例
	engine     *xorm.Engine // 数据库引擎
}

// New 创建一个新的 API 服务器实例。
// 该函数会初始化数据库、配置、运行时堆栈，并注册所有API处理器。
//
// 参数：
//   - opts: 服务器启动选项
//
// 返回：
//   - *Server: 服务器实例
//   - error: 初始化过程中的错误
func New(opts Options) (*Server, error) {
	// 设置默认值
	addr := firstNonEmpty(opts.Addr, ":8090")
	dbPath := firstNonEmpty(opts.DBPath, layout.DBPath)
	modelDir := firstNonEmpty(opts.ModelDir, layout.ModelDir)
	configPath := firstNonEmpty(opts.ConfigPath, layout.ConfigFileName)
	debugDir := firstNonEmpty(opts.DebugDir, layout.DebugSourceDir)
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	// 确保数据库目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("prepare db dir: %w", err)
	}

	// 初始化数据库引擎
	engine, err := db.NewEngine(dbPath, false)
	if err != nil {
		return nil, fmt.Errorf("init db: %w", err)
	}
	// 加载应用程序配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	// 创建运行时堆栈
	stack, err := bootstrap.NewStack(cfg, engine)
	if err != nil {
		return nil, fmt.Errorf("init runtime stack: %w", err)
	}

	// 创建并注册各个API处理器
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

	// 创建HTTP多路复用器并注册路由
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
	// 注册静态文件服务
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

// Start 启动 API 服务器。
// 该函数会在后台启动 HTTP 服务器监听指定地址。
//
// 返回：
//   - error: 监听失败时的错误
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	// 在后台启动服务器
	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("api server stopped: %v", err)
		}
	}()
	return nil
}

// Close 优雅地关闭 API 服务器。
// 该函数会先关闭 HTTP 服务器，然后关闭数据库连接。
//
// 参数：
//   - ctx: 关闭超时上下文
//
// 返回：
//   - error: 关闭过程中的错误
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

// Addr 返回服务器的监听地址。
//
// 返回：
//   - string: 服务器地址
func (s *Server) Addr() string {
	return s.addr
}

// registerStaticModel 注册模型目录的静态文件服务。
// 将 modelDir 目录下的文件通过 /static/model/ 路径提供访问。
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

// registerStaticDebug 注册调试目录的静态文件服务。
// 将 debugDir 目录下的文件通过 /static/console/ 路径提供访问。
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
	// 控制台随版本频繁变化，要求浏览器每次向服务端校验，否则会长期使用缓存里的旧脚本
	mux.Handle("/static/console/", http.StripPrefix("/static/console/", noCache(fs)))
	log.Printf("static console route: /static/console/ -> %s", absDir)
}

// registerLive2DCore 注册 Live2D Cubism Core 库的静态文件服务。
// 该函数会尝试在多个常见位置查找 live2dcubismcore.min.js 文件。
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

// withCORS 是一个中间件，为响应添加 CORS 头支持跨域请求。
// 允许所有来源的请求，支持常见的HTTP方法和头部。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		// 处理预检请求
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// firstNonEmpty 返回第一个非空字符串。
// 如果所有字符串都为空，则返回空字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// noCache 让浏览器每次使用缓存前都向服务端校验；文件未变化时仍返回 304
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		next.ServeHTTP(w, r)
	})
}
