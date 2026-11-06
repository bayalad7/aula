package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/classtimer"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

func timerHarness(t *testing.T) (*harness, *bytes.Buffer) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapMsgShow, protocol.CapTimer),
		student("s2", "PC02", protocol.CapMsgShow, protocol.CapTimer),
		student("s3", "PC03", protocol.CapMsgShow)) // an older agent
	logs := &bytes.Buffer{}
	h.app.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return h, logs
}

// «Iniciar» sends timer.start to who can show it; the acks reach the franja;
// «Extender» and «Detener» follow; the label never reaches the log.
func TestTimerStartExtendStop(t *testing.T) {
	h, logs := timerHarness(t)
	s, err := h.app.StartTimer(TimerRequest{Label: "Ejercicio secreto", DurationS: 300})
	if err != nil {
		t.Fatal(err)
	}
	if !s.Active || !s.All || s.Eligible != 2 || len(s.Unsupported) != 1 || s.Unsupported[0] != "PC03" || len(s.Receivers) != 2 {
		t.Fatalf("snapshot = %+v", s)
	}
	sent := h.class.takeDirect()
	if len(sent) != 2 {
		t.Fatalf("sent %+v", sent)
	}
	start, ok := sent[0].p.(protocol.TimerStart)
	if !ok || start.Label != "Ejercicio secreto" || start.DurationS != 300 || start.RemainingMS > 300_000 || start.RemainingMS < 299_000 {
		t.Fatalf("timer.start = %+v", sent[0].p)
	}
	h.hooks.onTimer("s1", protocol.TimerAck{RefID: sent[0].envID, TimerID: start.TimerID, Status: protocol.TimerRunning, RemainingMS: 299_000})
	ev := h.rec.last(EventTimer).(classtimer.Snapshot)
	if ev.Receivers[0].Status != classtimer.ReceiverRunning || ev.Receivers[1].Status != classtimer.ReceiverSent {
		t.Fatalf("receivers after an ack = %+v", ev.Receivers)
	}

	if _, err := h.app.ExtendTimer(300); err != nil {
		t.Fatal(err)
	}
	sent = h.class.takeDirect()
	if len(sent) != 2 {
		t.Fatalf("extend sent %+v", sent)
	}
	if ext, ok := sent[0].p.(protocol.TimerExtend); !ok || ext.AddS != 300 || ext.RemainingMS < 599_000 {
		t.Fatalf("timer.extend = %+v", sent[0].p)
	}
	if _, err := h.app.ExtendTimer(5 * 3600); err == nil || errorCode(err) != codeInvalidTimer {
		t.Errorf("extension past the limit = %v", err)
	}

	s, err = h.app.StopTimer()
	if err != nil || s.Active {
		t.Fatalf("StopTimer = %+v, %v", s, err)
	}
	sent = h.class.takeDirect()
	if len(sent) != 2 {
		t.Fatalf("stop sent %+v", sent)
	}
	if st, ok := sent[1].p.(protocol.TimerStop); !ok || st.TimerID != start.TimerID {
		t.Fatalf("timer.stop = %+v", sent[1].p)
	}
	if _, err := h.app.StopTimer(); errorCode(err) != codeTimerGone {
		t.Errorf("second stop = %v", err)
	}
	if strings.Contains(logs.String(), "secreto") {
		t.Errorf("the label is in the log:\n%s", logs.String())
	}
}

// A student who reconnects gets the timer again with what is left; a
// selection only reaches the marked students; a selection that cannot show
// it says so.
func TestTimerReconnectionAndSelection(t *testing.T) {
	h, _ := timerHarness(t)
	if _, err := h.app.StartTimer(TimerRequest{DurationS: 120, Hosts: []string{"pc03"}}); errorCode(err) != codeNoTimerTargets {
		t.Fatalf("a selection without timer = %v", err)
	}
	if _, err := h.app.StartTimer(TimerRequest{DurationS: 5}); errorCode(err) != codeInvalidTimer {
		t.Fatalf("5 s = %v", err)
	}
	if _, err := h.app.StartTimer(TimerRequest{Label: "Ejercicio 1", DurationS: 120, Hosts: []string{"PC02"}}); err != nil {
		t.Fatal(err)
	}
	if sent := h.class.takeDirect(); len(sent) != 1 || sent[0].sessionID != "s2" {
		t.Fatalf("selection sent %+v", sent)
	}
	// PC02 reconnects with a new session.
	h.class.mu.Lock()
	h.class.sessions = []session.Session{h.class.sessions[0], student("s9", "PC02", protocol.CapTimer), h.class.sessions[2]}
	h.class.mu.Unlock()
	h.hooks.onChange()
	sent := h.class.takeDirect()
	if len(sent) != 1 || sent[0].sessionID != "s9" {
		t.Fatalf("reconnect sent %+v", sent)
	}
	if p, ok := sent[0].p.(protocol.TimerStart); !ok || p.Label != "Ejercicio 1" || p.RemainingMS > 120_000 {
		t.Fatalf("resend = %+v", sent[0].p)
	}
	h.hooks.onChange()
	if sent := h.class.takeDirect(); len(sent) != 0 {
		t.Fatalf("sent twice: %+v", sent)
	}
	if v := h.app.View(); !v.Timer.Active || v.Timer.All || v.Students.CanTimer != 2 {
		t.Fatalf("view = %+v, canTimer %d", v.Timer, v.Students.CanTimer)
	}
	// Closing the class forgets it.
	h.app.EndClass()
	if h.app.Timer().Active {
		t.Error("the timer outlived the class")
	}
}
