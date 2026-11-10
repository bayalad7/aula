// Command student is the Aula Cliente Wails application: the agent that
// receives code from the teacher's console and shows it, always identifying
// itself on screen.
package main

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/platform"
)

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
	if in, err := platform.NewInputInjector(); err == nil && app.screen.Available() {
		app.enableRemote(in)
	} else {
		log.Info("remote control not available", "err", err)
	}
	if wf, err := platform.NewWorkFolder(); err == nil {
		app.enableFiles(wf)
	} else {
		log.Info("receiving files not available", "err", err)
	}
	if wr, err := platform.NewWorkFolderReader(); err == nil {
		app.enableCollect(wr)
	} else {
		log.Info("collecting files not available", "err", err)
	}
	if l, err := platform.NewLauncher(); err == nil {
		app.enableOpen(l)
	} else {
		log.Info("opening pages and apps not available", "err", err)
	}
	if p, err := platform.NewPower(); err == nil {
		app.enablePower(p)
	} else {
		log.Info("power actions not available", "err", err)
	}
	if len(os.Args) == 1 {
		// Configured by files (installer): follow their changes. Flags pin it.
		app.reload = func() (Config, ed25519.PublicKey, error) { return loadConfig(nil) }
	}
	if err := runWindow(app, func(ctx context.Context) {
		// The window sets its own title from the status (rule 1: visible in
		// the taskbar too).
		app.emit = func(event string, data any) { runtime.EventsEmit(ctx, event, data) }
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
	files, err := frontend()
	if err != nil {
		return fmt.Errorf("frontend: %w", err)
	}
	var (
		win     *appWindow
		winOnce sync.Once
	)
	app.quitWindow = func() {
		if win != nil {
			win.quit() // let runtime.Quit close the window
		}
	}
	return wails.Run(&options.App{
		Title:     "Aula — Cliente",
		Width:     900,
		Height:    700,
		MinWidth:  480,
		MinHeight: 360,
		AssetServer: &assetserver.Options{
			Assets: files,
		},
		OnStartup: func(ctx context.Context) {
			app.raise = func() { raiseWindow(ctx) } // until the page is loaded (attachWindow)
			app.maximize = func(on bool) {
				if on {
					runtime.WindowMaximise(ctx)
				} else {
					runtime.WindowUnmaximise(ctx)
				}
			}
			app.onTop = func(on bool) { runtime.WindowSetAlwaysOnTop(ctx, on) }
			onStartup(ctx)
		},
		// The window exists once the page is loaded: then its notification
		// area icon and native controls (window.go).
		OnDomReady: func(ctx context.Context) {
			winOnce.Do(func() { win = app.attachWindow(ctx) })
		},
		OnBeforeClose: func(ctx context.Context) bool {
			if win == nil {
				return false
			}
			return win.beforeClose(ctx)
		},
		OnShutdown: func(context.Context) {
			if win != nil {
				win.quit()
			}
			onShutdown()
		},
		Bind: []any{app},
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
		AssetServer: &assetserver.Options{Assets: assets}, // hidden: it never draws
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
