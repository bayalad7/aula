package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// --- Broadcast of the teacher's screen (docs/protocol.md §5.12) --------------------

// newBroadcaster wires the broadcast to this app: messages go through the open
// class, and status changes reach the frontend.
func (a *App) newBroadcaster(maxKbps int) *broadcast.Broadcaster {
	b := &broadcast.Broadcaster{
		NewEncoder: platform.NewH264Encoder,
		Send: func(ctx context.Context, sessionID string, p protocol.Payload) (string, error) {
			a.mu.Lock()
			class := a.bcClass
			a.mu.Unlock()
			if class == nil {
				return "", errNoClass
			}
			return class.Send(ctx, sessionID, p)
		},
		OnStatus: a.onBroadcastStatus,
		MaxKbps:  maxKbps,
		Logger:   a.log,
	}
	if c, err := platform.NewDisplayCapture(); err == nil {
		b.Capture = c
	} else {
		a.log.Info("screen broadcast not available", "err", err)
	}
	return b
}

// Displays lists the monitors the teacher can broadcast.
func (a *App) Displays() []DisplayInfo {
	ds, err := a.bc.Displays()
	out := []DisplayInfo{}
	if err != nil {
		return out
	}
	for _, d := range ds {
		label := fmt.Sprintf("Monitor %d", d.Index+1)
		if d.Primary {
			label += " (principal)"
		}
		out = append(out, DisplayInfo{Index: d.Index, Label: fmt.Sprintf("%s · %d×%d", label, d.Bounds.Dx(), d.Bounds.Dy()),
			Width: d.Bounds.Dx(), Height: d.Bounds.Dy(), Primary: d.Primary})
	}
	return out
}

// StartBroadcast shows monitor display to every student whose agent can
// receive it, and to those who join later. The console window is excluded
// from the capture first, so the class never sees the students' list or
// screens (docs/privacidad.md rule 15).
func (a *App) StartBroadcast(display int) (BroadcastState, error) {
	class := a.currentClass()
	if class == nil {
		return a.BroadcastState(), errNoClass
	}
	a.bcMu.Lock()
	defer a.bcMu.Unlock()
	if err := a.exclude(true); err != nil {
		a.log.Warn("console cannot be excluded from the capture", "err", err)
		return a.BroadcastState(), errors.New("no se pudo ocultar la consola de la transmisión; no se transmite para no mostrar la lista de alumnos")
	}
	a.bc.Media.UDPPort = a.broadcastUDPPort
	a.mu.Lock()
	a.bcClass = class
	a.mu.Unlock()
	if err := a.bc.Start(display, class.Sessions()); err != nil {
		_ = a.exclude(false)
		a.log.Warn("broadcast not started", "display", display, "err", err)
		return a.BroadcastState(), friendlyBroadcast(err)
	}
	st := a.BroadcastState()
	a.emit(EventBroadcast, st)
	return st, nil
}

// StopBroadcast ends the broadcast: every student's view closes. EndClass
// calls it before closing the class, so the stop messages still go out.
func (a *App) StopBroadcast() BroadcastState {
	a.bcMu.Lock()
	defer a.bcMu.Unlock()
	a.bc.Stop()
	a.mu.Lock()
	a.bcClass = nil
	a.mu.Unlock()
	if err := a.exclude(false); err != nil {
		a.log.Debug("console capture affinity not restored", "err", err)
	}
	st := a.BroadcastState()
	a.emit(EventBroadcast, st)
	return st
}

// BroadcastState returns the broadcast as the console shows it.
func (a *App) BroadcastState() BroadcastState {
	return a.stateOf(a.bc.Status())
}

func (a *App) stateOf(s broadcast.Status) BroadcastState {
	display := -1
	if s.Active {
		display = a.bc.Display()
	}
	return BroadcastState{Active: s.Active, Display: display, Receivers: s.Receivers, Eligible: s.Eligible,
		Available: a.bc.Capture != nil}
}

// onBroadcastStatus forwards the receiver counts; it runs on the caster's
// goroutine. A broadcast that ends by itself (the encoder failed) also gives
// the console window back to captures.
func (a *App) onBroadcastStatus(s broadcast.Status) {
	if !s.Active {
		_ = a.exclude(false)
	}
	a.emit(EventBroadcast, a.stateOf(s))
}

// onBroadcastMessage takes the agents' answers, candidates and errors.
func (a *App) onBroadcastMessage(sessionID string, env protocol.Envelope) {
	a.bc.Handle(sessionID, env)
}

func friendlyBroadcast(err error) error {
	switch {
	case errors.Is(err, platform.ErrNotSupported):
		return errors.New("este equipo no puede transmitir su pantalla: falta el codificador H.264 de Windows (en ediciones N, instala el Media Feature Pack)")
	case errors.Is(err, broadcast.ErrAlreadyBroadcasting):
		return errors.New("ya estás transmitiendo")
	default:
		return fmt.Errorf("no se pudo transmitir: %w", err)
	}
}
