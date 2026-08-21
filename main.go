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
		Title:            "Vibe DB",
		Width:            1280,
		Height:           820,
		MinWidth:         900,
		MinHeight:        560,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 0x0F, G: 0x11, B: 0x15, A: 1},
		// A file dropped on a bucket is uploaded, so the window has to hand over the **paths** of what
		// was let go rather than a `File` the webview would have to read: that is the whole reason a
		// drop can be an upload at all, an object's bytes never crossing the bridge. Registering the
		// listeners is also what stops the webview doing what a browser does with a dropped file —
		// navigate to it, taking the app off the screen — which is why `bridge/filedrop.ts` registers
		// at startup and not when a bucket is opened.
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		OnStartup:   application.Startup,
		OnShutdown:  application.Shutdown,
		Bind:        application.Bindings(),
	})
	if err != nil {
		log.Fatalf("vibe-db: %v", err)
	}
}
