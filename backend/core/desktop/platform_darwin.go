//go:build darwin

package desktop

import (
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

// 从 Finder 或 open 启动时工作目录是 /，而配置与 resource 目录都按相对路径解析
func platformInit() {
	root, err := detectProjectRoot()
	if err != nil {
		log.Printf("project root not found: %v", err)
		return
	}
	if err := os.Chdir(root); err != nil {
		log.Printf("chdir to project root %s: %v", root, err)
	}
}

// macOS 窗口：网页背景透明，且不叠加系统毛玻璃（WindowIsTranslucent 会产生灰色模糊层）
func applyPlatformOptions(o *options.App) {
	o.Mac = &mac.Options{
		TitleBar:             mac.TitleBarHidden(),
		WebviewIsTransparent: true,
		WindowIsTranslucent:  false,
	}
}

func loadTrayIcon() []byte {
	root, err := detectProjectRoot()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, "build", "appicon.png"))
	if err != nil {
		return nil
	}
	return data
}
