//go:build !windows

package desktop

import "context"

func (a *App) runGlobalHotkeys(ctx context.Context) {
	_ = ctx
}
