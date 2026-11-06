package main

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/participation"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// --- Hands and messages of the students (docs/protocol.md §5.18) ------------------
//
// The «Participación» view: raised hands in order of arrival and the
// students' messages, in memory until the class closes (docs/privacidad.md
// rules 50-52). The text of a message or a note is never logged.

// ParticipationHand is a raised hand with the session it can be answered on.
type ParticipationHand struct {
	participation.Hand
	SessionID string `json:"sessionId"` // "" while the student is not connected
}

// ParticipationMessage is a student's message with the session it can be
// answered on.
type ParticipationMessage struct {
	participation.Message
	SessionID string `json:"sessionId"` // "" while the student is not connected
}

// Arrival tells the window that a hand or a message just arrived, for the
// floating notice. Never the text (rule 52).
type Arrival struct {
	Kind string `json:"kind"` // "hand" | "message"
	participation.Student
}

// Participation is the «Participación» view.
type Participation struct {
	Hands    []ParticipationHand    `json:"hands"`
	Messages []ParticipationMessage `json:"messages"`
	Pending  int                    `json:"pending"` // the number next to «Participación» in the menu
	Arrival  *Arrival               `json:"arrival"` // set only in the event that announces it
}

// ReplyRequest answers a student from «Participación»: a message (MessageID
// set: the notice carries reply_to) or a raised hand (MessageID empty).
type ReplyRequest struct {
	participation.Student
	MessageID string `json:"messageId"`
	Text      string `json:"text"`
}

// Participation returns the view.
func (a *App) Participation() Participation {
	return a.participation(a.currentClass(), nil)
}

func (a *App) participation(class classroom, arrival *Arrival) Participation {
	in := a.board.Inbox()
	out := Participation{Hands: make([]ParticipationHand, 0, len(in.Hands)),
		Messages: make([]ParticipationMessage, 0, len(in.Messages)), Pending: in.Pending, Arrival: arrival}
	sessions := participants(class)
	for _, h := range in.Hands {
		out.Hands = append(out.Hands, ParticipationHand{Hand: h, SessionID: sessions[h.Student].ID})
	}
	for _, m := range in.Messages {
		out.Messages = append(out.Messages, ParticipationMessage{Message: m, SessionID: sessions[m.Student].ID})
	}
	return out
}

// participants maps each connected student, real or simulated, to its
// session.
func participants(class classroom) map[participation.Student]session.Session {
	out := map[participation.Student]session.Session{}
	if class == nil {
		return out
	}
	add := func(list []session.Session, simulated bool) {
		for _, s := range list {
			if s.State == session.StateConnected {
				out[participation.Student{Hostname: s.Hostname, Username: s.Username, Simulated: simulated}] = s
			}
		}
	}
	add(class.Sessions(), false)
	add(simSessionsOf(class), true)
	return out
}

// participant finds who a session is, real or simulated.
func participant(class classroom, sessionID string) (participation.Student, session.Session, bool) {
	if s, ok := connectedSession(class.Sessions(), sessionID); ok {
		return participation.Student{Hostname: s.Hostname, Username: s.Username}, s, true
	}
	if s, ok := connectedSession(simSessionsOf(class), sessionID); ok {
		return participation.Student{Hostname: s.Hostname, Username: s.Username, Simulated: true}, s, true
	}
	return participation.Student{}, session.Session{}, false
}

// onClassroomMessage handles hand.raise, hand.lower and msg.student from a
// student, real or simulated (the session already checked the capability).
func (a *App) onClassroomMessage(sessionID string, env protocol.Envelope) {
	class := a.currentClass()
	if class == nil {
		return
	}
	who, _, ok := participant(class, sessionID)
	if !ok {
		return
	}
	log := a.log.With("conn", sessionID, "host", who.Hostname, "simulated", who.Simulated)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	send := func(p protocol.Payload) {
		if _, err := class.Send(ctx, sessionID, p); err != nil {
			log.Debug("cannot answer", "type", p.MessageType(), "err", err)
		}
	}
	tooFast := func() { send(protocol.NewError(env.ID, protocol.CodeInvalidMessage, "too fast")) }

	var arrival *Arrival
	switch p := env.Payload.(type) {
	case protocol.HandRaise:
		switch a.board.Raise(who, p) {
		case participation.Added:
			log.Info("hand raised", "hand", p.HandID, "note_len", utf8.RuneCountInString(p.Note))
			arrival = &Arrival{Kind: "hand", Student: who}
		case participation.Repeated:
			log.Debug("hand raised again", "hand", p.HandID)
		case participation.LoweredBefore:
			send(protocol.HandLower{HandID: p.HandID, By: protocol.HandByTeacher})
			return
		case participation.TooFast:
			tooFast()
			return
		}
	case protocol.HandLower:
		if err := p.ValidFrom(protocol.HandByStudent); err != nil {
			send(protocol.NewError(env.ID, protocol.CodeInvalidMessage, "by must be student"))
			return
		}
		lowered, ok := a.board.LowerByStudent(who, p)
		if !ok {
			tooFast()
			return
		}
		if !lowered {
			return
		}
		log.Info("hand lowered by the student", "hand", p.HandID)
	case protocol.MsgStudent:
		switch a.board.Message(who, p) {
		case participation.Added:
			log.Info("student message", "message", p.MessageID, "len", utf8.RuneCountInString(p.Text))
			arrival = &Arrival{Kind: "message", Student: who}
		case participation.TooFast:
			tooFast()
			return
		}
		send(protocol.MsgStudentAck{RefID: env.ID, MessageID: p.MessageID})
	default:
		return
	}
	a.emitParticipation(class, arrival)
}

func (a *App) emitParticipation(class classroom, arrival *Arrival) {
	a.emit(EventParticipation, a.participation(class, arrival))
	if class != nil {
		a.emit(EventStudents, a.studentsIn(class)) // the «Mano levantada» badge
	}
}

// LowerHand lowers a student's hand («Bajar la mano»). If the student is
// connected, their window shows it lowered; otherwise the agent learns it
// when it raises that hand again after reconnecting.
func (a *App) LowerHand(who participation.Student) error {
	class := a.currentClass()
	if class == nil {
		return errNoClass
	}
	handID, ok := a.board.LowerByTeacher(who)
	if !ok {
		return codedf(codeHandGone, "%s ya no tiene la mano levantada", who.Hostname)
	}
	a.sendLower(class, who, handID)
	a.emitParticipation(class, nil)
	return nil
}

// LowerAllHands lowers every raised hand («Bajar todas») and returns how many.
func (a *App) LowerAllHands() (int, error) {
	class := a.currentClass()
	if class == nil {
		return 0, errNoClass
	}
	hands := a.board.LowerAll()
	for _, h := range hands {
		a.sendLower(class, h.Student, h.HandID)
	}
	a.emitParticipation(class, nil)
	return len(hands), nil
}

func (a *App) sendLower(class classroom, who participation.Student, handID string) {
	s, ok := participants(class)[who]
	a.log.Info("hand lowered by the teacher", "host", who.Hostname, "hand", handID, "connected", ok)
	if !ok || !s.Can(protocol.CapHand) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := class.Send(ctx, s.ID, protocol.HandLower{HandID: handID, By: protocol.HandByTeacher}); err != nil {
		a.log.Debug("cannot send hand.lower", "conn", s.ID, "err", err)
	}
}

// ReplyToStudent answers one student («Responder»): a notice (msg.show) to
// that student only, with reply_to when it answers a message. The delivery is
// tracked like any other notice.
func (a *App) ReplyToStudent(req ReplyRequest) (codedist.Delivery, error) {
	class := a.currentClass()
	if class == nil {
		return codedist.Delivery{}, errNoClass
	}
	text := strings.TrimSpace(req.Text)
	// To this student only: the window titles it «Mensaje para ti» or, with
	// reply_to, «Respuesta a tu mensaje» (rule 53).
	p := protocol.MsgShow{Text: text, Level: protocol.LevelInfo, Personal: true}
	if req.MessageID != "" {
		if _, ok := a.board.Lookup(req.Student, req.MessageID); !ok {
			return codedist.Delivery{}, codedf(codeMessageGone, "ese mensaje ya no está en la lista")
		}
		p.ReplyTo = req.MessageID
	}
	if err := p.Validate(); err != nil {
		return codedist.Delivery{}, friendly(err)
	}
	s, ok := participants(class)[req.Student]
	if !ok {
		return codedist.Delivery{}, codedf(codeStudentGone, "%s no está conectado", req.Hostname)
	}
	label := "Respuesta a " + req.Hostname
	d, err := a.broadcastTo("notice", label, p, []string{s.ID})
	if err != nil {
		return d, err
	}
	if req.MessageID != "" {
		a.board.Replied(req.Student, req.MessageID, text)
	}
	a.log.Info("reply to a student", "host", req.Hostname, "message", req.MessageID, "len", utf8.RuneCountInString(text))
	a.emitParticipation(class, nil)
	return d, nil
}

// MarkMessageDone marks a message as seen without an answer («Marcar como
// atendido»).
func (a *App) MarkMessageDone(who participation.Student, messageID string) error {
	if !a.board.Done(who, messageID) {
		return codedf(codeMessageGone, "ese mensaje ya no está en la lista")
	}
	a.emitParticipation(a.currentClass(), nil)
	return nil
}

// markParticipation puts each student's place in line and new messages on
// their row (the badge in Notas).
func (a *App) markParticipation(list []Student) {
	in := a.board.Inbox()
	place := map[participation.Student]int{}
	for _, h := range in.Hands {
		place[h.Student] = h.Position
	}
	unread := map[participation.Student]int{}
	for _, m := range in.Messages {
		if m.Status == participation.MessageNew {
			unread[m.Student]++
		}
	}
	for i := range list {
		k := participation.Student{Hostname: list[i].Hostname, Username: list[i].Username, Simulated: list[i].Simulated}
		list[i].Hand, list[i].NewMessages = place[k], unread[k]
	}
}
