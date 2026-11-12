package main

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/screen"
	"github.com/bayalad7/aula/internal/session"
)

// --- Pausar pantallas (screen.freeze, docs/protocol.md §5.9–5.11) -----------------
//
// Only real students: a simulated one cannot declare screen.freeze and the
// console never sends it to them (docs/privacidad.md rule 34). The student
// shown to the class is skipped: their screen is what the others see.

// PauseRequest is what the «Pausar pantallas» panel, the Pantalla grande and
// Ctrl+Mayús+P send.
type PauseRequest struct {
	Hosts   []string `json:"hosts"`   // marked students; empty: the whole channel
	Message string   `json:"message"` // optional, 0-300 characters
}

// PauseResult says what happened to a pause or a resume.
type PauseResult struct {
	Sent        int      `json:"sent"`
	Unsupported []string `json:"unsupported"` // hostnames without screen.freeze
	Presenting  []string `json:"presenting"`  // shown to the class: not paused
	Failed      []string `json:"failed"`      // hostnames the message did not reach
}

func newPauseResult() PauseResult {
	return PauseResult{Unsupported: []string{}, Presenting: []string{}, Failed: []string{}}
}

// PauseScreens pauses the screens of the class or of the marked students.
// On a screen already paused it only changes the message.
func (a *App) PauseScreens(req PauseRequest) (PauseResult, error) {
	class := a.currentClass()
	if class == nil {
		return PauseResult{}, errNoClass
	}
	freeze := protocol.ScreenFreeze{Message: req.Message}
	if freeze.Validate() != nil {
		return PauseResult{}, codedf(codeInvalidPauseMessage, "el mensaje de la pausa tiene %d caracteres; el máximo es %d",
			utf8.RuneCountInString(req.Message), protocol.MaxFreezeMessage)
	}
	res := newPauseResult()
	presenting := a.proj.Status().SourceID
	var targets []session.Session
	for _, s := range targetsOf(class.Sessions(), req.Hosts) {
		switch {
		case !s.Can(protocol.CapScreenFreeze):
			res.Unsupported = append(res.Unsupported, s.Hostname)
		case s.ID == presenting:
			res.Presenting = append(res.Presenting, s.Hostname)
		default:
			targets = append(targets, s)
		}
	}
	if len(targets) == 0 {
		return res, codedf(codeNoPauseTargets, "ningún alumno conectado de los elegidos puede pausar su pantalla")
	}
	a.pauses.SetMessage(req.Message)
	a.sendPause(class, targets, freeze, &res)
	a.log.Info("screens paused", "sent", res.Sent, "unsupported", len(res.Unsupported),
		"presenting", len(res.Presenting), "message_len", utf8.RuneCountInString(req.Message))
	return res, nil
}

// ChangePauseMessage sends the new message to every student in pause.
func (a *App) ChangePauseMessage(message string) (PauseResult, error) {
	class := a.currentClass()
	if class == nil {
		return PauseResult{}, errNoClass
	}
	freeze := protocol.ScreenFreeze{Message: message}
	if freeze.Validate() != nil {
		return PauseResult{}, codedf(codeInvalidPauseMessage, "el mensaje de la pausa tiene %d caracteres; el máximo es %d",
			utf8.RuneCountInString(message), protocol.MaxFreezeMessage)
	}
	res := newPauseResult()
	var targets []session.Session
	for _, s := range targetsOf(class.Sessions(), nil) {
		if a.pauses.Active(s.ID) {
			targets = append(targets, s)
		}
	}
	a.pauses.SetMessage(message)
	a.sendPause(class, targets, freeze, &res)
	a.log.Info("pause message changed", "sent", res.Sent, "message_len", utf8.RuneCountInString(message))
	return res, nil
}

// ResumeScreens removes the pause of the class or of the marked students.
func (a *App) ResumeScreens(hosts []string) PauseResult {
	res := newPauseResult()
	class := a.currentClass()
	if class == nil {
		return res
	}
	var targets []session.Session
	for _, s := range targetsOf(class.Sessions(), hosts) {
		if e, ok := a.pauses.Get(s.ID); ok && e.Status != screen.PauseResuming {
			targets = append(targets, s)
		}
	}
	a.sendPause(class, targets, protocol.ScreenUnfreeze{}, &res)
	a.log.Info("screens resumed", "sent", res.Sent)
	return res
}

// Pauses is the global strip's summary, for the first render.
func (a *App) Pauses() screen.PauseSummary { return a.pauses.Summary() }

// sendPause sends a screen.freeze or screen.unfreeze to each target and
// follows its ack; the strip learns who did not answer after AckWait.
func (a *App) sendPause(class classroom, targets []session.Session, p protocol.Payload, res *PauseResult) {
	_, freeze := p.(protocol.ScreenFreeze)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range targets {
		envID, err := class.Send(ctx, s.ID, p)
		if err != nil {
			a.log.Debug("pause not sent", "session", s.ID, "err", err)
			res.Failed = append(res.Failed, s.Hostname)
			continue
		}
		a.pauses.Sent(s.ID, envID, freeze)
		res.Sent++
	}
	a.emitPauses(class)
	if res.Sent > 0 {
		time.AfterFunc(screen.AckWait+100*time.Millisecond, func() {
			if c := a.currentClass(); c != nil {
				a.emitPauses(c)
			}
		})
	}
}

// unpauseBefore removes the pause of one student before the teacher takes
// control of or projects their screen: the teacher wants the real screen
// (docs/protocol.md §5.9). It reports whether a screen.unfreeze was sent.
func (a *App) unpauseBefore(class classroom, s session.Session) bool {
	if !a.pauses.Active(s.ID) {
		return false
	}
	res := newPauseResult()
	a.sendPause(class, []session.Session{s}, protocol.ScreenUnfreeze{}, &res)
	return res.Sent > 0
}

// onPauseMessage takes the agents' screen.freeze.ack and the errors that
// answer a pause.
func (a *App) onPauseMessage(sessionID string, env protocol.Envelope) {
	changed := false
	switch p := env.Payload.(type) {
	case protocol.ScreenFreezeAck:
		changed = a.pauses.Ack(sessionID, p)
	case protocol.ErrorPayload:
		changed = a.pauses.Error(sessionID, p.RefID)
	}
	if !changed {
		return
	}
	if class := a.currentClass(); class != nil {
		a.emitPauses(class)
	}
}

// syncPauses forgets the students that left: their agent removed the pause.
func (a *App) syncPauses(class classroom) {
	connected := map[string]bool{}
	for _, s := range class.Sessions() {
		if s.State == session.StateConnected {
			connected[s.ID] = true
		}
	}
	if a.pauses.Prune(connected) {
		a.emit(EventPauses, a.pauses.Summary())
	}
}

// emitPauses tells the window: the strip and the badges of the list.
func (a *App) emitPauses(class classroom) {
	a.emit(EventPauses, a.pauses.Summary())
	a.emit(EventStudents, a.studentsIn(class))
}

// markPauses fills each student's pause for the list and the Pantalla grande.
func (a *App) markPauses(list []Student) {
	now := time.Now()
	for i := range list {
		if e, ok := a.pauses.Get(list[i].ID); ok {
			list[i].Pause = string(e.Status)
			list[i].PauseUnacked = e.Unacked(now)
		}
	}
}
