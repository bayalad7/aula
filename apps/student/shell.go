package main

import (
	"context"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// shell is what the window code asks of Wails, so that hold and release can
// be tested without a window (window_test.go).
type shell interface {
	raise(keep func() bool) // show and bring forward (raiseWindow)
	isMaximised() bool
	maximise(on bool)
	fullscreen(on bool)
	onTop(on bool)
	hide()
	minimise()
	setSize(width, height int)
	setMinSize(width, height int)
}

// wailsShell is the Wails window of this process.
type wailsShell struct{ ctx context.Context }

func (s wailsShell) raise(keep func() bool) { raiseWindow(s.ctx, keep) }
func (s wailsShell) isMaximised() bool      { return runtime.WindowIsMaximised(s.ctx) }
func (s wailsShell) maximise(on bool) {
	if on {
		runtime.WindowMaximise(s.ctx)
	} else {
		runtime.WindowUnmaximise(s.ctx)
	}
}
func (s wailsShell) fullscreen(on bool) {
	if on {
		runtime.WindowFullscreen(s.ctx)
	} else {
		runtime.WindowUnfullscreen(s.ctx)
	}
}
func (s wailsShell) onTop(on bool)                { runtime.WindowSetAlwaysOnTop(s.ctx, on) }
func (s wailsShell) hide()                        { runtime.WindowHide(s.ctx) }
func (s wailsShell) minimise()                    { runtime.WindowMinimise(s.ctx) }
func (s wailsShell) setSize(width, height int)    { runtime.WindowSetSize(s.ctx, width, height) }
func (s wailsShell) setMinSize(width, height int) { runtime.WindowSetMinSize(s.ctx, width, height) }

// raiseWindow shows the window and brings it forward. Toggling always-on-top
// gets past Windows' focus-stealing rules without keeping it on top, unless
// keep says a broadcast holds the window on top by then. Only for what the
// student must see (power, restrictions, broadcast) or asked for (the
// notification area icon, a second launch).
func raiseWindow(ctx context.Context, keep func() bool) {
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
	runtime.WindowSetAlwaysOnTop(ctx, true)
	time.AfterFunc(1500*time.Millisecond, func() {
		if keep == nil || !keep() {
			runtime.WindowSetAlwaysOnTop(ctx, false)
		}
	})
}
