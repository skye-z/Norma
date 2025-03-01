//go:build windows

package main

import (
	"errors"
	"syscall"
	"unsafe"
)

const (
	hwndTopmost     uintptr = ^uintptr(0)
	hwndNoTopmost   uintptr = ^uintptr(1)
	wsExLayered     uintptr = 0x00080000
	wsExTransparent uintptr = 0x00000020
	wsExAppWindow   uintptr = 0x00040000
	wsExToolWindow  uintptr = 0x00000080

	swpNoSize       uintptr = 0x0001
	swpNoMove       uintptr = 0x0002
	swpNoZOrder     uintptr = 0x0004
	swpFrameChanged uintptr = 0x0020
)

var gwlExStyle = int32(-20)

type point struct {
	X int32
	Y int32
}

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	procFindWindowW    = user32.NewProc("FindWindowW")
	procGetWindowLongW = user32.NewProc("GetWindowLongW")
	procSetWindowLongW = user32.NewProc("SetWindowLongW")
	procGetCursorPos   = user32.NewProc("GetCursorPos")
	procGetWindowRect  = user32.NewProc("GetWindowRect")
	procSetWindowPos   = user32.NewProc("SetWindowPos")
)

func findWindowByTitle(title string) (uintptr, error) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0, err
	}
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		return 0, errors.New("window not found")
	}
	return hwnd, nil
}

func setMousePassthroughByTitle(title string, enabled bool) error {
	hwnd, err := findWindowByTitle(title)
	if err != nil {
		return err
	}
	return setMousePassthroughByHandle(hwnd, enabled)
}

func setMousePassthroughByHandle(hwnd uintptr, enabled bool) error {
	if hwnd == 0 {
		return errors.New("window handle invalid")
	}

	style, _, callErr := procGetWindowLongW.Call(hwnd, uintptr(int32(gwlExStyle)))
	if style == 0 && callErr != syscall.Errno(0) {
		return callErr
	}

	if enabled {
		style |= wsExTransparent | wsExLayered
	} else {
		style &^= wsExTransparent
	}

	_, _, callErr = procSetWindowLongW.Call(hwnd, uintptr(int32(gwlExStyle)), style)
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return nil
}

func setWindowTopmostByTitle(title string, enabled bool) error {
	hwnd, err := findWindowByTitle(title)
	if err != nil {
		return err
	}
	return setWindowTopmostByHandle(hwnd, enabled)
}

func setWindowTopmostByHandle(hwnd uintptr, enabled bool) error {
	if hwnd == 0 {
		return errors.New("window handle invalid")
	}
	insertAfter := hwndNoTopmost
	if enabled {
		insertAfter = hwndTopmost
	}
	_, _, callErr := procSetWindowPos.Call(hwnd, insertAfter, 0, 0, 0, 0, swpNoMove|swpNoSize|swpFrameChanged)
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return nil
}
func hideWindowFromTaskbarByHandle(hwnd uintptr) error {
	if hwnd == 0 {
		return errors.New("window handle invalid")
	}

	style, _, callErr := procGetWindowLongW.Call(hwnd, uintptr(int32(gwlExStyle)))
	if style == 0 && callErr != syscall.Errno(0) {
		return callErr
	}

	style |= wsExToolWindow
	style &^= wsExAppWindow

	_, _, callErr = procSetWindowLongW.Call(hwnd, uintptr(int32(gwlExStyle)), style)
	if callErr != syscall.Errno(0) {
		return callErr
	}

	_, _, callErr = procSetWindowPos.Call(
		hwnd,
		0,
		0,
		0,
		0,
		0,
		swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged,
	)
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return nil
}

func cursorInModelInteractRect(hwnd uintptr) (bool, error) {
	var c point
	r1, _, e1 := procGetCursorPos.Call(uintptr(unsafe.Pointer(&c)))
	if r1 == 0 {
		if e1 != syscall.Errno(0) {
			return false, e1
		}
		return false, errors.New("GetCursorPos failed")
	}

	var wr rect
	r2, _, e2 := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	if r2 == 0 {
		if e2 != syscall.Errno(0) {
			return false, e2
		}
		return false, errors.New("GetWindowRect failed")
	}

	w := wr.Right - wr.Left
	h := wr.Bottom - wr.Top
	if w <= 0 || h <= 0 {
		return false, nil
	}

	// Interaction region approximates the visible Live2D body area.
	left := wr.Left + int32(float64(w)*0.25)
	right := wr.Left + int32(float64(w)*0.75)
	top := wr.Top + int32(float64(h)*0.08)
	bottom := wr.Top + int32(float64(h)*0.98)

	inside := c.X >= left && c.X <= right && c.Y >= top && c.Y <= bottom
	return inside, nil
}

func cursorNormalizedInWindow(hwnd uintptr) (float64, float64, error) {
	var c point
	r1, _, e1 := procGetCursorPos.Call(uintptr(unsafe.Pointer(&c)))
	if r1 == 0 {
		if e1 != syscall.Errno(0) {
			return 0, 0, e1
		}
		return 0, 0, errors.New("GetCursorPos failed")
	}

	var wr rect
	r2, _, e2 := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	if r2 == 0 {
		if e2 != syscall.Errno(0) {
			return 0, 0, e2
		}
		return 0, 0, errors.New("GetWindowRect failed")
	}

	w := wr.Right - wr.Left
	h := wr.Bottom - wr.Top
	if w <= 0 || h <= 0 {
		return 0, 0, errors.New("window size invalid")
	}

	left := wr.Left + int32(float64(w)*0.25)
	right := wr.Left + int32(float64(w)*0.75)
	top := wr.Top + int32(float64(h)*0.08)
	bottom := wr.Top + int32(float64(h)*0.98)

	rw := right - left
	rh := bottom - top
	if rw <= 0 || rh <= 0 {
		return 0, 0, errors.New("interaction rect invalid")
	}

	x := ((float64(c.X-left) / float64(rw)) - 0.5) * 2
	y := ((float64(c.Y-top) / float64(rh)) - 0.5) * 2

	if x > 1 {
		x = 1
	}
	if x < -1 {
		x = -1
	}
	if y > 1 {
		y = 1
	}
	if y < -1 {
		y = -1
	}

	return x, y, nil
}
