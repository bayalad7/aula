// Command teacher is the Aula Consola Wails application: the teacher picks a
// channel, sees the students connect, and emits code and notices to them.
package main

import (
	"context"
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/security"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// "aula-consola keygen" and friends run in the terminal, without a window.
	if len(os.Args) > 1 {
		platform.AttachParentConsole()
		os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
	}
	log := slog.Default()
	dir, err := consoleDir()
	if err != nil {
		log.Error("no config dir", "err", err)
		os.Exit(1)
	}
	clip, clipErr := platform.NewClipboard()
	if clipErr != nil {
		log.Warn("no system clipboard", "err", clipErr)
	}
	app := NewApp(dir, clip, clipErr, platform.SystemHotkeys(), log)

	err = wails.Run(&options.App{
		Title:     "Aula — Consola",
		Width:     1280,
		Height:    760,
		MinWidth:  900,
		MinHeight: 600,
		// Classroom projectors are often 1280×720: use the whole screen.
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.emit = func(event string, data any) { runtime.EventsEmit(ctx, event, data) }
			app.startup(ctx)
		},
		OnShutdown: app.shutdown,
		Bind:       []any{app},
	})
	if err != nil {
		log.Error("wails run failed", "err", err)
		os.Exit(1)
	}
}

// consoleDir is where the identity, certificate and settings live.
// AULA_CONSOLE_DIR overrides it, to test without touching the real key.
func consoleDir() (string, error) {
	if d := os.Getenv("AULA_CONSOLE_DIR"); d != "" {
		return d, nil
	}
	return security.DefaultDir()
}
