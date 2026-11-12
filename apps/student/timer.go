package main

import (
	"context"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// --- Exercise timer (docs/protocol.md §5.19) --------------------------------------
//
// The badge «Ejercicio 1 · 4:32» in the identity strip. It only informs: it
// never takes the focus nor brings the window forward (no raise), and it is
// not a band (docs/privacidad.md rule 54). Go sends states; the window counts
// the seconds down by itself from RemainingMS and words it.

// EventTimer carries a classtimer.View whenever the timer starts, is
// extended, stops, ends (JustEnded) or goes away.
const EventTimer = "aula:timer"

// handleTimer applies timer.start, timer.extend and timer.stop.
func (a *App) handleTimer(ctx context.Context, cl *transport.Client, env protocol.Envelope) bool {
	var acks []protocol.TimerAck
	switch p := env.Payload.(type) {
	case protocol.TimerStart:
		prev := a.countdown.View().TimerID
		acks = a.countdown.Start(env.ID, p)
		a.log.Info("timer started", "timer", p.TimerID, "duration_s", p.DurationS, "remaining_ms", p.RemainingMS)
		// A new timer with what to do: its card says «Nuevo» (nota de interfaz 18). Never
		// raise; a resend of the same timer does not tell again.
		if p.Description != "" && p.TimerID != prev {
			defer a.notify()
		}
	case protocol.TimerExtend:
		ack, ok := a.countdown.Extend(env.ID, p)
		if !ok {
			return true
		}
		acks = []protocol.TimerAck{ack}
		a.log.Info("timer extended", "timer", p.TimerID, "add_s", p.AddS, "remaining_ms", p.RemainingMS)
	case protocol.TimerStop:
		ack, ok := a.countdown.Stop(env.ID, p)
		if !ok {
			return true
		}
		acks = []protocol.TimerAck{ack}
		a.log.Info("timer stopped", "timer", p.TimerID)
	default:
		return false
	}
	for _, ack := range acks {
		if _, err := cl.Send(ctx, ack); err != nil {
			a.log.Debug("cannot send timer.ack", "err", err)
		}
	}
	a.timerChanged(false)
	return true
}

// timerReconnected: a new session started. The count goes on; the console
// sends it again if it is still running, or it goes away (ReconnectGrace).
func (a *App) timerReconnected() {
	a.countdown.Reconnected()
	a.armTimer()
}

// timerTick runs when the countdown has something to do: the end, or
// dropping the badge.
func (a *App) timerTick() {
	ack, ended, changed := a.countdown.Tick()
	if ended {
		a.log.Info("timer ended", "timer", ack.TimerID)
		if cl := a.connection(); cl != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if _, err := cl.Send(ctx, ack); err != nil {
				a.log.Debug("cannot send timer.ack", "err", err)
			}
			cancel()
		}
	}
	if changed {
		a.timerChanged(ended)
		return
	}
	a.armTimer()
}

// timerChanged tells the window and arms the next tick. At the end only
// notify: the student keeps the keyboard (prompt 19).
func (a *App) timerChanged(justEnded bool) {
	v := a.countdown.View()
	v.JustEnded = justEnded
	a.emit(EventTimer, v)
	if justEnded {
		a.notify()
	}
	a.armTimer()
}

func (a *App) armTimer() {
	a.timerMu.Lock()
	defer a.timerMu.Unlock()
	if a.timerT != nil {
		a.timerT.Stop()
		a.timerT = nil
	}
	if d, ok := a.countdown.Next(); ok {
		a.timerT = time.AfterFunc(d, a.timerTick)
	}
}

// stopTimer stops the tick when the agent stops.
func (a *App) stopTimer() {
	a.timerMu.Lock()
	defer a.timerMu.Unlock()
	if a.timerT != nil {
		a.timerT.Stop()
		a.timerT = nil
	}
}
