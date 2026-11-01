//go:build windows || darwin

package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/browser"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"norma/backend/core/audio"
	utils "norma/backend/utils"
	"norma/pkg/layout"
)

var trayQuitOnce sync.Once

// 托盘后端：Windows 用 systray，macOS 用自己的 NSStatusItem（systray 在 macOS 上的 AppDelegate 与 Wails 冲突）
type trayItem interface {
	SetTitle(string)
	Clicked() <-chan struct{}
}

type trayBackend interface {
	SetIcon([]byte)
	SetTooltip(string)
	AddItem(title, tooltip string) trayItem
	AddSeparator()
}

func (a *App) runTray() {
	startTray(func(tb trayBackend) {
		a.initTrayMenu(tb)
	})
}

func (a *App) stopTray() {
	trayQuitOnce.Do(func() {
		quitTray()
	})
}

func (a *App) initTrayMenu(tb trayBackend) {
	if icon := loadTrayIcon(); len(icon) > 0 {
		tb.SetIcon(icon)
	}
	tb.SetTooltip("Norma Desktop Companion")

	menuConsole := tb.AddItem("打开控制台", "在浏览器打开开发工具控制台")
	menuToggle := tb.AddItem("开启完全穿透", "切换完全穿透模式")
	menuWakeMode := tb.AddItem("唤醒模式：自动", "切换自动唤醒或手动唤醒")

	tb.AddSeparator()
	menuShow := tb.AddItem("隐藏伴侣", "显示或隐藏桌面伴侣")
	menuReload := tb.AddItem("重新加载伴侣", "重新加载窗口与模型")
	tb.AddSeparator()
	menuQuit := tb.AddItem("退出", "退出 Norma")

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
		if autoWakeEnabled() {
			menuWakeMode.SetTitle("唤醒模式：自动")
		} else {
			menuWakeMode.SetTitle("唤醒模式：手动")
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
				case <-menuShow.Clicked():
					a.ToggleCompanionVisibility()
					refreshMenus()
				case <-menuToggle.Clicked():
					a.ToggleForcePassthrough()
					refreshMenus()
				case <-menuWakeMode.Clicked():
					nextAutoWake, err := toggleWakeMode()
					if err != nil {
						_ = publishRuntimeEvent("debug.notice", map[string]any{"text": "唤醒模式切换失败: " + err.Error()})
					} else {
						_ = publishRuntimeEvent("assistant.say", map[string]any{"text": wakeModeAnnouncement(nextAutoWake), "interrupt": true, "source": "tray"})
						_ = publishRuntimeEvent("runtime.utils.updated", map[string]any{"path": layout.ConfigFileName, "source": "tray"})
					}
					refreshMenus()
				case <-menuConsole.Clicked():
					go a.openExternalURL(debugURL())
				case <-menuReload.Clicked():
					go a.reloadCompanion()
					refreshMenus()
				case <-menuQuit.Clicked():
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

func autoWakeEnabled() bool {
	cfg, err := utils.Load(layout.ConfigFileName)
	if err != nil {
		return true
	}
	return cfg.Audio.EnableWakeWord && strings.TrimSpace(cfg.Audio.WakeMode) == audio.WakeModeWakeWordVAD
}

func wakeModeAnnouncement(auto bool) string {
	if auto {
		return "我一直都在，有事叫我哦"
	}
	return "我先休息啦，有事按我头上的按钮"
}

func toggleWakeMode() (bool, error) {
	cfg, err := utils.Load(layout.ConfigFileName)
	if err != nil {
		return false, err
	}
	if cfg.Audio.EnableWakeWord && strings.TrimSpace(cfg.Audio.WakeMode) == audio.WakeModeWakeWordVAD {
		cfg.Audio.EnableWakeWord = false
		cfg.Audio.WakeMode = audio.WakeModeVADOnly
	} else {
		cfg.Audio.EnableWakeWord = true
		cfg.Audio.WakeMode = audio.WakeModeWakeWordVAD
		if strings.TrimSpace(cfg.Audio.WakeKeyword) == "" {
			cfg.Audio.WakeKeyword = "诺玛"
		}
	}
	if err := utils.Save(layout.ConfigFileName, cfg); err != nil {
		return false, err
	}
	return cfg.Audio.EnableWakeWord && strings.TrimSpace(cfg.Audio.WakeMode) == audio.WakeModeWakeWordVAD, nil
}
