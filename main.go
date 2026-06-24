package main

import (
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var version = "dev"

func main() {
	app := application.New(application.Options{
		Name:        "AndroidFS",
		Description: "Android Device File Viewer",
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "AndroidFS",
		Width:           1200,
		Height:          760,
		DevToolsEnabled: true,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
