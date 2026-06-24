package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "AndroidFS",
		Description: "Android Device File Viewer",
		Services: []application.Service{
			application.NewService(NewApp()),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "AndroidFS",
		Width:           1200,
		Height:          760,
		DevToolsEnabled: true,
		EnableFileDrop:  true,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
