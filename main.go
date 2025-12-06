package main

import (
	"embed"
	"log"

	"LagrangeQQ/internal/backend"
	"LagrangeQQ/internal/db"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := db.InitDB(); err != nil {
		log.Fatalf("init db failed: %v", err)
	}

	// Create an instance of the backend structure
	backendApp := backend.NewBackend()

	// Create an instance of the app structure
	app := NewApp(backendApp)

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "LagrangeQQ",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
			backendApp,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
