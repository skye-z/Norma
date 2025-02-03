//go:build !windows

package main

import "errors"

func setMousePassthroughByTitle(_ string, _ bool) error {
	return errors.New("mouse passthrough is only supported on windows")
}
