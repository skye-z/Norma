//go:build darwin

package desktop

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Carbon -framework Cocoa
#include "hotkeys_darwin.h"
*/
import "C"

import (
	"context"
	"sync"
)

var (
	hotkeyMu  sync.Mutex
	hotkeyApp *App
)

// Option+T/P/N/H，与 Windows 版 Alt+T/P/N/H 的动作一致
func (a *App) runGlobalHotkeys(ctx context.Context) {
	hotkeyMu.Lock()
	hotkeyApp = a
	hotkeyMu.Unlock()
	C.normaRegisterHotkeys()
	go func() {
		<-ctx.Done()
		C.normaUnregisterHotkeys()
		hotkeyMu.Lock()
		hotkeyApp = nil
		hotkeyMu.Unlock()
	}()
}

//export normaHotkeyPressed
func normaHotkeyPressed(id C.int) {
	hotkeyMu.Lock()
	a := hotkeyApp
	hotkeyMu.Unlock()
	if a == nil {
		return
	}
	switch int(id) {
	case 1:
		a.ToggleAlwaysOnTop()
	case 2:
		a.ToggleForcePassthrough()
	case 3:
		a.CycleNotificationMode()
	case 4:
		_ = publishRuntimeEvent("debug.notice", map[string]any{"text": "快捷键：Option+H 由主界面控制面板处理。"})
	}
}
