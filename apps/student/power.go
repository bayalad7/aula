package main

import (
	"slices"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/protocol"
)

// enablePower lets the console shut down, reboot or log off this computer
// after the notice and its countdown (docs/protocol.md §5.15), and declares
// power in hello. Call it before start.
func (a *App) enablePower(p platform.Power) {
	a.power.Power = p
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), protocol.CapPower)
}

// onPower shows the power notice (docs/privacidad.md rules 26-28): the window
// comes forward and stays above the others while it counts.
func (a *App) onPower(n *power.Notice) {
	a.mu.Lock()
	v := a.powerV
	if n == nil {
		v = PowerView{}
	} else {
		if v.Notice == nil || v.Notice.ActionID != n.ActionID {
			// Who gave the order, kept even if the connection drops (rule 30).
			v.Teacher, v.Channel = a.status.Teacher, a.status.Channel
			if v.Channel == "" {
				v.Channel = a.cfg.Channel
			}
		}
		v.Notice, v.Verb = n, powerVerb(n.Action)
	}
	a.powerV = v
	st := a.status
	a.mu.Unlock()
	a.setStatus(st)
	a.emit(EventPower, v)
	shown := n != nil && n.State != power.NoticeFailed
	a.onTop(shown)
	if n != nil {
		a.raise()
	}
}

// PowerPostpone adds 5 minutes to the countdown, at most twice (rule 27).
func (a *App) PowerPostpone() error {
	if a.remote != nil {
		return a.forward("PowerPostpone", nil)
	}
	return a.power.Postpone()
}

// DismissPower hides a notice of a power order that failed.
func (a *App) DismissPower() {
	if a.remote != nil {
		_ = a.forward("DismissPower", nil)
		return
	}
	a.power.Dismiss()
}

func powerVerb(k protocol.PowerKind) string {
	switch k {
	case protocol.PowerReboot:
		return "reiniciar este equipo"
	case protocol.PowerLogoff:
		return "cerrar tu sesión"
	}
	return "apagar este equipo"
}

// powerPending is what the banner adds while an order counts or runs.
func powerPending(n *power.Notice) string {
	if n == nil || n.State == power.NoticeFailed {
		return ""
	}
	switch n.Action {
	case protocol.PowerReboot:
		return "reinicio pendiente"
	case protocol.PowerLogoff:
		return "cierre de sesión pendiente"
	}
	return "apagado pendiente"
}
