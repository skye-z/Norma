//go:build windows

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/pkg/browser"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

var trayQuitOnce sync.Once

func (a *App) runTray() {
	systray.Run(func() {
		a.initTrayMenu()
	}, func() {})
}

func (a *App) stopTray() {
	trayQuitOnce.Do(func() {
		systray.Quit()
	})
}

func (a *App) initTrayMenu() {
	if len(trayIcon) > 0 {
		systray.SetIcon(trayIcon)
	}
	systray.SetTooltip("Norma Desktop Companion")

	menuToggle := systray.AddMenuItem("开启完全穿透", "切换完全穿透模式")
	menuStatus := systray.AddMenuItem("状态: 普通交互", "当前穿透状态")
	menuStatus.Disable()

	systray.AddSeparator()
	menuDebug := systray.AddMenuItem("调试", "在浏览器打开调试面板")
	menuSettings := systray.AddMenuItem("设置", "在浏览器打开设置页面")
	menuReload := systray.AddMenuItem("重新加载", "重新加载窗口与模型")
	systray.AddSeparator()
	menuQuit := systray.AddMenuItem("退出", "退出 Norma")

	refreshToggleMenu := func(enabled bool) {
		if enabled {
			menuToggle.SetTitle("关闭完全穿透")
			menuStatus.SetTitle("状态: 完全穿透")
		} else {
			menuToggle.SetTitle("开启完全穿透")
			menuStatus.SetTitle("状态: 普通交互")
		}
	}
	refreshToggleMenu(a.isForcePassthrough())

	go func() {
		defer func() {
			if recover() != nil {
				// Avoid exiting tray event loop due to unexpected runtime panic.
			}
		}()
		for {
			select {
			case <-menuToggle.ClickedCh:
				enabled := a.ToggleForcePassthrough()
				refreshToggleMenu(enabled)
			case <-menuDebug.ClickedCh:
				go a.openExternalURL(debugURL())
			case <-menuSettings.ClickedCh:
				go a.openExternalURL(settingsURL())
			case <-menuReload.ClickedCh:
				go a.reloadCompanion()
			case <-menuQuit.ClickedCh:
				if a.ctx != nil {
					wruntime.Quit(a.ctx)
				} else {
					a.stopTray()
				}
				return
			}
		}
	}()
}

func (a *App) reloadCompanion() {
	a.applyForcePassthrough(false)
	_ = publishRuntimeEvent("live2d.reload", map[string]any{"source": "tray"})
	if a.ctx != nil {
		wruntime.WindowShow(a.ctx)
		time.AfterFunc(350*time.Millisecond, func() {
			wruntime.WindowReloadApp(a.ctx)
		})
	}
}

func publishRuntimeEvent(name string, payload any) error {
	body, err := json.Marshal(map[string]any{"name": name, "payload": payload})
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Post("http://127.0.0.1:8090/runtime/publish", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (a *App) openExternalURL(url string) {
	if strings.TrimSpace(url) == "" {
		return
	}
	if a.ctx != nil {
		wruntime.BrowserOpenURL(a.ctx, url)
		return
	}
	_ = browser.OpenURL(url)
}

func debugURL() string {
	if os.Getenv("devserver") != "" {
		return "http://localhost:34115/?debug=1"
	}
	return "http://127.0.0.1:8090/static/debug/index.html"
}

func settingsURL() string {
	if os.Getenv("devserver") != "" {
		return "http://localhost:34115/?settings=1"
	}
	return "http://127.0.0.1:8090/static/debug/settings.html"
}
