//go:build windows

package desktop

import (
	"os"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

func platformInit() {}

// Windows 窗口：网页与窗口背景都透明，不使用系统背景材质
func applyPlatformOptions(o *options.App) {
	o.Windows = &windows.Options{
		WebviewIsTransparent:              true,
		WindowIsTranslucent:               true,
		DisableFramelessWindowDecorations: true,
		BackdropType:                      windows.None,
	}
}

// systray.Run 会阻塞到托盘退出，调用方已在独立协程中运行
func startTray(onReady func(trayBackend)) {
	systray.Run(func() { onReady(systrayBackend{}) }, func() {})
}

func quitTray() { systray.Quit() }

type systrayBackend struct{}

func (systrayBackend) SetIcon(b []byte)    { systray.SetIcon(b) }
func (systrayBackend) SetTooltip(s string) { systray.SetTooltip(s) }
func (systrayBackend) AddSeparator()       { systray.AddSeparator() }
func (systrayBackend) AddItem(title, tip string) trayItem {
	return systrayItem{systray.AddMenuItem(title, tip)}
}

type systrayItem struct{ *systray.MenuItem }

func (i systrayItem) SetTitle(s string)        { i.MenuItem.SetTitle(s) }
func (i systrayItem) Clicked() <-chan struct{} { return i.MenuItem.ClickedCh }

func loadTrayIcon() []byte {
	root, err := detectProjectRoot()
	if err != nil {
		return nil
	}
	iconPath := filepath.Join(root, "build", "windows", "icon.ico")
	data, err := os.ReadFile(iconPath)
	if err != nil {
		return nil
	}
	return data
}
