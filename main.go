package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Locale Studio",
		Width:     1180,
		Height:    900,
		MinWidth:  680,
		MinHeight: 740,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 16, G: 20, B: 29, A: 255},
		OnStartup:        app.startup,
		SingleInstanceLock: &options.SingleInstanceLock{
			// Keep this ID stable across versions and portable installation paths.
			UniqueId:               "9e5cd41a-2681-4cff-abe2-8a9dd8a7b7d1",
			OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
