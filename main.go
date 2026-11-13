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
	app := NewApp()

	err := wails.Run(&options.App{
		Title:                    "Star With U",
		Width:                    1060,
		Height:                   720,
		MinWidth:                 1060,
		MinHeight:                720,
		DisableResize:            true,
		Frameless:                true,
		BackgroundColour:         &options.RGBA{R: 5, G: 6, B: 13, A: 1},
		AssetServer:              &assetserver.Options{Assets: assets},
		OnStartup:                app.startup,
		Bind:                     []interface{}{app},
		EnableDefaultContextMenu: false,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
