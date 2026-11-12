package main

import (
	"slices"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/remote"
	"github.com/bayalad7/aula/internal/ui"
)

// The pause window (screen.freeze, docs/protocol.md §5.9; docs/privacidad.md
// rules 9-11). It acts on the computer: the window comes forward (raise) and
// shows its band «… pausó esta computadora», which has no close button. The
// pause goes away with screen.unfreeze, when the connection closes and when
// the agent ends.

// pauseLook is the pause window in the colors of the design system, the same
// as the prototype (apps/teacher/frontend/dev/prototipo/alumno-pausa.html).
func pauseLook() platform.PauseLook {
	get := func(dark bool) (c platform.PauseColors) {
		c.Background, _ = ui.Color("c-vista-solido", dark)
		c.Text, _ = ui.Color("c-vista-solido-texto", dark)
		return c
	}
	return platform.PauseLook{Light: get(false), Dark: get(true)}
}

// newPauseOverlay is the OS pause window, in the design system's colors.
func newPauseOverlay() (platform.PauseOverlay, error) {
	return platform.NewPauseOverlay(pauseLook())
}

// enableFreeze lets the teacher pause this screen and declares screen.freeze
// in hello. Call it before start.
func (a *App) enableFreeze(o platform.PauseOverlay) {
	a.freeze.Overlay = o
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), a.freeze.Capabilities()...)
}

// pauseWho is who paused the screen, for the fixed text (rule 9).
func (a *App) pauseWho() (teacher, channel string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	channel = a.status.Channel
	if channel == "" {
		channel = a.cfg.Channel
	}
	return a.status.Teacher, channel
}

// onPaused shows or removes the band; a pause brings the window forward, out
// of the compact mode, although the pause covers it.
func (a *App) onPaused(on bool) {
	a.mu.Lock()
	a.paused = on
	st := a.status
	a.mu.Unlock()
	a.setStatus(st)
	if on {
		a.raise()
	}
}

// updateYield keeps the pause below the window while the student must see
// it: the teacher's or a classmate's screen, a power countdown or a remote
// control (docs/protocol.md §5.9).
func (a *App) updateYield() {
	a.mu.Lock()
	on := a.watching || a.ctrl != remote.Off ||
		(a.powerV.Notice != nil && a.powerV.Notice.State != power.NoticeFailed)
	a.mu.Unlock()
	a.freeze.Yield(on)
}

// pausedState is Status.Paused: only while connected (the pause ends with
// the connection).
func pausedState(paused bool, conn string) bool { return paused && conn == StateConnected }
