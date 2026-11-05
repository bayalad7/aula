package simclass

import (
	"context"
	"slices"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// SampleMessages are what a simulated student writes when the teacher does
// not give a text: questions a class would ask.
var SampleMessages = []string{
	"¿El ejercicio es para hoy?",
	"No me compila, ¿me ayuda?",
	"Ya terminé, ¿qué sigue?",
	"¿Puede repetir el último paso?",
}

// Raise makes a simulated student raise its hand (up) or lower it: what a
// real student does with «Levantar la mano» (docs/protocol.md §5.18).
func (c *Class) Raise(id string, up bool) error {
	s := c.find(id)
	if s == nil {
		return ErrUnknownStudent
	}
	if !slices.Contains(s.caps, protocol.CapHand) {
		return ErrBadCapability
	}
	cl := s.connection()
	if cl == nil {
		return ErrNotConnected
	}
	var p protocol.Payload
	if up {
		r, err := s.desk.Raise("")
		if err != nil {
			return err
		}
		p = r
	} else {
		l, ok, err := s.desk.Lower()
		if err != nil || !ok {
			return err
		}
		p = l
	}
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()
	s.send(ctx, cl, p)
	c.changed()
	return nil
}

// Write makes a simulated student write to the teacher; an empty text picks
// one of SampleMessages.
func (c *Class) Write(id, text string) error {
	s := c.find(id)
	if s == nil {
		return ErrUnknownStudent
	}
	if !slices.Contains(s.caps, protocol.CapMsgStudent) {
		return ErrBadCapability
	}
	cl := s.connection()
	if cl == nil {
		return ErrNotConnected
	}
	if text == "" {
		s.mu.Lock()
		text = SampleMessages[s.sent%len(SampleMessages)]
		s.mu.Unlock()
	}
	p, err := s.desk.Write(text)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.sent++
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()
	s.send(ctx, cl, p)
	c.changed()
	return nil
}

func (s *student) connection() *transport.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cl
}

func (s *student) send(ctx context.Context, cl *transport.Client, p protocol.Payload) {
	id, err := cl.Send(ctx, p)
	if err != nil {
		s.class.log.Debug("cannot send", "student", s.name, "type", p.MessageType(), "err", err)
		return
	}
	s.desk.Sending(id, p)
}

// handleParticipation takes hand.lower, msg.student.ack and the errors to a
// hand or a message, like the real app.
func (s *student) handleParticipation(env protocol.Envelope) bool {
	var changed, handled bool
	switch p := env.Payload.(type) {
	case protocol.HandLower:
		changed, handled = s.desk.LoweredByTeacher(p), true
	case protocol.MsgStudentAck:
		changed, handled = s.desk.Acked(p), true
	case protocol.ErrorPayload:
		changed = s.desk.Refused(p)
		handled = changed
	}
	if changed {
		s.class.changed()
	}
	return handled
}
