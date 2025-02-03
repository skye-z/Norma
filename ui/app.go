package main

import (
	"context"
	"fmt"
)

// App struct
type App struct {
	ctx              context.Context
	mousePassthrough bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// SetMousePassthrough toggles click-through mode for the floating window.
func (a *App) SetMousePassthrough(enabled bool) string {
	if err := setMousePassthroughByTitle(appTitle, enabled); err != nil {
		return "error: " + err.Error()
	}
	a.mousePassthrough = enabled
	return "ok"
}

func (a *App) MousePassthrough() bool {
	return a.mousePassthrough
}
