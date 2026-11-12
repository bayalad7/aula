package main

import (
	"slices"

	"github.com/bayalad7/aula/internal/broadcast"
)

// --- Showing this screen to the class (docs/protocol.md §5.24) -----------------------
//
// The teacher decides when it starts and ends, as with a remote control: the
// student is not asked and cannot stop it (docs/privacidad.md rules 75-82).
// It is an action of the teacher on this computer: the window comes forward
// (raise) with the band «Tu pantalla se está mostrando a la clase», without a
// button, and the native badge says the same before the first frame.

// EventPresent carries a broadcast.PresentView when the band appears or goes
// away.
const EventPresent = "aula:present"

// enablePresent lets this agent show its screen to the class and declares
// screen.present in hello. It needs the native badge (enableScreen first).
func (a *App) enablePresent(p broadcast.Publisher) {
	a.present.Publisher = p
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), a.present.Capabilities()...)
}

// onPresent shows or removes the band; when it starts, the window comes
// forward and out of the compact mode.
func (a *App) onPresent(v broadcast.PresentView) {
	a.mu.Lock()
	began := !a.presenting && v.Phase == broadcast.PresentLive
	a.presenting = v.Phase == broadcast.PresentLive
	st := a.status
	a.mu.Unlock()
	a.setStatus(st)
	a.emit(EventPresent, v)
	if began {
		a.raise()
	}
}
