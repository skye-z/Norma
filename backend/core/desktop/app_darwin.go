//go:build darwin

package desktop

import (
	"context"
	"io/fs"
	"log"
	"os"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"norma/pkg/apiserver"
)

const appTitle = "Norma"

// macOS 上的最小桌面实现：无边框置顶透明窗口加本地 API 服务
// 鼠标穿透、托盘、全局热键依赖 Win32，暂不提供
type App struct {
	ctx       context.Context
	mu        sync.RWMutex
	apiServer *apiserver.Server
}

func NewApp() *App { return &App{} }

func Run(assets fs.FS) error {
	// 从 Finder 或 open 启动时工作目录是 /，而配置与 resource 目录都按相对路径解析
	if root, err := detectProjectRoot(); err == nil {
		if err := os.Chdir(root); err != nil {
			log.Printf("chdir to project root %s: %v", root, err)
		}
	} else {
		log.Printf("project root not found: %v", err)
	}
	app := NewApp()
	return wails.Run(&options.App{
		Title:         appTitle,
		Width:         360,
		Height:        560,
		Frameless:     true,
		AlwaysOnTop:   true,
		DisableResize: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHidden(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
	})
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.startLocalAPI(); err != nil {
		log.Printf("start embedded api server: %v", err)
	}
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
	a.stopLocalAPI()
}
