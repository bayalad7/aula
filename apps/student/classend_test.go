package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/classend"
	"github.com/bayalad7/aula/internal/classtimer"
	"github.com/bayalad7/aula/internal/participation"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// The teacher ends the class: the window is left as freshly opened (code,
// notices, timer, poll, hand), the card says when, it notifies without
// raising, and a second class.end is clean too (idempotent). The code never
// reaches the log (docs/privacidad.md rule 86).
func TestClassEndClearsTheWindow(t *testing.T) {
	got := &heard{}
	c := startLiveConsoleWith(t, got.onMessage)
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: c.addr}
	pub, _ := cfg.Validate()
	a, rec := newTestApp(nil)
	logs := &bytes.Buffer{}
	a.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a.cfg, a.pub = cfg, pub
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx)
	defer a.stop()
	s := c.waitAgent(t, "agent", func(s session.Session) bool { return s.Can(protocol.CapClassEnd) })
	waitConnected(t, a)

	send := func(p protocol.Payload) string {
		t.Helper()
		id, err := c.console.Send(ctx, s.ID, p)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	send(protocol.CodeShare{SnippetID: protocol.NewID(), Title: "t", Lang: "python", Content: "print('secreto')", Mode: protocol.ModeReplace})
	send(protocol.MsgShow{Text: "aviso", Level: protocol.LevelInfo})
	timerID := send(protocol.TimerStart{TimerID: protocol.NewID(), DurationS: 600, RemainingMS: 600_000})
	send(protocol.PollOpen{PollID: protocol.NewID(), Kind: protocol.PollKindYesNo, Question: "¿Listo?"})
	if _, err := a.RaiseHand(""); err != nil {
		t.Fatal(err)
	}
	got.wait(t, "the timer running", func(e protocol.Envelope) bool {
		p, ok := e.Payload.(protocol.TimerAck)
		return ok && p.RefID == timerID
	})
	waitFor(t, "everything on the window", func() bool {
		v := a.View()
		return len(v.Snippets) == 1 && len(v.Notices) == 1 && len(v.Polls.Cards) == 1
	})

	end := protocol.ClassEnd{EndID: protocol.NewID()}
	rec.mu.Lock()
	raisedBefore, notifiedBefore := rec.raised, rec.notified
	rec.mu.Unlock()
	for i := range 2 {
		id := send(end)
		ack := got.wait(t, "class.end.ack", func(e protocol.Envelope) bool {
			p, ok := e.Payload.(protocol.ClassEndAck)
			return ok && p.RefID == id
		}).Payload.(protocol.ClassEndAck)
		if ack.Status != protocol.ClassEndClean || ack.EndID != end.EndID {
			t.Fatalf("class.end #%d: ack = %+v", i+1, ack)
		}
	}
	v := a.View()
	if len(v.Snippets) != 0 || len(v.Notices) != 0 || len(v.Polls.Cards) != 0 || v.Timer.State != classtimer.StateNone ||
		v.Participation.Hand != participation.HandDown {
		t.Fatalf("the window is not empty: %+v", v)
	}
	if v.ClassEnd == nil || v.ClassEnd.EndID != end.EndID {
		t.Fatalf("View().ClassEnd = %+v", v.ClassEnd)
	}
	card, ok := rec.last(EventClassEnd).(*classend.Card)
	if !ok || card == nil {
		t.Fatalf("class-end event = %#v", rec.last(EventClassEnd))
	}
	rec.mu.Lock()
	raised, notified := rec.raised-raisedBefore, rec.notified-notifiedBefore
	rec.mu.Unlock()
	if raised != 0 || notified < 1 {
		t.Errorf("raised %d, notified %d: ending the class notifies and never raises", raised, notified)
	}
	if strings.Contains(logs.String(), "secreto") {
		t.Errorf("the code is in the log:\n%s", logs.String())
	}

	// New code means a new class: the card goes.
	send(protocol.CodeShare{SnippetID: protocol.NewID(), Title: "t2", Lang: "go", Content: "x", Mode: protocol.ModeReplace})
	waitFor(t, "the card gone", func() bool { return a.View().ClassEnd == nil })
	a.DismissClassEnd()
	if card, _ := rec.last(EventClassEnd).(*classend.Card); card != nil {
		t.Fatal("DismissClassEnd must send a null card")
	}
}

// What the first class.end removes is counted on the card; restrictions that
// cannot be removed make the ack failed, and the rest is still cleared.
func TestClassEndRestrictionsFail(t *testing.T) {
	a, rec := newTestApp(nil)
	a.endRestrictions = func() error { return errors.New("dns busy") }
	a.board.Apply(protocol.CodeShare{SnippetID: protocol.NewID(), Lang: "go", Content: "x", Mode: protocol.ModeReplace}, time.Now())
	p := protocol.ClassEnd{EndID: protocol.NewID()}
	ack := a.endClass(protocol.NewID(), p)
	if ack.Status != protocol.ClassEndFailed || ack.Reason != protocol.ClassEndRestrictions || ack.Validate() != nil {
		t.Fatalf("ack = %+v", ack)
	}
	card, _ := rec.last(EventClassEnd).(*classend.Card)
	if card == nil || card.Removed.Snippets != 1 || len(a.board.Snippets()) != 0 {
		t.Fatalf("card = %+v", card)
	}
}
