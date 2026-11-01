//go:build !windows && !darwin

package desktop

import "errors"

func findWindowByTitle(title string) (uintptr, error) {
	_ = title
	return 0, errors.New("unsupported platform")
}

func setMousePassthroughByTitle(title string, enabled bool) error {
	_ = title
	_ = enabled
	return errors.New("unsupported platform")
}

func setMousePassthroughByHandle(hwnd uintptr, enabled bool) error {
	_ = hwnd
	_ = enabled
	return errors.New("unsupported platform")
}

func hideWindowFromTaskbarByHandle(hwnd uintptr) error {
	_ = hwnd
	return errors.New("unsupported platform")
}

func cursorInModelInteractRect(hwnd uintptr) (bool, error) {
	_ = hwnd
	return false, errors.New("unsupported platform")
}

func setWindowTopmostByTitle(title string, enabled bool) error {
	_ = title
	_ = enabled
	return nil
}

func setWindowTopmostByHandle(hwnd uintptr, enabled bool) error {
	_ = hwnd
	_ = enabled
	return nil
}
