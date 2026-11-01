//go:build !windows && !darwin

package desktop

import "context"

func (a *App) runGlobalHotkeys(ctx context.Context) {
	_ = ctx
}
