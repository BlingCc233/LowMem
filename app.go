package main

import (
	"LagrangeQQ/internal/backend"
	"context"
	"fmt"
)

// App struct
type App struct {
	ctx     context.Context
	Backend *backend.Backend
}

// NewApp creates a new App application struct
func NewApp(b *backend.Backend) *App {
	return &App{
		Backend: b,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.Backend.SetContext(ctx)
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
