package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/classtimer"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// The teacher starts a timer, the agent counts and acks, ends once at zero
// (notify, never raise), the teacher extends it and stops it. The label never
// reaches the log (docs/privacidad.md rule 54).
func TestTimerWithLiveConsole(t *testing.T) {
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
	s := c.waitAgent(t, "agent", func(s session.Session) bool { return s.Can(protocol.CapTimer) })
	waitConnected(t, a)

	ackOf := func(what string, refID string, st protocol.TimerStatus) protocol.TimerAck {
		t.Helper()
		return got.wait(t, what, func(e protocol.Envelope) bool {
			p, ok := e.Payload.(protocol.TimerAck)
			return ok && p.RefID == refID && p.Status == st
		}).Payload.(protocol.TimerAck)
	}

	timerID := protocol.NewID()
	startID, err := c.console.Send(ctx, s.ID, protocol.TimerStart{TimerID: timerID, Label: "Ejercicio secreto", DurationS: 10, RemainingMS: 400})
	if err != nil {
		t.Fatal(err)
	}
	if ack := ackOf("running", startID, protocol.TimerRunning); ack.TimerID != timerID || ack.RemainingMS > 400 {
		t.Fatalf("running ack = %+v", ack)
	}
	if v, ok := rec.last(EventTimer).(classtimer.View); !ok || v.Label != "Ejercicio secreto" {
		t.Fatalf("timer event = %#v", rec.last(EventTimer))
	}
	ackOf("ended", startID, protocol.TimerEnded)
	waitFor(t, "the ended view", func() bool {
		v, ok := rec.last(EventTimer).(classtimer.View)
		return ok && v.State == classtimer.StateEnded && v.JustEnded
	})
	if a.View().Timer.State != classtimer.StateEnded {
		t.Fatalf("View().Timer = %+v", a.View().Timer)
	}

	// Extending revives it with what the console says is left.
	extID, err := c.console.Send(ctx, s.ID, protocol.TimerExtend{TimerID: timerID, AddS: 60, RemainingMS: 60_000})
	if err != nil {
		t.Fatal(err)
	}
	ackOf("running after extending", extID, protocol.TimerRunning)
	if v := a.View().Timer; v.State != classtimer.StateRunning || v.RemainingMS < 59_000 {
		t.Fatalf("after extending = %+v", v)
	}

	stopID, err := c.console.Send(ctx, s.ID, protocol.TimerStop{TimerID: timerID})
	if err != nil {
		t.Fatal(err)
	}
	ackOf("stopped", stopID, protocol.TimerStopped)
	if v := a.View().Timer; v.State != classtimer.StateNone {
		t.Fatalf("after stopping = %+v", v)
	}

	rec.mu.Lock()
	notified, raised := rec.notified, rec.raised
	rec.mu.Unlock()
	if raised != 0 || notified < 1 {
		t.Errorf("notified %d, raised %d: the end notifies and nothing raises", notified, raised)
	}
	if strings.Contains(logs.String(), "secreto") {
		t.Errorf("the label is in the log:\n%s", logs.String())
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
