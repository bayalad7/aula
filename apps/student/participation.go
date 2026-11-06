package main

import (
	"context"
	"errors"
	"time"

	"github.com/bayalad7/aula/internal/participation"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// --- Raise the hand and write to the teacher (docs/protocol.md §5.18) -------------
//
// The student acts: nothing here takes the focus or brings the window
// forward (prompt 19), and there is no band (it does not observe or control
// this computer). Go sends states; the window words them.

// EventParticipation carries a participation.DeskView whenever the hand goes
// up or down or a message is sent, acknowledged or refused.
const EventParticipation = "aula:participation"

// Errors of the participation bindings. The window shows mano.error.<code>
// (textos.es-MX.js); on the wire they read "aula:<code>".
const (
	partNoConnection = "noConnection" // not connected to a class
	partEmpty        = "empty"
	partTooLong      = "tooLong"
	partTooSoon      = "tooSoon"
	partWaiting      = "waiting"     // MaxAwaitingAck messages wait for their ack
	partUnsupported  = "unsupported" // an older console
	partNotSent      = "notSent"     // the connection dropped while sending
)

// partErrorCodes lists them all, for the texts test.
var partErrorCodes = []string{partNoConnection, partEmpty, partTooLong, partTooSoon, partWaiting, partUnsupported, partNotSent}

type partError string

func (e partError) Error() string { return "aula:" + string(e) }

func partErrorOf(err error) error {
	switch {
	case errors.Is(err, participation.ErrEmpty):
		return partError(partEmpty)
	case errors.Is(err, participation.ErrTooLong):
		return partError(partTooLong)
	case errors.Is(err, participation.ErrTooSoon):
		return partError(partTooSoon)
	case errors.Is(err, participation.ErrWaiting):
		return partError(partWaiting)
	case errors.Is(err, participation.ErrUnsupported):
		return partError(partUnsupported)
	}
	return err
}

// onConnect keeps the connection the agent may send on and sends again the
// raised hand and the messages without an ack, with their IDs.
func (a *App) onConnect(ctx context.Context, cl *transport.Client) {
	a.mu.Lock()
	a.cl = cl
	a.mu.Unlock()
	a.timerReconnected() // before the console can send it again (§5.19)
	for _, p := range a.desk.Reconnected() {
		a.sendParticipation(ctx, cl, p)
	}
	a.emitParticipation()
}

// connection is the connection with the console, nil while not connected.
func (a *App) connection() *transport.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cl
}

func (a *App) sendParticipation(ctx context.Context, cl *transport.Client, p protocol.Payload) error {
	id, err := cl.Send(ctx, p)
	if err != nil {
		a.log.Debug("cannot send", "type", p.MessageType(), "err", err)
		return err
	}
	a.desk.Sending(id, p)
	return nil
}

// handleParticipation takes what the console answers: hand.lower,
// msg.student.ack and the errors to a hand or a message.
func (a *App) handleParticipation(env protocol.Envelope) bool {
	switch p := env.Payload.(type) {
	case protocol.HandLower:
		if p.ValidFrom(protocol.HandByTeacher) != nil {
			return true
		}
		if a.desk.LoweredByTeacher(p) {
			a.log.Info("hand lowered by the teacher", "hand", p.HandID)
			a.emitParticipation()
			a.notify() // never raise: it is only news for the student
		}
		return true
	case protocol.MsgStudentAck:
		if a.desk.Acked(p) {
			a.emitParticipation()
		}
		return true
	case protocol.ErrorPayload:
		if a.desk.Refused(p) {
			a.log.Info("the console refused a hand or a message", "code", p.Code)
			a.emitParticipation()
			return true
		}
	}
	return false
}

func (a *App) emitParticipation() { a.emit(EventParticipation, a.desk.View()) }

// send sends a payload of the desk now, or fails with noConnection.
func (a *App) sendNow(p protocol.Payload) error {
	cl := a.connection()
	if cl == nil {
		return partError(partNoConnection)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.sendParticipation(ctx, cl, p); err != nil {
		return partError(partNotSent)
	}
	return nil
}

// RaiseHand raises the student's hand, with an optional short reason («pedir
// ayuda»). Raising it again only updates the reason: it keeps its place.
func (a *App) RaiseHand(note string) (participation.DeskView, error) {
	if a.remote != nil {
		var v participation.DeskView
		err := a.forward("RaiseHand", &v, note)
		return v, err
	}
	if a.connection() == nil {
		return a.desk.View(), partError(partNoConnection)
	}
	p, err := a.desk.Raise(note)
	if err != nil {
		return a.desk.View(), partErrorOf(err)
	}
	if err := a.sendNow(p); err != nil {
		// Still up: it is sent again on the next connection.
		a.emitParticipation()
		return a.desk.View(), err
	}
	a.log.Info("hand raised", "hand", p.HandID)
	a.emitParticipation()
	return a.desk.View(), nil
}

// LowerHand lowers the student's own hand.
func (a *App) LowerHand() (participation.DeskView, error) {
	if a.remote != nil {
		var v participation.DeskView
		err := a.forward("LowerHand", &v)
		return v, err
	}
	p, ok, err := a.desk.Lower()
	if err != nil {
		return a.desk.View(), partErrorOf(err)
	}
	if ok {
		a.log.Info("hand lowered by the student", "hand", p.HandID)
		// Not connected: the console keeps it until the teacher lowers it or
		// the class closes; nothing else to do.
		_ = a.sendNow(p)
		a.emitParticipation()
	}
	return a.desk.View(), nil
}

// SendTeacherMessage sends a short message to the teacher («Mensaje al
// profesor»). Its text never reaches the log.
func (a *App) SendTeacherMessage(text string) (participation.DeskView, error) {
	if a.remote != nil {
		var v participation.DeskView
		err := a.forward("SendTeacherMessage", &v, text)
		return v, err
	}
	if a.connection() == nil {
		return a.desk.View(), partError(partNoConnection)
	}
	p, err := a.desk.Write(text)
	if err != nil {
		return a.desk.View(), partErrorOf(err)
	}
	// If it cannot go now it waits, "sending", for the next connection.
	sendErr := a.sendNow(p)
	a.log.Info("message to the teacher", "message", p.MessageID, "bytes", len(p.Text))
	a.emitParticipation()
	return a.desk.View(), sendErr
}
