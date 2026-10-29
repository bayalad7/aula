// Command student is the Aula Cliente Wails application: the agent that
// receives code from the teacher's console and shows it, always identifying
// itself on screen.
package main

import (
	"context"
	"crypto/ed25519"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

// singleInstanceID keeps one agent per user session: opening the app again
// (or the autostart plus a manual start) just brings the window forward.
const singleInstanceID = "aula-cliente-5f0c2a4e-agent"

func main() {
	if len(os.Args) == 2 && isVersionFlag(os.Args[1]) {
		platform.AttachParentConsole()
		fmt.Println("aula-cliente " + version)
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "servicio" {
		platform.AttachParentConsole()
		os.Exit(runServiceCLI(os.Args[2:], os.Stdout, os.Stderr))
	}
	log := openLog()
	if len(os.Args) == 2 && os.Args[1] == "diagnostico" {
		platform.AttachParentConsole()
		os.Exit(runDiagnose(os.Stdout, log))
	}
	if len(os.Args) == 3 && os.Args[1] == agentsvc.HelperFlag {
		os.Exit(runHelper(os.Args[2], log))
	}
	if len(os.Args) == 1 && serviceRunning() {
		// The agent runs as a service and its helper owns the window: bring
		// it forward instead of starting a second agent.
		log.Info("agent service running; raising its window")
		runLauncher(log)
		return
	}
	log.Info("aula-cliente starting", "version", version)
	cfg, pub, cfgErr := loadConfig(os.Args[1:])
	if cfgErr != nil {
		log.Warn("agent not configured", "err", cfgErr)
	}
	clip, clipErr := platform.NewClipboard()
	if clipErr != nil {
		log.Warn("no system clipboard", "err", clipErr)
	}
	app := NewApp(cfg, pub, cfgErr, localHello(), clip, clipErr, log)
	app.mediaWait = 5 * time.Second // the frontend reports its WebRTC support on load
	capture, capErr := platform.NewScreenCapture()
	indicator, indErr := platform.NewScreenIndicator()
	if capErr == nil && indErr == nil {
		app.enableScreen(capture, indicator)
	} else {
		log.Info("screen thumbnails not available", "capture", capErr, "indicator", indErr)
	}
	if len(os.Args) == 1 {
		// Configured by files (installer): follow their changes. Flags pin it.
		app.reload = func() (Config, ed25519.PublicKey, error) { return loadConfig(nil) }
	}
	if err := runWindow(app, func(ctx context.Context) {
		app.emit = func(event string, data any) {
			runtime.EventsEmit(ctx, event, data)
			if s, ok := data.(Status); ok {
				runtime.WindowSetTitle(ctx, s.Banner) // visible in the taskbar too
			}
		}
		app.start(ctx)
	}, func() { app.stop() }); err != nil {
		log.Error("wails run failed", "err", err)
		os.Exit(1)
	}
}

// runWindow runs the Wails window of app (the standalone agent or the
// helper). onStartup runs once the window exists; raise and maximize are
// wired here for both.
func runWindow(app *App, onStartup func(ctx context.Context), onShutdown func()) error {
	return wails.Run(&options.App{
		Title:     "Aula — Cliente",
		Width:     900,
		Height:    700,
		MinWidth:  480,
		MinHeight: 360,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.raise = func() { raiseWindow(ctx) }
			app.maximize = func(on bool) {
				if on {
					runtime.WindowMaximise(ctx)
				} else {
					runtime.WindowUnmaximise(ctx)
				}
			}
			onStartup(ctx)
		},
		OnShutdown: func(context.Context) { onShutdown() },
		Bind:       []any{app},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               singleInstanceID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.raise() },
		},
	})
}

// runLauncher hands over to the window that already holds the single
// instance lock (the helper), which comes forward; if there is none (the
// service is starting it again) it just exits, hidden.
func runLauncher(log *slog.Logger) {
	err := wails.Run(&options.App{
		Title:       "Aula — Cliente",
		StartHidden: true,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   func(ctx context.Context) { runtime.Quit(ctx) },
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               singleInstanceID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {},
		},
	})
	if err != nil {
		log.Error("wails run failed", "err", err)
	}
}

// openLog logs to %LOCALAPPDATA%\Aula\alumno\aula-cliente.log as well as
// stderr: a window app has no console, and the administrator needs to know why
// an agent does not connect. Nothing secret or private is logged.
func openLog() *slog.Logger {
	f, _, err := platform.OpenLogFile("alumno", "aula-cliente.log")
	if err != nil {
		return slog.Default()
	}
	// The file goes first: io.MultiWriter stops at the first failing writer,
	// and a window app has no usable stderr, so with stderr first nothing
	// reached the file.
	return slog.New(slog.NewTextHandler(io.MultiWriter(f, os.Stderr), &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func isVersionFlag(s string) bool {
	return s == "version" || s == "-version" || s == "--version"
}

// raiseWindow shows the window when something arrives. Toggling always-on-top
// gets past Windows' focus-stealing rules without keeping it on top.
func raiseWindow(ctx context.Context) {
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
	runtime.WindowSetAlwaysOnTop(ctx, true)
	time.AfterFunc(1500*time.Millisecond, func() { runtime.WindowSetAlwaysOnTop(ctx, false) })
}
