package main

import (
	"context"
	"errors"
	"time"

	"github.com/bayalad7/aula/internal/classtimer"
	"github.com/bayalad7/aula/internal/protocol"
)

// --- Exercise timer (docs/protocol.md §5.19) --------------------------------------
//
// The «Temporizador» panel of the action bar, its franja while it runs and its
// section of «Participación». It only informs: nothing changes on a computer
// at zero, so simulated students get it too (docs/privacidad.md rule 54). The
// label is the teacher's text and is never logged.

// TimerRequest is what the Temporizador panel sends.
type TimerRequest struct {
	Label string `json:"label"`
	// Description is what the students have to do (nota de interfaz 18).
	Description string   `json:"description"`
	DurationS   int      `json:"durationS"`
	Hosts       []string `json:"hosts"` // marked students; empty: the whole channel
}

// Timer returns the timer in force, for the franja and «Participación».
func (a *App) Timer() classtimer.Snapshot {
	return a.timerSnapshot(a.currentClass())
}

// StartTimer starts a timer for the class or the marked students; it
// replaces the one in force.
func (a *App) StartTimer(req TimerRequest) (classtimer.Snapshot, error) {
	class := a.currentClass()
	if class == nil {
		return classtimer.Snapshot{}, errNoClass
	}
	// Only who can show it: so a selection of students with an older Aula
	// says so instead of starting a timer nobody sees.
	if !anyCanTimer(class, req.Hosts) {
		return classtimer.Snapshot{}, codedf(codeNoTimerTargets, "ninguno de los alumnos elegidos puede mostrar el temporizador")
	}
	id, err := a.timer.Start(req.Label, req.Description, req.DurationS, req.Hosts)
	if err != nil {
		return classtimer.Snapshot{}, codedf(codeInvalidTimer, "temporizador no válido: un nombre de hasta %d caracteres, una descripción de hasta %d y de %d s a %d h",
			protocol.MaxTimerLabel, protocol.MaxTimerDescription, protocol.MinTimerDurationS, protocol.MaxTimerDurationS/3600)
	}
	a.log.Info("timer started", "timer", id, "duration_s", req.DurationS, "hosts", len(req.Hosts))
	return a.timerChanged(class), nil
}

// ExtendTimer adds addS seconds to the timer in force (it revives an ended one).
func (a *App) ExtendTimer(addS int) (classtimer.Snapshot, error) {
	class := a.currentClass()
	if class == nil {
		return classtimer.Snapshot{}, errNoClass
	}
	switch err := a.timer.Extend(addS); {
	case errors.Is(err, classtimer.ErrNotRunning):
		return classtimer.Snapshot{}, codedf(codeTimerGone, "ya no hay un temporizador en marcha")
	case errors.Is(err, classtimer.ErrTooLong):
		return classtimer.Snapshot{}, codedf(codeTimerTooLong, "el temporizador no puede pasar de %d horas", protocol.MaxTimerDurationS/3600)
	case err != nil:
		return classtimer.Snapshot{}, codedf(codeInvalidTimer, "extensión no válida: de %d a %d minutos",
			protocol.MinTimerExtendS/60, protocol.MaxTimerExtendS/60)
	}
	a.log.Info("timer extended", "add_s", addS)
	return a.timerChanged(class), nil
}

// StopTimer removes the timer from every screen («Detener», or «Quitar» once
// it ended).
func (a *App) StopTimer() (classtimer.Snapshot, error) {
	class := a.currentClass()
	if class == nil {
		return classtimer.Snapshot{}, errNoClass
	}
	if err := a.timer.Stop(); err != nil {
		return classtimer.Snapshot{}, codedf(codeTimerGone, "ya no hay un temporizador en marcha")
	}
	a.log.Info("timer stopped")
	return a.timerChanged(class), nil
}

// onTimerAck records what an agent, real or simulated, said about the timer.
func (a *App) onTimerAck(sessionID string, p protocol.TimerAck) {
	if a.timer.Ack(sessionID, p) {
		a.emitTimer(a.currentClass())
	}
}

// syncTimer brings every connected student up to date: who reconnects or
// joins gets the timer with what is left now (docs/protocol.md §5.19).
func (a *App) syncTimer(class classroom) {
	sessions := append(class.Sessions(), simSessionsOf(class)...)
	cmds := a.timer.Plan(sessions)
	if len(cmds) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, cmd := range cmds {
		if _, err := class.Send(ctx, cmd.SessionID, cmd.Payload); err != nil {
			a.log.Debug("timer not sent", "conn", cmd.SessionID, "type", cmd.Payload.MessageType(), "err", err)
			a.timer.Failed(cmd)
		}
	}
	a.emitTimer(class)
}

// timerChanged sends what changed, tells the window and arms the end.
func (a *App) timerChanged(class classroom) classtimer.Snapshot {
	a.syncTimer(class)
	a.armTimerEnd()
	s := a.timerSnapshot(class)
	a.emit(EventTimer, s)
	return s
}

// armTimerEnd tells the window when the timer reaches zero, so the franja
// says «terminó» without waiting for the acks.
func (a *App) armTimerEnd() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.timerEnd != nil {
		a.timerEnd.Stop()
		a.timerEnd = nil
	}
	if left, ok := a.timer.Remaining(); ok && left > 0 {
		a.timerEnd = time.AfterFunc(left, func() {
			a.log.Info("timer ended")
			a.emitTimer(a.currentClass())
			a.closeQuizAtZero() // a quiz with time closes at zero (§5.23.1)
		})
	}
}

func (a *App) emitTimer(class classroom) { a.emit(EventTimer, a.timerSnapshot(class)) }

func (a *App) timerSnapshot(class classroom) classtimer.Snapshot {
	if class == nil {
		return a.timer.Snapshot(nil, nil)
	}
	return a.timer.Snapshot(class.Sessions(), simSessionsOf(class))
}

// resetTimer forgets the timer when the class closes or changes channel;
// agents drop it by themselves (they do not hear from the console again).
func (a *App) resetTimer() {
	a.timer.Reset()
	a.armTimerEnd()
	a.emit(EventTimer, a.timer.Snapshot(nil, nil))
}

// anyCanTimer reports whether a connected student among hosts (all when
// empty), real or simulated, declared timer.
func anyCanTimer(class classroom, hosts []string) bool {
	for _, s := range targetsOf(append(class.Sessions(), simSessionsOf(class)...), hosts) {
		if s.Can(protocol.CapTimer) {
			return true
		}
	}
	return false
}
