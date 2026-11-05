package main

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/bayalad7/aula/internal/platform"
)

// The window of the student app (prompt 19, delivery D):
//
//   - Code and notices never take the keyboard focus nor move the window
//     (notify): a visible window updates in place, a minimized one flashes
//     its taskbar button, and one closed to the notification area only marks
//     its icon.
//   - Power, restrictions and the teacher's broadcast are actions the student
//     must see before they happen (docs/privacidad.md rules 12, 20, 26): they
//     bring the window forward (raise), out of the notification area and out
//     of the compact mode.
//   - Closing the window hides it; the agent keeps running with its icon in
//     the notification area (rule 2). Where there is no notification area,
//     closing minimizes.
//   - The compact mode shrinks the window to a strip in a corner.

// Window sizes, in device-independent pixels.
const (
	normalMinWidth  = 480
	normalMinHeight = 360
	compactWidth    = 380
	compactHeight   = 112 // the title bar and the strip
)

// trayOpenLabel is the notification area menu entry. Native text, like the
// screen indicator's: the page's texts are in textos.es-MX.js.
const trayOpenLabel = "Abrir Aula"

type appWindow struct {
	app *App
	log *slog.Logger

	mu       sync.Mutex
	ctx      context.Context // the Wails window; nil until it exists
	win      platform.AppWindow
	tray     platform.Tray
	hidden   bool // closed to the notification area
	compact  bool
	quitting atomic.Bool // the process is ending: let the window close
}

// attachWindow wires the app to its Wails window once the page is loaded:
// the hooks of App and the notification area icon.
func (a *App) attachWindow(ctx context.Context) *appWindow {
	w := &appWindow{app: a, log: a.log, ctx: ctx}
	if win, err := platform.NewAppWindow(); err == nil {
		w.win = win
	} else {
		a.log.Info("window: no native control, using Wails only", "err", err)
	}
	normal, marked, err := trayIcons()
	if err == nil {
		w.tray, err = platform.NewTray(platform.TrayOptions{Icon: normal, MarkedIcon: marked,
			Tooltip: "Aula", OpenLabel: trayOpenLabel, OnOpen: w.open})
	}
	if err != nil {
		a.log.Info("window: no notification area icon; closing will minimize", "err", err)
	}
	a.notify = w.notify
	a.raise = w.raise
	a.compactHook = w.setCompact
	a.trayText = w.setTooltip
	return w
}

// notify tells the student something arrived, without the focus.
func (w *appWindow) notify() {
	w.mu.Lock()
	hidden, tray, win := w.hidden, w.tray, w.win
	w.mu.Unlock()
	switch {
	case hidden && tray != nil:
		_ = tray.SetMarked(true)
	case win != nil:
		if err := win.Flash(); err != nil {
			w.log.Warn("window: flash", "err", err)
		}
	}
}

// raise brings the window forward for an action of the teacher the student
// must see: out of the notification area and out of the compact mode.
func (w *appWindow) raise() {
	w.mu.Lock()
	ctx := w.ctx
	wasCompact := w.compact
	w.hidden = false
	tray := w.tray
	w.mu.Unlock()
	if ctx == nil {
		return
	}
	if tray != nil {
		_ = tray.SetMarked(false)
	}
	if wasCompact {
		_ = w.setCompact(false)
		w.app.emit(EventCompact, Compact{On: false})
	}
	raiseWindow(ctx)
}

// open is the notification area icon's click: the student asked for it.
func (w *appWindow) open() {
	w.mu.Lock()
	ctx := w.ctx
	w.hidden = false
	tray := w.tray
	w.mu.Unlock()
	if tray != nil {
		_ = tray.SetMarked(false)
	}
	if ctx != nil {
		raiseWindow(ctx)
	}
}

// beforeClose keeps the agent running when the student closes the window
// (rule 2). It returns true to cancel the close.
func (w *appWindow) beforeClose(ctx context.Context) bool {
	if w.quitting.Load() {
		return false
	}
	w.mu.Lock()
	tray := w.tray
	if tray != nil {
		w.hidden = true
	}
	w.mu.Unlock()
	if tray != nil {
		runtime.WindowHide(ctx)
	} else {
		runtime.WindowMinimise(ctx)
	}
	return true
}

// quit lets the window close (the process is ending) and removes the icon.
func (w *appWindow) quit() {
	w.quitting.Store(true)
	w.mu.Lock()
	tray := w.tray
	w.tray = nil
	w.mu.Unlock()
	if tray != nil {
		_ = tray.Close()
	}
}

func (w *appWindow) setTooltip(text string) {
	w.mu.Lock()
	tray := w.tray
	w.mu.Unlock()
	if tray != nil {
		_ = tray.SetTooltip(text)
	}
}

// setCompact shrinks the window to a strip at the bottom right of the
// screen, or restores it. It never activates the window nor keeps it on top.
func (w *appWindow) setCompact(on bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ctx == nil || w.compact == on {
		return nil
	}
	w.compact = on
	if on {
		runtime.WindowSetMinSize(w.ctx, 200, 60)
		if w.win != nil {
			return w.win.Compact(true, compactWidth, compactHeight)
		}
		runtime.WindowSetSize(w.ctx, compactWidth, compactHeight)
		return nil
	}
	var err error
	if w.win != nil {
		err = w.win.Compact(false, 0, 0)
	} else {
		runtime.WindowSetSize(w.ctx, 900, 700)
	}
	runtime.WindowSetMinSize(w.ctx, normalMinWidth, normalMinHeight)
	return err
}

// raiseWindow shows the window and brings it forward. Toggling always-on-top
// gets past Windows' focus-stealing rules without keeping it on top. Only for
// what the student must see (power, restrictions, broadcast) or asked for
// (the notification area icon, a second launch).
func raiseWindow(ctx context.Context) {
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
	runtime.WindowSetAlwaysOnTop(ctx, true)
	time.AfterFunc(1500*time.Millisecond, func() { runtime.WindowSetAlwaysOnTop(ctx, false) })
}
