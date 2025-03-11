//go:build windows

package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx              context.Context
	mu               sync.RWMutex
	mousePassthrough bool
	forcePassthrough bool
	alwaysOnTop      bool
	notificationsOn  bool
	notificationMode string
	visible          bool
	eyeLastX         float64
	eyeLastY         float64
	eyeLastAt        time.Time
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{alwaysOnTop: true, notificationsOn: true, notificationMode: "important", visible: true}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.autoPassthroughLoop()
	go a.ensureWindowStyleLoop()
	go a.runTray()
	go a.runGlobalHotkeys(ctx)
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
	a.stopTray()
}

func (a *App) beforeClose(ctx context.Context) bool {
	_ = ctx
	return false
}

func (a *App) ensureWindowStyleLoop() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(8 * time.Second)

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			hwnd, err := findWindowByTitle(appTitle)
			if err == nil && hwnd != 0 {
				_ = hideWindowFromTaskbarByHandle(hwnd)
				_ = setWindowTopmostByHandle(hwnd, a.AlwaysOnTop())
				return
			}
			if time.Now().After(deadline) {
				return
			}
		}
	}
}

func (a *App) autoPassthroughLoop() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			hwnd, err := findWindowByTitle(appTitle)
			if err != nil || hwnd == 0 {
				continue
			}

			enablePassthrough := true
			if !a.isForcePassthrough() {
				if inside, e := cursorInModelInteractRect(hwnd); e == nil && inside {
					enablePassthrough = false
				}
			}

			if enablePassthrough != a.MousePassthrough() {
				_ = setMousePassthroughByHandle(hwnd, enablePassthrough)
				a.setMousePassthrough(enablePassthrough)
			}

			a.publishEyeIfNeeded(hwnd)
		}
	}
}

func (a *App) publishEyeIfNeeded(hwnd uintptr) {
	x, y, err := cursorNormalizedInWindow(hwnd)
	if err != nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	if now.Sub(a.eyeLastAt) < 70*time.Millisecond {
		return
	}

	if !a.eyeLastAt.IsZero() {
		dx := math.Abs(x - a.eyeLastX)
		dy := math.Abs(y - a.eyeLastY)
		if dx < 0.025 && dy < 0.025 && now.Sub(a.eyeLastAt) < 220*time.Millisecond {
			return
		}
		x = a.eyeLastX + (x-a.eyeLastX)*0.35
		y = a.eyeLastY + (y-a.eyeLastY)*0.35
	}

	a.eyeLastX = x
	a.eyeLastY = y
	a.eyeLastAt = now

	_ = publishRuntimeEvent("live2d.eye", map[string]any{"x": x, "y": y})
}

func (a *App) setMousePassthrough(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mousePassthrough = enabled
}

func (a *App) setForcePassthrough(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.forcePassthrough = enabled
}

func (a *App) setAlwaysOnTop(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.alwaysOnTop = enabled
}

func (a *App) setNotificationMode(mode string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.notificationMode = normalizeNotificationMode(mode)
	a.notificationsOn = a.notificationMode != "silent"
}

func (a *App) setVisible(visible bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.visible = visible
}

func (a *App) isForcePassthrough() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.forcePassthrough
}

func (a *App) applyForcePassthrough(enabled bool) bool {
	a.setForcePassthrough(enabled)
	if err := setMousePassthroughByTitle(appTitle, enabled); err == nil {
		a.setMousePassthrough(enabled)
	}
	return a.isForcePassthrough()
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// SetMousePassthrough toggles click-through mode for the floating window.
func (a *App) SetMousePassthrough(enabled bool) string {
	if err := setMousePassthroughByTitle(appTitle, enabled); err != nil {
		return "error: " + err.Error()
	}
	a.setMousePassthrough(enabled)
	return "ok"
}

func (a *App) MousePassthrough() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mousePassthrough
}

func (a *App) SetForcePassthrough(enabled bool) string {
	a.applyForcePassthrough(enabled)
	return "ok"
}

func (a *App) ForcePassthrough() bool {
	return a.isForcePassthrough()
}

func (a *App) ToggleForcePassthrough() bool {
	return a.applyForcePassthrough(!a.isForcePassthrough())
}

func (a *App) AlwaysOnTop() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.alwaysOnTop
}

func (a *App) SetAlwaysOnTop(enabled bool) string {
	if err := setWindowTopmostByTitle(appTitle, enabled); err != nil {
		return "error: " + err.Error()
	}
	a.setAlwaysOnTop(enabled)
	return "ok"
}

func (a *App) ToggleAlwaysOnTop() bool {
	enabled := !a.AlwaysOnTop()
	_ = a.SetAlwaysOnTop(enabled)
	return a.AlwaysOnTop()
}

func (a *App) NotificationsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.notificationsOn
}

func (a *App) SetNotificationsEnabled(enabled bool) string {
	if enabled {
		a.setNotificationMode("all")
	} else {
		a.setNotificationMode("silent")
	}
	return "ok"
}

func (a *App) NotificationMode() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return normalizeNotificationMode(a.notificationMode)
}

func (a *App) SetNotificationMode(mode string) string {
	a.setNotificationMode(mode)
	return a.NotificationMode()
}

func (a *App) CycleNotificationMode() string {
	switch a.NotificationMode() {
	case "all":
		a.setNotificationMode("important")
	case "important":
		a.setNotificationMode("silent")
	default:
		a.setNotificationMode("all")
	}
	return a.NotificationMode()
}

func (a *App) Notify(title, body string) string {
	if a.NotificationMode() == "silent" {
		return "disabled"
	}
	if strings.TrimSpace(title) == "" {
		title = "Norma"
	}
	if strings.TrimSpace(body) == "" {
		body = "收到新的桌面提醒。"
	}
	if err := publishRuntimeEvent("debug.notice", map[string]any{"text": title + "：" + body}); err != nil {
		return "error: " + err.Error()
	}
	return a.NotificationMode()
}

func (a *App) DesktopState() map[string]any {
	return map[string]any{
		"always_on_top":     a.AlwaysOnTop(),
		"mouse_passthrough": a.MousePassthrough(),
		"force_passthrough": a.ForcePassthrough(),
		"notification_mode": a.NotificationMode(),
		"notifications_on":  a.NotificationsEnabled(),
		"visible":           a.Visible(),
	}
}

func (a *App) Visible() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.visible
}

func (a *App) ShowCompanion() string {
	if a.ctx != nil {
		wruntime.WindowShow(a.ctx)
	}
	a.setVisible(true)
	return "ok"
}

func (a *App) HideCompanion() string {
	if a.ctx != nil {
		wruntime.WindowHide(a.ctx)
	}
	a.setVisible(false)
	return "ok"
}

func (a *App) ToggleCompanionVisibility() bool {
	if a.Visible() {
		_ = a.HideCompanion()
	} else {
		_ = a.ShowCompanion()
	}
	return a.Visible()
}

func (a *App) OpenDebugPanel() string {
	go a.openExternalURL(debugURL())
	return "ok"
}

func (a *App) OpenSettingsPanel() string {
	go a.openExternalURL(settingsURL())
	return "ok"
}

func (a *App) OpenOnboardingPanel() string {
	go a.openExternalURL(onboardingPanelURL())
	return "ok"
}

func (a *App) ReloadCompanion() string {
	go a.reloadCompanion()
	return "ok"
}

func onboardingPanelURL() string {
	if os.Getenv("devserver") != "" {
		return "http://localhost:34115/?onboarding=1"
	}
	return "http://127.0.0.1:8090/static/debug/onboarding.html"
}

func normalizeNotificationMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "all":
		return "all"
	case "important":
		return "important"
	case "silent", "off":
		return "silent"
	default:
		return "important"
	}
}
