package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/participation"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

func envOf(p protocol.Payload) protocol.Envelope { return protocol.New(p) }

func participationHarness(t *testing.T) (*harness, *bytes.Buffer) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapMsgShow, protocol.CapHand, protocol.CapMsgStudent),
		student("s2", "PC02", protocol.CapMsgShow, protocol.CapHand, protocol.CapMsgStudent),
		student("s3", "PC03", protocol.CapMsgShow)) // an older agent
	logs := &bytes.Buffer{}
	h.app.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return h, logs
}

func key(host string) participation.Student { return participation.Student{Hostname: host} }

// Two hands in order of arrival, the badge on each row, the floating notice,
// and «Bajar la mano» sent to the student.
func TestHandsLineUpAndTheTeacherLowersThem(t *testing.T) {
	h, logs := participationHarness(t)
	second, first := protocol.NewID(), protocol.NewID()
	h.hooks.onClassroom("s2", envOf(protocol.HandRaise{HandID: first, Note: "no compila el for"}))
	h.hooks.onClassroom("s1", envOf(protocol.HandRaise{HandID: second}))

	p := h.rec.last(EventParticipation).(Participation)
	if len(p.Hands) != 2 || p.Hands[0].Hostname != "PC02" || p.Hands[1].Hostname != "PC01" || p.Pending != 2 {
		t.Fatalf("hands = %+v", p.Hands)
	}
	if p.Hands[0].SessionID != "s2" || p.Hands[1].Position != 2 {
		t.Errorf("session or position: %+v", p.Hands)
	}
	if p.Arrival == nil || p.Arrival.Kind != "hand" || p.Arrival.Hostname != "PC01" {
		t.Errorf("arrival = %+v", p.Arrival)
	}
	rows := h.rec.last(EventStudents).(Students).Students
	if rows[0].Hand != 2 || rows[1].Hand != 1 || rows[2].Hand != 0 || !rows[0].CanHand || rows[2].CanHand {
		t.Errorf("badges = %d %d %d", rows[0].Hand, rows[1].Hand, rows[2].Hand)
	}

	h.class.takeDirect()
	if err := h.app.LowerHand(key("PC02")); err != nil {
		t.Fatal(err)
	}
	sent := h.class.takeDirect()
	if len(sent) != 1 || sent[0].sessionID != "s2" {
		t.Fatalf("sent %+v", sent)
	}
	if low, ok := sent[0].p.(protocol.HandLower); !ok || low.HandID != first || low.By != protocol.HandByTeacher {
		t.Errorf("hand.lower = %+v", sent[0].p)
	}
	if err := h.app.LowerHand(key("PC02")); errorCode(err) != codeHandGone {
		t.Errorf("lower again = %v", err)
	}
	// The agent resends the lowered hand after reconnecting: lowered again,
	// not queued.
	h.hooks.onClassroom("s2", envOf(protocol.HandRaise{HandID: first}))
	if d := h.class.takeDirect(); len(d) != 1 || d[0].p.(protocol.HandLower).HandID != first {
		t.Errorf("resent lowered hand answered %+v", d)
	}
	if n, err := h.app.LowerAllHands(); n != 1 || err != nil {
		t.Errorf("LowerAllHands = %d, %v", n, err)
	}
	if strings.Contains(logs.String(), "no compila") {
		t.Error("the note reached the log")
	}
}

// A message is acked, answered with msg.show reply_to to that student only,
// and its text never reaches the log.
func TestStudentMessageIsAckedAndAnswered(t *testing.T) {
	h, logs := participationHarness(t)
	m := protocol.MsgStudent{MessageID: protocol.NewID(), Text: "¿El ejercicio 3 es para hoy?"}
	env := envOf(m)
	h.hooks.onClassroom("s1", env)
	h.hooks.onClassroom("s1", env) // resent after a reconnection: acked again, once in the list
	sent := h.class.takeDirect()
	if len(sent) != 2 {
		t.Fatalf("sent %d, want 2 acks", len(sent))
	}
	for _, s := range sent {
		if ack, ok := s.p.(protocol.MsgStudentAck); !ok || ack.RefID != env.ID || ack.MessageID != m.MessageID {
			t.Errorf("ack = %+v", s.p)
		}
	}
	p := h.app.Participation()
	if len(p.Messages) != 1 || p.Messages[0].Status != participation.MessageNew || p.Pending != 1 {
		t.Fatalf("messages = %+v", p.Messages)
	}

	d, err := h.app.ReplyToStudent(ReplyRequest{Student: key("PC01"), MessageID: m.MessageID, Text: " Sí, antes de las 12. "})
	if err != nil {
		t.Fatal(err)
	}
	sent = h.class.takeDirect()
	if len(sent) != 1 || sent[0].sessionID != "s1" || d.Sent != 1 {
		t.Fatalf("reply sent %+v, delivery %+v", sent, d)
	}
	if show := sent[0].p.(protocol.MsgShow); show.ReplyTo != m.MessageID || !show.Personal || show.Text != "Sí, antes de las 12." {
		t.Errorf("msg.show = %+v", show)
	}
	if got := h.app.Participation(); got.Messages[0].Status != participation.MessageReplied || got.Pending != 0 {
		t.Errorf("after reply: %+v", got.Messages[0])
	}
	if _, err := h.app.ReplyToStudent(ReplyRequest{Student: key("PC01"), MessageID: protocol.NewID(), Text: "x"}); errorCode(err) != codeMessageGone {
		t.Errorf("reply to an unknown message = %v", err)
	}
	if _, err := h.app.ReplyToStudent(ReplyRequest{Student: key("PC09"), Text: "x"}); errorCode(err) != codeStudentGone {
		t.Errorf("reply to someone absent = %v", err)
	}
	if out := logs.String(); strings.Contains(out, "ejercicio 3") || strings.Contains(out, "antes de las 12") {
		t.Errorf("message text in the log:\n%s", out)
	}
}

func TestStudentMessagesArePacedAndMarkedDone(t *testing.T) {
	h, _ := participationHarness(t)
	var last protocol.Envelope
	for range participation.MessageBurst + 1 {
		last = envOf(protocol.MsgStudent{MessageID: protocol.NewID(), Text: "hola"})
		h.hooks.onClassroom("s2", last)
	}
	sent := h.class.takeDirect()
	e, ok := sent[len(sent)-1].p.(protocol.ErrorPayload)
	if !ok || e.Code != protocol.CodeInvalidMessage || e.RefID != last.ID {
		t.Fatalf("over the pace answered %+v", sent[len(sent)-1].p)
	}
	msgs := h.app.Participation().Messages
	if len(msgs) != participation.MessageBurst {
		t.Fatalf("%d messages kept", len(msgs))
	}
	if err := h.app.MarkMessageDone(key("PC02"), msgs[0].MessageID); err != nil {
		t.Fatal(err)
	}
	if p := h.app.Participation(); p.Pending != participation.MessageBurst-1 {
		t.Errorf("pending after done = %d", p.Pending)
	}
	// A hand.lower must say by student.
	h.hooks.onClassroom("s2", envOf(protocol.HandLower{HandID: protocol.NewID(), By: protocol.HandByTeacher}))
	if d := h.class.takeDirect(); len(d) != 1 || d[0].p.(protocol.ErrorPayload).Code != protocol.CodeInvalidMessage {
		t.Errorf("hand.lower by teacher from an agent answered %+v", d)
	}
}

// Closing the class forgets everything (docs/privacidad.md rule 51).
func TestParticipationEndsWithTheClass(t *testing.T) {
	h, _ := participationHarness(t)
	h.hooks.onClassroom("s1", envOf(protocol.HandRaise{HandID: protocol.NewID()}))
	h.hooks.onClassroom("s1", envOf(protocol.MsgStudent{MessageID: protocol.NewID(), Text: "x"}))
	h.app.EndClass()
	if p := h.rec.last(EventParticipation).(Participation); len(p.Hands)+len(p.Messages) != 0 {
		t.Errorf("after EndClass: %+v", p)
	}
	// Someone unknown (not connected) is ignored.
	h.class.sessions = append(h.class.sessions, session.Session{ID: "gone", Hostname: "X", State: session.StateDisconnected})
	h.hooks.onClassroom("gone", envOf(protocol.HandRaise{HandID: protocol.NewID()}))
	if p := h.app.Participation(); len(p.Hands) != 0 {
		t.Error("a hand from a disconnected session was queued")
	}
}
