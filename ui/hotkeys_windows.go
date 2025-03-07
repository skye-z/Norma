//go:build windows

package main

import (
	"context"
	"syscall"
	"unsafe"
)

const (
	hotkeyAlt = 0x0001
	wmHotkey  = 0x0312
	wmQuit    = 0x0012
)

type msg struct {
	hWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

var (
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procPostThreadMessage  = user32.NewProc("PostThreadMessageW")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")
)

func (a *App) runGlobalHotkeys(ctx context.Context) {
	threadID, done := startHotkeyLoop(a)
	if threadID == 0 {
		return
	}
	go func() {
		<-ctx.Done()
		_, _, _ = procPostThreadMessage.Call(threadID, wmQuit, 0, 0)
		<-done
	}()
}

func startHotkeyLoop(a *App) (uintptr, chan struct{}) {
	threadCh := make(chan uintptr, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		threadID, _, _ := procGetCurrentThreadID.Call()
		threadCh <- threadID
		bindings := []struct {
			id  uintptr
			key uintptr
		}{
			{id: 1, key: 'T'},
			{id: 2, key: 'P'},
			{id: 3, key: 'N'},
			{id: 4, key: 'H'},
		}
		registered := make([]uintptr, 0, len(bindings))
		for _, item := range bindings {
			r1, _, _ := procRegisterHotKey.Call(0, item.id, hotkeyAlt, item.key)
			if r1 != 0 {
				registered = append(registered, item.id)
			}
		}
		defer func() {
			for _, id := range registered {
				_, _, _ = procUnregisterHotKey.Call(0, id)
			}
		}()

		var m msg
		for {
			r1, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r1) <= 0 {
				return
			}
			if m.Message == wmQuit {
				return
			}
			if m.Message == wmHotkey {
				switch m.WParam {
				case 1:
					a.ToggleAlwaysOnTop()
				case 2:
					a.ToggleForcePassthrough()
				case 3:
					a.CycleNotificationMode()
				case 4:
					_ = publishRuntimeEvent("debug.notice", map[string]any{"text": "快捷键：Alt+H 由主界面控制面板处理。"})
				}
				continue
			}
			_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	return <-threadCh, done
}
