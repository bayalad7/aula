package main

import (
	"slices"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/remote"
)

// Values of Status.Control.
const (
	ControlActive = "active"
	ControlPaused = "paused"
)

// enableRemote lets the teacher control this computer's keyboard and mouse
// (docs/protocol.md §5.16) and declares remote in hello. It needs the screen
// (enableScreen first): the teacher must see what is controlled, and the
// student must see the notice.
func (a *App) enableRemote(in platform.InputInjector) {
	a.control.Input = in
	if !a.screen.Available() {
		a.control.Input = nil
		return
	}
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), protocol.CapRemote)
}

// onControl shows or removes the control band and the window title
// (docs/privacidad.md rules 37-38). A control is an action of the teacher on
// the computer: the window comes forward and out of the compact mode. It
// runs with the remote agent locked.
func (a *App) onControl(s remote.State) {
	a.mu.Lock()
	began := a.ctrl == remote.Off && s != remote.Off
	a.ctrl = s
	st := a.status
	a.mu.Unlock()
	a.setStatus(st)
	a.updateYield() // the control notice stays in front of a pause
	if began {
		a.raise()
	}
}

// controlState is Status.Control: only while connected (the control ends
// with the connection).
func controlState(s remote.State, conn string) string {
	if conn != StateConnected {
		return ""
	}
	switch s {
	case remote.Active:
		return ControlActive
	case remote.Paused:
		return ControlPaused
	}
	return ""
}
