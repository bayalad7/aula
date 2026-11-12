package main

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
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
//   - The broadcast also holds the window up (hold, rule 12): on top and
//     without minimize nor close, or in full screen. When it ends, for any
//     reason, the window goes back exactly as it was: its place and size,
//     compact, or closed to the notification area.
//   - Closing the window hides it; the agent keeps running with its icon in
//     the notification area (rule 2). Where there is no notification area,
//     closing minimizes. While a broadcast holds it, closing does nothing:
//     the exception to rule 2.
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
	sh       shell // the Wails window; nil until it exists
	win      platform.AppWindow
	tray     platform.Tray
	hidden   bool // closed to the notification area
	compact  bool
	quitting atomic.Bool // the process is ending: let the window close

	held    bool                   // a broadcast holds the window (hold)
	mode    protocol.BroadcastMode // its mode; "" is v0.4's maximized window
	before  windowBefore           // how the window was before the hold
	natived bool                   // the hold went through platform.AppWindow
}

// windowBefore is what hold puts back.
type windowBefore struct {
	hidden, compact, maximized bool
}

// attachWindow wires the app to its Wails window once the page is loaded:
// the hooks of App and the notification area icon.
func (a *App) attachWindow(ctx context.Context) *appWindow {
	w := &appWindow{app: a, log: a.log, sh: wailsShell{ctx}}
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
	a.hold = w.hold
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
	sh := w.sh
	wasCompact := w.compact
	w.hidden = false
	tray := w.tray
	w.mu.Unlock()
	if sh == nil {
		return
	}
	if tray != nil {
		_ = tray.SetMarked(false)
	}
	if wasCompact {
		_ = w.setCompact(false)
		w.app.emit(EventCompact, Compact{On: false})
	}
	sh.raise(w.holding)
}

// open is the notification area icon's click: the student asked for it.
func (w *appWindow) open() {
	w.mu.Lock()
	sh := w.sh
	w.hidden = false
	tray := w.tray
	w.mu.Unlock()
	if tray != nil {
		_ = tray.SetMarked(false)
	}
	if sh != nil {
		sh.raise(w.holding)
	}
}

// hold brings the window forward and keeps it up while the teacher
// broadcasts (rule 12), or, with on false, puts it back as it was. A new
// broadcast while one holds the window only changes the mode.
func (w *appWindow) hold(on bool, mode protocol.BroadcastMode) {
	w.mu.Lock()
	sh, win := w.sh, w.win
	if sh == nil {
		w.mu.Unlock()
		return
	}
	if !on {
		was, prev, before, natived := w.held, w.mode, w.before, w.natived
		w.held, w.mode, w.before, w.natived = false, "", windowBefore{}, false
		w.mu.Unlock()
		if was {
			w.release(sh, win, prev, before, natived)
		}
		return
	}
	first := !w.held
	if first {
		w.before = windowBefore{hidden: w.hidden, compact: w.compact}
	}
	w.held, w.mode = true, mode
	w.mu.Unlock()
	if first {
		// Outside w.mu: the Wails call waits for the window's thread, which
		// takes w.mu in beforeClose.
		maximized := sh.isMaximised()
		w.mu.Lock()
		w.before.maximized = maximized
		w.mu.Unlock()
	}
	w.raise()
	natived := false
	if pm, ok := platformHold(mode); ok && win != nil {
		if err := win.Hold(pm); err != nil {
			w.log.Warn("window: hold", "mode", mode, "err", err)
		} else {
			natived = true
		}
	}
	if !natived {
		holdWithWails(sh, true, mode)
	}
	w.mu.Lock()
	w.natived = natived
	w.mu.Unlock()
}

// release puts the window back as before the hold: what platform.AppWindow
// or Wails changed, then the compact mode or the notification area.
func (w *appWindow) release(sh shell, win platform.AppWindow, mode protocol.BroadcastMode, before windowBefore, natived bool) {
	switch {
	case natived:
		if err := win.Release(); err != nil {
			w.log.Warn("window: release", "err", err)
		}
	case mode == "":
		if !before.maximized {
			sh.maximise(false)
		}
	default:
		holdWithWails(sh, false, mode)
	}
	if before.compact {
		if err := w.setCompact(true); err == nil {
			w.app.emit(EventCompact, Compact{On: true})
		}
	}
	if before.hidden {
		w.mu.Lock()
		tray := w.tray
		w.hidden = tray != nil
		w.mu.Unlock()
		if tray != nil {
			sh.hide()
		}
	}
}

// platformHold is the native hold of a broadcast mode; v0.4's maximized
// window ("") has none: it is a plain maximize.
func platformHold(mode protocol.BroadcastMode) (platform.HoldMode, bool) {
	switch mode {
	case protocol.BroadcastWindow:
		return platform.HoldWindow, true
	case protocol.BroadcastFullscreen:
		return platform.HoldFullscreen, true
	}
	return 0, false
}

// holdWithWails holds the window with Wails alone: before the page is
// loaded, and where platform.AppWindow is not supported. It cannot remove
// the minimize box; the close is refused by beforeClose all the same.
func holdWithWails(sh shell, on bool, mode protocol.BroadcastMode) {
	switch mode {
	case "":
		sh.maximise(on)
	case protocol.BroadcastFullscreen:
		sh.fullscreen(on)
		sh.onTop(on)
	default:
		sh.onTop(on)
	}
}

// holding reports whether a broadcast holds the window and the student may
// not close it (window and full screen; not v0.4's maximized window).
func (w *appWindow) holding() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.held && w.mode != ""
}

// beforeClose keeps the agent running when the student closes the window
// (rule 2). It returns true to cancel the close. While a broadcast holds the
// window, the close does nothing (rule 12): it ends with the broadcast.
func (w *appWindow) beforeClose() bool {
	if w.quitting.Load() {
		return false
	}
	if w.holding() {
		return true
	}
	w.mu.Lock()
	tray, sh := w.tray, w.sh
	if tray != nil {
		w.hidden = true
	}
	w.mu.Unlock()
	if sh == nil {
		return false
	}
	if tray != nil {
		sh.hide()
	} else {
		sh.minimise()
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
	if w.sh == nil || w.compact == on || (on && w.held && w.mode != "") {
		return nil // a broadcast holds the window: no compact strip (rule 12)
	}
	w.compact = on
	if on {
		w.sh.setMinSize(200, 60)
		if w.win != nil {
			return w.win.Compact(true, compactWidth, compactHeight)
		}
		w.sh.setSize(compactWidth, compactHeight)
		return nil
	}
	var err error
	if w.win != nil {
		err = w.win.Compact(false, 0, 0)
	} else {
		w.sh.setSize(900, 700)
	}
	w.sh.setMinSize(normalMinWidth, normalMinHeight)
	return err
}
