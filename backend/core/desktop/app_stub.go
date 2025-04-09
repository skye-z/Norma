//go:build !windows

package desktop

import (
	"errors"
	"io/fs"
)

type App struct{}

func Run(_ fs.FS) error {
	return errors.New("desktop UI is only supported on windows")
}
