// Command student is the Aula Cliente Wails application: the agent that
// receives code from the teacher's console and shows it, always identifying
// itself on screen.
package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/bayalad7/aula/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	log := slog.Default()
	cfg, pub, cfgErr := loadConfig(os.Args[1:])
	if cfgErr != nil {
		log.Warn("agent not configured", "err", cfgErr)
	}
	clip, clipErr := platform.NewClipboard()
	if clipErr != nil {
		log.Warn("no system clipboard", "err", clipErr)
	}
	app := NewApp(cfg, pub, cfgErr, localHello(), clip, clipErr, log)

	err := wails.Run(&options.App{
		Title:     "Aula — Cliente",
		Width:     900,
		Height:    700,
		MinWidth:  480,
		MinHeight: 360,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.emit = func(event string, data any) {
				runtime.EventsEmit(ctx, event, data)
				if s, ok := data.(Status); ok {
					runtime.WindowSetTitle(ctx, s.Banner) // visible in the taskbar too
				}
			}
			app.raise = func() { raiseWindow(ctx) }
			app.start(ctx)
		},
		OnShutdown: func(context.Context) { app.stop() },
		Bind:       []any{app},
	})
	if err != nil {
		log.Error("wails run failed", "err", err)
		os.Exit(1)
	}
}

// raiseWindow shows the window when something arrives. Toggling always-on-top
// gets past Windows' focus-stealing rules without keeping it on top.
func raiseWindow(ctx context.Context) {
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
	runtime.WindowSetAlwaysOnTop(ctx, true)
	time.AfterFunc(1500*time.Millisecond, func() { runtime.WindowSetAlwaysOnTop(ctx, false) })
}
