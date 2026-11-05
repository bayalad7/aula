package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/participation"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

// heard collects what the console receives from the agent.
type heard struct {
	mu   sync.Mutex
	envs []protocol.Envelope
	conn *transport.Conn
}

func (h *heard) onMessage(_ context.Context, c *transport.Conn, env protocol.Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.envs = append(h.envs, env)
	h.conn = c
}

func (h *heard) wait(t *testing.T, what string, cond func(protocol.Envelope) bool) protocol.Envelope {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		for _, e := range h.envs {
			if cond(e) {
				h.mu.Unlock()
				return e
			}
		}
		h.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
	return protocol.Envelope{}
}

func waitView(t *testing.T, a *App, what string, cond func(participation.DeskView) bool) participation.DeskView {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if v := a.desk.View(); cond(v) {
			return v
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s: %+v", what, a.desk.View())
	return participation.DeskView{}
}

// The student raises the hand and writes; the console gets both, acks the
// message, answers it with reply_to and lowers the hand. Nothing takes the
// focus and the text never reaches the log.
func TestHandAndMessageWithLiveConsole(t *testing.T) {
	got := &heard{}
	c := startLiveConsoleWith(t, got.onMessage)
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: c.addr}
	pub, _ := cfg.Validate()
	a, rec := newTestApp(nil)
	logs := &bytes.Buffer{}
	a.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a.cfg, a.pub = cfg, pub

	if _, err := a.RaiseHand(""); err == nil || err.Error() != "aula:"+partNoConnection {
		t.Fatalf("RaiseHand before connecting = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx)
	defer a.stop()
	s := c.waitAgent(t, "agent", func(s session.Session) bool { return s.Can(protocol.CapHand) && s.Can(protocol.CapMsgStudent) })
	waitConnected(t, a)

	v, err := a.RaiseHand("no compila el for")
	if err != nil || v.Hand != participation.HandUp {
		t.Fatalf("RaiseHand = %+v, %v", v, err)
	}
	raised := got.wait(t, "hand.raise", func(e protocol.Envelope) bool { return e.Type == protocol.TypeHandRaise }).Payload.(protocol.HandRaise)
	if raised.Note != "no compila el for" {
		t.Errorf("note %q", raised.Note)
	}

	if _, err := a.SendTeacherMessage("¿El ejercicio 3 es para hoy?"); err != nil {
		t.Fatal(err)
	}
	env := got.wait(t, "msg.student", func(e protocol.Envelope) bool { return e.Type == protocol.TypeMsgStudent })
	m := env.Payload.(protocol.MsgStudent)
	if _, err := c.console.Send(ctx, s.ID, protocol.MsgStudentAck{RefID: env.ID, MessageID: m.MessageID}); err != nil {
		t.Fatal(err)
	}
	waitView(t, a, "received", func(v participation.DeskView) bool {
		return len(v.Messages) == 1 && v.Messages[0].Status == participation.SentReceived
	})
	if _, err := a.SendTeacherMessage("otro"); err == nil || err.Error() != "aula:"+partTooSoon {
		t.Errorf("second message right away = %v", err)
	}

	// The teacher's answer: a notice that quotes the student's message.
	if _, err := c.console.Send(ctx, s.ID, protocol.MsgShow{Text: "Sí.", Level: protocol.LevelInfo, ReplyTo: m.MessageID}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.console.Send(ctx, s.ID, protocol.HandLower{HandID: raised.HandID, By: protocol.HandByTeacher}); err != nil {
		t.Fatal(err)
	}
	waitView(t, a, "lowered by the teacher", func(v participation.DeskView) bool { return v.Hand == participation.HandLoweredByTeacher })
	deadline := time.Now().Add(5 * time.Second)
	for len(a.View().Notices) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if n := a.View().Notices; len(n) != 1 || n[0].ReplyTo != m.MessageID {
		t.Fatalf("notices %+v", n)
	}
	if rec.raised != 0 {
		t.Errorf("the window was raised %d times; hands and messages never take the focus", rec.raised)
	}
	if out := logs.String(); strings.Contains(out, "ejercicio 3") || strings.Contains(out, "compila") {
		t.Errorf("text in the log:\n%s", out)
	}
}

func waitConnected(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for a.connection() == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if a.connection() == nil {
		t.Fatal("not connected")
	}
}

// After a reconnection the raised hand and the message without ack go again,
// with the same IDs.
func TestHandAndMessageResentAfterReconnecting(t *testing.T) {
	a, _ := newTestApp(nil)
	a.desk = participation.NewDesk()
	h, err := a.desk.Raise("")
	if err != nil {
		t.Fatal(err)
	}
	m, err := a.desk.Write("hola")
	if err != nil {
		t.Fatal(err)
	}
	got := &heard{}
	c := startLiveConsoleWith(t, got.onMessage)
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: c.addr}
	a.cfg = cfg
	a.pub, _ = cfg.Validate()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx)
	defer a.stop()
	got.wait(t, "the same hand", func(e protocol.Envelope) bool {
		p, ok := e.Payload.(protocol.HandRaise)
		return ok && p.HandID == h.HandID
	})
	got.wait(t, "the same message", func(e protocol.Envelope) bool {
		p, ok := e.Payload.(protocol.MsgStudent)
		return ok && p.MessageID == m.MessageID
	})
}
