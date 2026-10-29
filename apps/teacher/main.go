// Command teacher is the Aula Consola Wails application.
package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/bayalad7/aula/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// "aula-consola keygen" and friends run in the terminal, without a window.
	if len(os.Args) > 1 {
		platform.AttachParentConsole()
		os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
	}
	err := wails.Run(&options.App{
		Title:  "Aula — Consola",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
	})
	if err != nil {
		slog.Error("wails run failed", "err", err)
		os.Exit(1)
	}
}
