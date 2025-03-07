//go:build !windows

package main

import "context"

func (a *App) runGlobalHotkeys(ctx context.Context) {
	_ = ctx
}
