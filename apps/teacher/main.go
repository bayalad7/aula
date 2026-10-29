// Command teacher is the Aula Consola Wails application: the teacher picks a
// channel, sees the students connect, and emits code and notices to them.
package main

import (
	"context"
	"embed"
	"io"
	"log/slog"
	"os"
	"os/user"
	"strings"

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
	log := openLog()
	log.Info("aula-consola starting", "version", version)
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
	app.autoStart = true
	if app.defaultName == "" {
		app.defaultName = userDisplayName()
	}

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
			app.raise = func() {
				runtime.WindowUnminimise(ctx)
				runtime.WindowShow(ctx)
			}
			app.startup(ctx)
		},
		OnShutdown: app.shutdown,
		Bind:       []any{app},
		// One console per user: a second launch would fight for port 47830.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "aula-consola-3b9d7e21-console",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.raise() },
		},
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

// openLog logs to %LOCALAPPDATA%\Aula\consola\aula-consola.log as well as
// stderr. Nothing secret or private is logged.
func openLog() *slog.Logger {
	f, _, err := platform.OpenLogFile("consola", "aula-consola.log")
	if err != nil {
		return slog.Default()
	}
	return slog.New(slog.NewTextHandler(io.MultiWriter(os.Stderr, f), &slog.HandlerOptions{Level: slog.LevelInfo}))
}

// userDisplayName is the default name students see: the Windows display name
// of the user ("Benjamín Ayala"), else the account name, else the machine.
func userDisplayName() string {
	if u, err := user.Current(); err == nil {
		if n := strings.TrimSpace(u.Name); n != "" && len(n) <= 63 {
			return n
		}
		name := u.Username
		if i := strings.LastIndexAny(name, `\/`); i >= 0 {
			name = name[i+1:]
		}
		if name != "" && len(name) <= 63 {
			return name
		}
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "Profesor"
}
