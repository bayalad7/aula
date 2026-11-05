package participation

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time               { return c.t }
func (c *clock) add(d time.Duration)          { c.t = c.t.Add(d) }
func newClock() *clock                        { return &clock{t: time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)} }
func student(h string) Student                { return Student{Hostname: h, Username: "alumno"} }
func raise(id string) protocol.HandRaise      { return protocol.HandRaise{HandID: id} }
func msg(id, text string) protocol.MsgStudent { return protocol.MsgStudent{MessageID: id, Text: text} }

func newTestBoard() (*Board, *clock) {
	c := newClock()
	b := NewBoard()
	b.now = c.now
	return b, c
}

func TestHandsKeepTheOrderOfArrival(t *testing.T) {
	b, c := newTestBoard()
	a, z, m := student("PC-A"), student("PC-Z"), student("PC-M")
	ids := map[Student]string{a: protocol.NewID(), z: protocol.NewID(), m: protocol.NewID()}
	for _, s := range []Student{z, a, m} {
		if got := b.Raise(s, raise(ids[s])); got != Added {
			t.Fatalf("Raise %s = %v, want Added", s.Hostname, got)
		}
		c.add(time.Second)
	}
	// A resend after a reconnection keeps the place and takes the new note.
	if got := b.Raise(z, protocol.HandRaise{HandID: ids[z], Note: "ayuda"}); got != Repeated {
		t.Fatalf("repeated raise = %v", got)
	}
	in := b.Inbox()
	want := []string{"PC-Z", "PC-A", "PC-M"}
	for i, h := range in.Hands {
		if h.Hostname != want[i] || h.Position != i+1 {
			t.Errorf("hand %d = %s at %d, want %s at %d", i, h.Hostname, h.Position, want[i], i+1)
		}
	}
	if in.Hands[0].Note != "ayuda" || in.Pending != 3 {
		t.Errorf("note %q, pending %d", in.Hands[0].Note, in.Pending)
	}

	// The student lowers theirs: the others move up.
	if lowered, ok := b.LowerByStudent(z, protocol.HandLower{HandID: ids[z], By: protocol.HandByStudent}); !lowered || !ok {
		t.Fatal("LowerByStudent did not lower the current hand")
	}
	if h, ok := b.HandOf(m); !ok || h.Position != 2 {
		t.Errorf("PC-M at %d after PC-Z left, want 2", h.Position)
	}
	// A stale hand_id is ignored without error.
	c.add(time.Second)
	if lowered, ok := b.LowerByStudent(a, protocol.HandLower{HandID: protocol.NewID(), By: protocol.HandByStudent}); lowered || !ok {
		t.Error("a stale hand.lower lowered the hand or was refused")
	}
}

func TestTheTeacherLowersAHandForGood(t *testing.T) {
	b, c := newTestBoard()
	s := student("PC-07")
	id := protocol.NewID()
	b.Raise(s, raise(id))
	got, ok := b.LowerByTeacher(s)
	if !ok || got != id {
		t.Fatalf("LowerByTeacher = %q, %v", got, ok)
	}
	if _, ok := b.LowerByTeacher(s); ok {
		t.Error("lowered a hand that was down")
	}
	// The agent resends it after a reconnection without knowing: refused.
	c.add(time.Second)
	if out := b.Raise(s, raise(id)); out != LoweredBefore {
		t.Errorf("resent lowered hand = %v, want LoweredBefore", out)
	}
	// A new hand is welcome.
	c.add(time.Second)
	if out := b.Raise(s, raise(protocol.NewID())); out != Added {
		t.Errorf("new hand = %v", out)
	}
	if all := b.LowerAll(); len(all) != 1 || len(b.Inbox().Hands) != 0 {
		t.Errorf("LowerAll = %v", all)
	}
}

func TestMessagesAreDeduplicatedPacedAndCapped(t *testing.T) {
	b, c := newTestBoard()
	s := student("PC-07")
	first := protocol.NewID()
	if b.Message(s, msg(first, "hola")) != Added || b.Message(s, msg(first, "hola")) != Repeated {
		t.Fatal("a resent message was not deduplicated")
	}
	// Burst of MessageBurst, then one every MessageEvery.
	for i := 1; i < MessageBurst; i++ {
		if out := b.Message(s, msg(protocol.NewID(), "x")); out != Added {
			t.Fatalf("message %d of the burst = %v", i, out)
		}
	}
	if out := b.Message(s, msg(protocol.NewID(), "x")); out != TooFast {
		t.Fatalf("over the burst = %v, want TooFast", out)
	}
	c.add(MessageEvery)
	if out := b.Message(s, msg(protocol.NewID(), "x")); out != Added {
		t.Fatalf("after waiting = %v", out)
	}
	// Another student has their own pace.
	if out := b.Message(student("PC-08"), msg(protocol.NewID(), "x")); out != Added {
		t.Fatalf("another student = %v", out)
	}

	if !b.Replied(s, first, "sí") {
		t.Fatal("Replied did not find the message")
	}
	if m, _ := b.Lookup(s, first); m.Status != MessageReplied || m.Reply != "sí" {
		t.Errorf("after reply: %+v", m)
	}
	// Over MaxMessagesPerStudent: the answered one goes first.
	for range MaxMessagesPerStudent {
		c.add(MessageEvery)
		b.Message(s, msg(protocol.NewID(), "x"))
	}
	if _, ok := b.Lookup(s, first); ok {
		t.Error("the answered message was kept over the cap")
	}
	n := 0
	for _, m := range b.Inbox().Messages {
		if m.Student == s {
			n++
		}
	}
	if n != MaxMessagesPerStudent {
		t.Errorf("%d messages kept, want %d", n, MaxMessagesPerStudent)
	}
}

func TestInboxIsNewestFirstAndCountsPending(t *testing.T) {
	b, c := newTestBoard()
	old, recent := protocol.NewID(), protocol.NewID()
	b.Message(student("PC-1"), msg(old, "antes"))
	c.add(time.Minute)
	b.Message(student("PC-2"), msg(recent, "después"))
	b.Raise(student("PC-3"), raise(protocol.NewID()))
	b.Done(student("PC-1"), old)
	in := b.Inbox()
	if in.Messages[0].MessageID != recent || in.Pending != 2 {
		t.Errorf("first %s, pending %d; want the newest and 2", in.Messages[0].Text, in.Pending)
	}
}

func TestResetAndForgetSimulated(t *testing.T) {
	b, _ := newTestBoard()
	real, sim := student("PC-1"), Student{Hostname: "SIM-01", Simulated: true}
	b.Raise(real, raise(protocol.NewID()))
	b.Raise(sim, raise(protocol.NewID()))
	b.Message(sim, msg(protocol.NewID(), "x"))
	b.ForgetSimulated()
	in := b.Inbox()
	if len(in.Hands) != 1 || in.Hands[0].Student != real || len(in.Messages) != 0 {
		t.Errorf("after ForgetSimulated: %+v", in)
	}
	b.Reset()
	if in := b.Inbox(); len(in.Hands) != 0 || in.Pending != 0 {
		t.Errorf("after Reset: %+v", in)
	}
}

func newTestDesk() (*Desk, *clock) {
	c := newClock()
	d := NewDesk()
	d.now = c.now
	return d, c
}

func TestDeskHand(t *testing.T) {
	d, c := newTestDesk()
	if d.View().Hand != HandDown {
		t.Fatal("hand starts up")
	}
	p, err := d.Raise("  no compila  ")
	if err != nil || p.Note != "no compila" || p.Validate() != nil {
		t.Fatalf("Raise = %+v, %v", p, err)
	}
	if _, _, err := d.Lower(); !errors.Is(err, ErrTooSoon) {
		t.Fatalf("lower right away = %v, want ErrTooSoon", err)
	}
	c.add(AgentHandEvery)
	// Raising again keeps the hand_id (the console keeps its place).
	again, _ := d.Raise("otra")
	if again.HandID != p.HandID {
		t.Error("a raised hand changed its hand_id")
	}
	if d.LoweredByTeacher(protocol.HandLower{HandID: protocol.NewID(), By: protocol.HandByTeacher}) {
		t.Error("a stale hand.lower lowered the hand")
	}
	if !d.LoweredByTeacher(protocol.HandLower{HandID: p.HandID, By: protocol.HandByTeacher}) {
		t.Fatal("hand.lower from the teacher ignored")
	}
	if v := d.View(); v.Hand != HandLoweredByTeacher || v.Note != "" {
		t.Errorf("view after teacher = %+v", v)
	}
	c.add(AgentHandEvery)
	next, _ := d.Raise("")
	if next.HandID == p.HandID {
		t.Error("a new hand reused the lowered hand_id")
	}
	c.add(AgentHandEvery)
	low, ok, err := d.Lower()
	if !ok || err != nil || low.HandID != next.HandID || low.By != protocol.HandByStudent {
		t.Errorf("Lower = %+v, %v, %v", low, ok, err)
	}
	if _, err := d.Raise(strings.Repeat("a", protocol.MaxHandNote+1)); !errors.Is(err, ErrTooLong) {
		t.Errorf("long note = %v", err)
	}
}

func TestDeskMessagesAckAndResend(t *testing.T) {
	d, c := newTestDesk()
	if _, err := d.Write("   "); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty = %v", err)
	}
	if _, err := d.Write(strings.Repeat("a", protocol.MaxStudentMsgText+1)); !errors.Is(err, ErrTooLong) {
		t.Fatalf("long = %v", err)
	}
	first, err := d.Write("¿Es para hoy?")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write("otra"); !errors.Is(err, ErrTooSoon) {
		t.Fatalf("second right away = %v", err)
	}
	for range MaxAwaitingAck - 1 {
		c.add(AgentMessageEvery)
		if _, err := d.Write("x"); err != nil {
			t.Fatal(err)
		}
	}
	c.add(AgentMessageEvery)
	if _, err := d.Write("x"); !errors.Is(err, ErrWaiting) {
		t.Fatalf("over the awaiting limit = %v", err)
	}
	if !d.Acked(protocol.MsgStudentAck{RefID: protocol.NewID(), MessageID: first.MessageID}) {
		t.Fatal("ack ignored")
	}
	if s, _ := d.Own(first.MessageID); s.Status != SentReceived {
		t.Errorf("status after ack = %s", s.Status)
	}

	hand, _ := d.Raise("")
	resend := d.Reconnected()
	if len(resend) != MaxAwaitingAck { // the hand and the 4 messages without ack
		t.Fatalf("resend %d payloads, want %d", len(resend), MaxAwaitingAck)
	}
	if r, ok := resend[0].(protocol.HandRaise); !ok || r.HandID != hand.HandID {
		t.Errorf("first resend = %#v, want the same hand", resend[0])
	}
	for _, p := range resend[1:] {
		if m := p.(protocol.MsgStudent); m.MessageID == first.MessageID {
			t.Error("an acked message was resent")
		}
	}
}

func TestDeskRefusals(t *testing.T) {
	d, c := newTestDesk()
	m, _ := d.Write("hola")
	env := protocol.NewID()
	d.Sending(env, m)
	if d.Refused(protocol.ErrorPayload{RefID: protocol.NewID(), Code: protocol.CodeInvalidMessage}) {
		t.Fatal("an unrelated error was taken")
	}
	if !d.Refused(protocol.ErrorPayload{RefID: env, Code: protocol.CodeInvalidMessage}) {
		t.Fatal("the error to the message was not matched")
	}
	if s, _ := d.Own(m.MessageID); s.Status != SentFailed {
		t.Errorf("refused message status %s", s.Status)
	}

	// An older console: unknown_type lowers the hand and stops sending.
	c.add(AgentMessageEvery)
	h, _ := d.Raise("")
	env = protocol.NewID()
	d.Sending(env, h)
	d.Refused(protocol.ErrorPayload{RefID: env, Code: protocol.CodeUnknownType})
	v := d.View()
	if !v.Unsupported || v.Hand != HandDown {
		t.Errorf("after unknown_type: %+v", v)
	}
	c.add(AgentMessageEvery)
	if _, err := d.Write("x"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("write to an older console = %v", err)
	}
	d.Reconnected()
	if d.View().Unsupported {
		t.Error("a new connection did not clear the refusal")
	}
}
