//go:build darwin

package desktop

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"

import (
	"sync"
	"unsafe"
)

var (
	trayMu    sync.Mutex
	trayNext  = 1
	trayItems = map[int]*darwinTrayItem{}
)

type darwinTrayItem struct {
	tag int
	ch  chan struct{}
}

func (i *darwinTrayItem) SetTitle(s string) {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	C.normaTraySetTitle(C.int(i.tag), cs)
}

func (i *darwinTrayItem) Clicked() <-chan struct{} { return i.ch }

type darwinTray struct{}

func (darwinTray) SetIcon(b []byte) {
	if len(b) == 0 {
		return
	}
	C.normaTraySetIcon(unsafe.Pointer(&b[0]), C.int(len(b)))
}

func (darwinTray) SetTooltip(string) {} // 提示文字在创建状态栏项目时已设置

func (darwinTray) AddSeparator() { C.normaTrayAddSeparator() }

func (darwinTray) AddItem(title, _ string) trayItem {
	trayMu.Lock()
	tag := trayNext
	trayNext++
	it := &darwinTrayItem{tag: tag, ch: make(chan struct{}, 1)}
	trayItems[tag] = it
	trayMu.Unlock()
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	C.normaTrayAddItem(cs, C.int(tag))
	return it
}

// Wails 已在运行 Cocoa 主循环，这里只创建状态栏项目并立即回调
func startTray(onReady func(trayBackend)) {
	tip := C.CString("Norma Desktop Companion")
	defer C.free(unsafe.Pointer(tip))
	C.normaTrayInit(tip)
	onReady(darwinTray{})
}

func quitTray() {}

//export normaTrayClicked
func normaTrayClicked(tag C.int) {
	trayMu.Lock()
	it := trayItems[int(tag)]
	trayMu.Unlock()
	if it == nil {
		return
	}
	select {
	case it.ch <- struct{}{}:
	default: // 上一次点击还没被处理时丢弃，避免阻塞主线程
	}
}
