package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"bl2save/desktop/internal/bridge"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	b := bridge.New()

	err := wails.Run(&options.App{
		Title:     "BL2 Save Editor",
		Width:     1500,
		Height:    950,
		MinWidth:  1100,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 12, B: 18, A: 255},
		OnStartup:        b.Startup,
		Bind: []interface{}{
			b,
		},
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
