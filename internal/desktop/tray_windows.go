//go:build windows

package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/pkg/browser"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	if icon := loadTrayIcon(); len(icon) > 0 {
		systray.SetIcon(icon)
	}
	systray.SetTooltip("Norma Desktop Companion")

	menuConsole := systray.AddMenuItem("打开控制台", "在浏览器打开开发工具控制台")
	menuToggle := systray.AddMenuItem("开启完全穿透", "切换完全穿透模式")
	menuNotify := systray.AddMenuItem("通知策略：重要", "切换通知策略")

	systray.AddSeparator()
	menuShow := systray.AddMenuItem("隐藏伴侣", "显示或隐藏桌面伴侣")
	menuReload := systray.AddMenuItem("重新加载伴侣", "重新加载窗口与模型")
	systray.AddSeparator()
	menuQuit := systray.AddMenuItem("退出", "退出 Norma")

	refreshMenus := func() {
		visibleText := "隐藏伴侣"
		if !a.Visible() {
			visibleText = "显示伴侣"
		}
		menuShow.SetTitle(visibleText)
		if a.ForcePassthrough() {
			menuToggle.SetTitle("关闭完全穿透")
		} else {
			menuToggle.SetTitle("开启完全穿透")
		}
		switch a.NotificationMode() {
		case "all":
			menuNotify.SetTitle("通知策略：全部")
		case "silent":
			menuNotify.SetTitle("通知策略：静默")
		default:
			menuNotify.SetTitle("通知策略：重要")
		}
	}
	refreshMenus()

	go func() {
		for {
			shouldQuit := false
			func() {
				defer func() {
					if recover() != nil {
						// Keep tray loop alive when one menu action panics.
					}
				}()
				select {
				case <-menuShow.ClickedCh:
					a.ToggleCompanionVisibility()
					refreshMenus()
				case <-menuToggle.ClickedCh:
					a.ToggleForcePassthrough()
					refreshMenus()
				case <-menuNotify.ClickedCh:
					a.CycleNotificationMode()
					refreshMenus()
				case <-menuConsole.ClickedCh:
					go a.openExternalURL(debugURL())
				case <-menuReload.ClickedCh:
					go a.reloadCompanion()
					refreshMenus()
				case <-menuQuit.ClickedCh:
					if a.ctx != nil {
						wruntime.Quit(a.ctx)
					} else {
						a.stopTray()
					}
					shouldQuit = true
				}
			}()
			if shouldQuit {
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
	return "http://127.0.0.1:8090/static/console/"
}

func settingsURL() string {
	if os.Getenv("devserver") != "" {
		return "http://localhost:34115/"
	}
	return "http://127.0.0.1:8090/static/console/index.html"
}

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
