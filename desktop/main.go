package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"bl2save/desktop/internal/bridge"
	"bl2save/desktop/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

var appCtx context.Context

func buildMenu(b *bridge.Bridge) *menu.Menu {
	openFolder := func(key string) menu.Callback {
		return func(*menu.CallbackData) {
			p, _ := b.CurrentPaths()[key].(string)
			if p == "" {
				return
			}
			if err := platform.OpenPath(p); err != nil {
				_, _ = wruntime.MessageDialog(appCtx, wruntime.MessageDialogOptions{
					Type:    wruntime.ErrorDialog,
					Title:   "Open folder failed",
					Message: err.Error(),
				})
			}
		}
	}

	m := menu.NewMenu()

	file := m.AddSubmenu("File")
	file.AddText("Open Save Folder", keys.CmdOrCtrl("o"), openFolder("save_dir"))
	file.AddText("Open Gibbed Data Folder", nil, openFolder("gibbed_dir"))
	file.AddSeparator()
	file.AddText("Quit", keys.CmdOrCtrl("q"), func(*menu.CallbackData) {
		wruntime.Quit(appCtx)
	})

	settings := m.AddSubmenu("Settings")
	settings.AddText("Re-run Setup Wizard…", keys.CmdOrCtrl(","), func(*menu.CallbackData) {
		wruntime.EventsEmit(appCtx, "app:navigate", "/setup/")
	})
	settings.AddText("Reload Configuration", keys.CmdOrCtrl("r"), func(*menu.CallbackData) {
		b.ReloadConfig()
	})
	settings.AddSeparator()
	settings.AddText("Open config.json", nil, func(*menu.CallbackData) {
		_ = platform.OpenPath(platform.ConfigPath())
	})

	help := m.AddSubmenu("Help")
	help.AddText("About BL2 Save Editor", nil, func(*menu.CallbackData) {
		_, _ = wruntime.MessageDialog(appCtx, wruntime.MessageDialogOptions{
			Type:    wruntime.InfoDialog,
			Title:   "About BL2 Save Editor",
			Message: "BL2 Save Editor (Go/Wails)\n\nA Borderlands 2 save editor: characters, inventory, missions, challenges, Gibbed codes.\n\nSaves are edited atomically with rotating backups. Achievement state is written into the save file.",
		})
	})

	return m
}

func main() {
	b := bridge.New()
	appSvc, savesSvc, editorSvc, itemsSvc, assetsSvc, steamSvc := b.Services()

	onStartup := func(ctx context.Context) {
		appCtx = ctx
		b.Startup(ctx)
	}

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
		OnStartup:        onStartup,
		Bind: []interface{}{
			appSvc, savesSvc, editorSvc, itemsSvc, assetsSvc, steamSvc,
		},
		Menu: buildMenu(b),
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
