//go:build windows

package main

import (
	"errors"
	"syscall"
	"unsafe"
)

const (
	wsExLayered     uintptr = 0x00080000
	wsExTransparent uintptr = 0x00000020
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	procFindWindowW    = user32.NewProc("FindWindowW")
	procGetWindowLongW = user32.NewProc("GetWindowLongW")
	procSetWindowLongW = user32.NewProc("SetWindowLongW")
)

func setMousePassthroughByTitle(title string, enabled bool) error {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return err
	}

	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		return errors.New("window not found")
	}

	index := int32(-20) // GWL_EXSTYLE
	style, _, callErr := procGetWindowLongW.Call(hwnd, uintptr(index))
	if style == 0 && callErr != syscall.Errno(0) {
		return callErr
	}

	if enabled {
		style |= wsExTransparent | wsExLayered
	} else {
		style &^= wsExTransparent
	}

	_, _, callErr = procSetWindowLongW.Call(hwnd, uintptr(index), style)
	if callErr != syscall.Errno(0) {
		return callErr
	}

	return nil
}
