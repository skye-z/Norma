//go:build windows

package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// App struct
type App struct {
	ctx              context.Context
	mu               sync.RWMutex
	mousePassthrough bool
	forcePassthrough bool
	eyeLastX         float64
	eyeLastY         float64
	eyeLastAt        time.Time
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.autoPassthroughLoop()
	go a.ensureWindowStyleLoop()
	go a.runTray()
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
	if now.Sub(a.eyeLastAt) < 80*time.Millisecond {
		return
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
