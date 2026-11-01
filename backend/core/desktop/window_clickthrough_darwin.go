//go:build darwin

package desktop

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "window_darwin.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// macOS 没有 HWND，用 1 表示“窗口存在”，其余函数按标题查找窗口
func findWindowByTitle(title string) (uintptr, error) {
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	if C.normaWindowExists(cs) == 0 {
		return 0, errors.New("window not found")
	}
	return 1, nil
}

func setMousePassthroughByTitle(title string, enabled bool) error {
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	if C.normaWindowExists(cs) == 0 {
		return errors.New("window not found")
	}
	C.normaSetIgnoreMouse(cs, boolInt(enabled))
	return nil
}

func setMousePassthroughByHandle(_ uintptr, enabled bool) error {
	return setMousePassthroughByTitle(appTitle, enabled)
}

func setWindowTopmostByTitle(title string, enabled bool) error {
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	if C.normaWindowExists(cs) == 0 {
		return errors.New("window not found")
	}
	C.normaSetFloating(cs, boolInt(enabled))
	return nil
}

func setWindowTopmostByHandle(_ uintptr, enabled bool) error {
	return setWindowTopmostByTitle(appTitle, enabled)
}

func hideWindowFromTaskbarByHandle(_ uintptr) error {
	C.normaHideFromDock()
	return nil
}

func cursorInteract() (inside bool, x, y float64, err error) {
	cs := C.CString(appTitle)
	defer C.free(unsafe.Pointer(cs))
	var in C.int
	var nx, ny C.double
	if C.normaCursorInteract(cs, &in, &nx, &ny) == 0 {
		return false, 0, 0, errors.New("window not found")
	}
	return in != 0, float64(nx), float64(ny), nil
}

func cursorInModelInteractRect(_ uintptr) (bool, error) {
	inside, _, _, err := cursorInteract()
	return inside, err
}

func cursorNormalizedInWindow(_ uintptr) (float64, float64, error) {
	_, x, y, err := cursorInteract()
	return x, y, err
}

func boolInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
