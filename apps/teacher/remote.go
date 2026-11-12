package main

import (
	"context"
	"errors"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/remote"
	"github.com/bayalad7/aula/internal/screen"
	"github.com/bayalad7/aula/internal/session"
)

// --- Remote control of one student (docs/protocol.md §5.16) ----------------------

// TakeControl starts controlling a student's computer: the agent shows its
// band and the native notice, then sends its screen live. One at a time;
// never a simulated student (it cannot declare remote).
func (a *App) TakeControl(sessionID string) (remote.Control, error) {
	class := a.currentClass()
	if class == nil {
		return remote.Control{}, errNoClass
	}
	s, ok := connectedSession(class.Sessions(), sessionID) // real students only
	if !ok || !s.Can(protocol.CapRemote) {
		return remote.Control{}, codedf(codeRemoteUnsupported, "el equipo de este alumno no permite el control remoto")
	}
	// The teacher wants the real screen: a paused one is resumed first (§5.9).
	if a.unpauseBefore(class, s) {
		a.log.Info("pause removed to take control", "session", s.ID)
	}
	start, c, err := a.desk.Take(s.ID, s.Hostname)
	if errors.Is(err, remote.ErrBusy) {
		return c, codedf(codeAlreadyControlling, "ya controlas %s: suéltalo antes de ayudar a otro alumno", c.Hostname)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	envID, err := class.Send(ctx, s.ID, start)
	if err != nil {
		c, _ = a.desk.Gone(s.ID)
		a.emit(EventRemote, c)
		return c, codedf(codeRemoteGone, "no se pudo llegar al equipo de %s: %v", s.Hostname, err)
	}
	a.desk.Started(envID)
	a.log.Info("remote control requested", "session", s.ID, "control_id", c.ControlID)
	a.emit(EventRemote, c)
	return c, nil
}

// ReleaseControl ends the control in force. The band disappears from the
// student's computer when its agent confirms (remote.ack ended).
func (a *App) ReleaseControl() error {
	class := a.currentClass()
	if class == nil {
		return errNoClass
	}
	sessionID, stop, err := a.desk.Release()
	if err != nil {
		return codedf(codeRemoteGone, "no hay ningún equipo controlado")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := class.Send(ctx, sessionID, stop); err != nil {
		// The connection is gone: the agent ended the control by itself.
		c, _ := a.desk.Gone(sessionID)
		a.emit(EventRemote, c)
	}
	a.log.Info("remote control released", "session", sessionID)
	return nil
}

// ShowControl says whether the control view is open: closed, the control
// goes on (its band stays on the console) with a slower video and no input.
func (a *App) ShowControl(visible bool) error {
	class := a.currentClass()
	if class == nil {
		return errNoClass
	}
	sessionID, start, err := a.desk.View(visible)
	if err != nil {
		return codedf(codeRemoteGone, "no hay ningún equipo controlado")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := class.Send(ctx, sessionID, start); err != nil {
		a.log.Debug("remote video not retuned", "err", err)
	}
	a.emit(EventRemote, a.desk.Current())
	return nil
}

// SendRemoteInput sends the teacher's keyboard and mouse to the controlled
// computer, batched and paced by the desk (at most 60 messages per second).
// Events the protocol does not accept (a key out of its list) are skipped.
func (a *App) SendRemoteInput(events []protocol.InputEvent) error {
	class := a.currentClass()
	if class == nil {
		return errNoClass
	}
	valid := events[:0:0]
	for _, e := range events {
		if (protocol.RemoteInput{ControlID: protocol.NewID(), Seq: 1, Events: []protocol.InputEvent{e}}).Validate() == nil {
			valid = append(valid, e)
		}
	}
	sessionID, batches, err := a.desk.Input(valid)
	if err != nil {
		return codedf(codeRemoteGone, "no hay ningún equipo controlado")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, b := range batches {
		if _, err := class.Send(ctx, sessionID, b); err != nil {
			return codedf(codeRemoteGone, "se perdió la conexión con el equipo controlado: %v", err)
		}
	}
	return nil
}

// onRemoteMessage takes remote.ack and the errors that answer a remote.start.
func (a *App) onRemoteMessage(sessionID string, env protocol.Envelope) {
	var (
		c       remote.Control
		changed bool
	)
	switch p := env.Payload.(type) {
	case protocol.RemoteAck:
		c, changed = a.desk.Ack(sessionID, p)
	case protocol.ErrorPayload:
		c, changed = a.desk.Error(sessionID, p.RefID)
	}
	if changed {
		a.log.Info("remote control", "session", sessionID, "status", c.Status, "reason", c.Reason)
		a.emit(EventRemote, c)
	}
}

// onRemoteFrame shows a frame of the controlled screen; it reports whether
// the frame was the control's (and not a thumbnail).
func (a *App) onRemoteFrame(sessionID string, f protocol.ScreenThumbFrame) bool {
	if !a.desk.Frame(sessionID, f.RefID) {
		return false
	}
	a.emit(EventRemoteFrame, screen.Thumb{SessionID: sessionID, Seq: f.Seq, Width: f.Width, Height: f.Height,
		JPEG: f.JPEG, Detail: true, At: a.now()})
	return true
}

// syncRemote ends the control of a student who left: the agent already
// ended it when its connection closed.
func (a *App) syncRemote(sessions []session.Session) {
	c := a.desk.Current()
	if !c.Live() {
		return
	}
	if _, ok := connectedSession(sessions, c.SessionID); ok {
		return
	}
	if c, ok := a.desk.Gone(c.SessionID); ok {
		a.log.Info("remote control ended: the student disconnected", "session", c.SessionID)
		a.emit(EventRemote, c)
	}
}

func connectedSession(list []session.Session, id string) (session.Session, bool) {
	for _, s := range list {
		if s.ID == id && s.State == session.StateConnected {
			return s, true
		}
	}
	return session.Session{}, false
}

// endControl releases the control when the class closes; the agent would end
// it anyway when the connection closes.
func (a *App) endControl(class classroom) {
	sessionID, stop, err := a.desk.Release()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = class.Send(ctx, sessionID, stop)
	if c, ok := a.desk.Gone(sessionID); ok {
		a.emit(EventRemote, c)
	}
}
