package main

import (
	"context"
	"errors"

	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// --- Showing a student's screen to the class (docs/protocol.md §5.24) ----------------
//
// The teacher picks a student in their card or in the big screen; the
// student is not asked and cannot stop it, as with a remote control
// (docs/privacidad.md rules 75-77): their window shows the band first. The
// console receives their screen and forwards it to every other student with
// screen.present.view, and to its own big screen. It shares the UDP port and
// the students' view with the teacher's broadcast: one at a time.

// Events of a presentation (docs/eventos-ui.md).
const (
	// EventPresent carries a PresentState when it starts, changes phase or
	// counts, or ends (then Ended says how).
	EventPresent = "aula:present"
	// EventPresentView carries a broadcast.View for the console's own video
	// of the presentation (the big screen), and EventPresentSignal a
	// broadcast.Signal for its RTCPeerConnection.
	EventPresentView   = "aula:present-view"
	EventPresentSignal = "aula:present-signal"
)

// PresentState is the presentation as the console shows it: the strip
// «Proyectando LAB3-PC07 · N viendo · Detener» and the notice when it ends.
type PresentState struct {
	Active    bool   `json:"active"`
	SessionID string `json:"sessionId"`
	Hostname  string `json:"hostname"`
	// Phase: connecting or live.
	Phase     string `json:"phase"`
	Receivers int    `json:"receivers"`
	Eligible  int    `json:"eligible"`
	// Ended, when not Active: stopped, failed or source_left; Reason details
	// failed (capture_unavailable, encoder_unavailable, busy, no_answer).
	Ended  string `json:"ended"`
	Reason string `json:"reason"`
}

func presentStateOf(s broadcast.ProjectStatus) PresentState {
	return PresentState{Active: s.Active, SessionID: s.SourceID, Hostname: s.Hostname, Phase: s.Phase,
		Receivers: s.Receivers, Eligible: s.Eligible, Ended: s.Ended, Reason: s.Reason}
}

// newProjector wires the presentation to this app: messages go through the
// open class, the state and the console's own video reach the frontend.
func (a *App) newProjector(maxKbps int) *broadcast.Projector {
	return &broadcast.Projector{
		Send: func(ctx context.Context, sessionID string, p protocol.Payload) (string, error) {
			a.mu.Lock()
			class := a.presClass
			a.mu.Unlock()
			if class == nil {
				return "", errNoClass
			}
			return class.Send(ctx, sessionID, p)
		},
		OnStatus: func(s broadcast.ProjectStatus) { a.emit(EventPresent, presentStateOf(s)) },
		OnView:   func(v broadcast.View) { a.emit(EventPresentView, v) },
		OnSignal: func(s broadcast.Signal) { a.emit(EventPresentSignal, s) },
		MaxKbps:  maxKbps,
		Logger:   a.log,
	}
}

// StartPresent shows the screen of the student of sessionID to the class. The teacher's broadcast, if any, ends first: the frontend confirmed
// it (both use the students' view and the same UDP port).
func (a *App) StartPresent(sessionID string) (PresentState, error) {
	class := a.currentClass()
	if class == nil {
		return a.PresentState(), errNoClass
	}
	src, ok := sessionByID(class.Sessions(), sessionID)
	if !ok {
		return a.PresentState(), codedf(codeStudentGone, "ese alumno ya no está conectado")
	}
	if !src.Can(protocol.CapScreenPresent) {
		return a.PresentState(), codedf(codePresentUnsupported, "el equipo de %s no puede mostrar su pantalla a la clase", src.Hostname)
	}
	if a.bc.Status().Active {
		a.StopBroadcast()
	}
	a.bcMu.Lock()
	defer a.bcMu.Unlock()
	a.proj.Media.UDPPort = a.broadcastUDPPort
	a.mu.Lock()
	a.presClass = class
	a.mu.Unlock()
	if err := a.proj.Start(src, class.Sessions()); err != nil {
		a.log.Warn("presentation not started", "conn", sessionID, "err", err)
		if errors.Is(err, broadcast.ErrPresentUnsupported) {
			return a.PresentState(), codedf(codePresentUnsupported, "el equipo de %s no puede mostrar su pantalla a la clase", src.Hostname)
		}
		return a.PresentState(), codedf(codePresentFailed, "no se pudo empezar a proyectar: %w", err)
	}
	st := a.PresentState()
	a.emit(EventPresent, st)
	return st, nil
}

// StopPresent ends the presentation in every computer. EndClass calls it
// before closing the class, so the stop messages still go out.
func (a *App) StopPresent() PresentState {
	a.bcMu.Lock()
	defer a.bcMu.Unlock()
	a.proj.Stop()
	a.mu.Lock()
	a.presClass = nil
	a.mu.Unlock()
	return a.PresentState()
}

// PresentState returns the presentation in force.
func (a *App) PresentState() PresentState { return presentStateOf(a.proj.Status()) }

// OpenPresentView shows the presentation in the console's big screen: the
// console becomes one more receiver (not counted among the students).
func (a *App) OpenPresentView() { a.proj.OpenView() }

// ClosePresentView closes the console's view; the presentation goes on.
func (a *App) ClosePresentView() { a.proj.CloseView() }

// PresentViewAnswer, PresentViewCandidate and PresentViewFailed are the
// console view's RTCPeerConnection, as the student's window does with
// BroadcastAnswer, BroadcastCandidate and BroadcastFailed.
func (a *App) PresentViewAnswer(broadcastID string, attempt int, sdp string) error {
	return a.proj.ViewAnswer(broadcastID, attempt, sdp)
}

// PresentViewCandidate hands over one of the console view's ICE candidates.
func (a *App) PresentViewCandidate(broadcastID string, attempt int, candidate, mid string, mline int) error {
	return a.proj.ViewCandidate(broadcastID, attempt, candidate, mid, mline)
}

// PresentViewFailed says the console view could not open the offer.
func (a *App) PresentViewFailed(broadcastID string, attempt int) error {
	return a.proj.ViewFailed(broadcastID, attempt)
}

// sessionByID finds a connected session.
func sessionByID(sessions []session.Session, id string) (session.Session, bool) {
	for _, s := range sessions {
		if s.ID == id && s.State == session.StateConnected {
			return s, true
		}
	}
	return session.Session{}, false
}
