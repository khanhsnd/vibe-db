package main

import (
	"embed"
	"log"

	"github.com/khanhsnd/vibe-db/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is set by the release workflow with -ldflags. A checkout stays a development build so it
// cannot accidentally offer an update while somebody is working in it.
var version = "dev"

func main() {
	application, err := app.New(version)
	if err != nil {
		log.Fatalf("vibe-db: %v", err)
	}

	err = wails.Run(&options.App{
		Title:            "vibe-db",
		Width:            1280,
		Height:           820,
		MinWidth:         900,
		MinHeight:        560,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 0x0F, G: 0x11, B: 0x15, A: 1},
		OnStartup:        application.Startup,
		OnShutdown:       application.Shutdown,
		Bind:             application.Bindings(),
	})
	if err != nil {
		log.Fatalf("vibe-db: %v", err)
	}
}
