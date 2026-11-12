package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/classend"
	"github.com/bayalad7/aula/internal/poll"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

var endCaps = []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow, protocol.CapScreenFreeze, protocol.CapAppBlock,
	protocol.CapTimer, protocol.CapPoll, protocol.CapPower, protocol.CapClassEnd}

// waitEnded waits for the steps of «Terminar la clase» to finish.
func waitEnded(t *testing.T, a *App) classend.View {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		v := a.ClassEnd()
		if !v.Running {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatal("the end of class never finished")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func sentTo(ds []directSend, t protocol.Type) []string {
	var out []string
	for _, d := range ds {
		if d.p.MessageType() == t {
			out = append(out, d.sessionID)
		}
	}
	slices.Sort(out)
	return out
}

// With everything on (pause, restrictions, timer, poll, a pending power
// order and code), «Terminar la clase» stops it all with the usual stops,
// keeps the power order, sends class.end, and leaves the console «sin
// clase». An earlier agent gets code.clear; a disconnected one gets class.end
// when it reconnects, before anything else; a second end is harmless.
func TestFinishClassStopsEverythingAndCleansWindows(t *testing.T) {
	away := student("s4", "AWAY04", endCaps...)
	away.State = session.StateDisconnected
	h := newHarness(t, student("s1", "PC01", endCaps...), student("s2", "PC02", endCaps...),
		student("s3", "OLD03", protocol.CapCodeView, protocol.CapMsgShow), away)
	a := h.app
	if _, err := a.Emit(EmitRequest{Title: "t", Lang: "python", Content: pySnippet, Mode: "replace"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PauseScreens(PauseRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetRestriction(RestrictionRequest{Kind: "apps", Mode: "deny", Items: []string{"steam"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartTimer(TimerRequest{Label: "Ej 1", DurationS: 600}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenPoll(poll.Request{Kind: protocol.PollKindYesNo, Question: "¿Listo?"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PowerAction(PowerRequest{Action: "shutdown", DelayS: 600}); err != nil {
		t.Fatal(err)
	}
	h.class.takeDirect()

	p, err := a.ClassEndPreview()
	if err != nil {
		t.Fatal(err)
	}
	if p.Paused != 2 || !p.Apps || !p.Timer || !p.Poll || p.Power != 2 || p.Students != 3 || p.Old != 1 ||
		p.Disconnected != 1 || p.Snippets != 1 {
		t.Fatalf("preview = %+v", p)
	}

	if _, err := a.FinishClass(ClassEndRequest{}); err != nil {
		t.Fatal(err)
	}
	v := waitEnded(t, a)
	ds := h.class.takeDirect()
	for typ, want := range map[protocol.Type][]string{
		protocol.TypeScreenUnfreeze: {"s1", "s2"},
		protocol.TypeAppBlockClear:  {"s1", "s2"},
		protocol.TypeTimerStop:      {"s1", "s2"},
		protocol.TypePollClose:      {"s1", "s2"},
		protocol.TypeClassEnd:       {"s1", "s2"},
		protocol.TypeCodeClear:      {"s3"},
		protocol.TypePowerCancel:    nil, // kept: the teacher did not tick it
	} {
		if got := sentTo(ds, typ); !slices.Equal(got, want) {
			t.Errorf("%s sent to %v, want %v", typ, got, want)
		}
	}
	// class.end goes after the stops.
	lastStop, firstEnd := -1, len(ds)
	for i, d := range ds {
		switch d.p.MessageType() {
		case protocol.TypeClassEnd:
			firstEnd = min(firstEnd, i)
		case protocol.TypeScreenUnfreeze, protocol.TypeAppBlockClear, protocol.TypeTimerStop, protocol.TypePollClose:
			lastStop = i
		}
	}
	if lastStop > firstEnd {
		t.Error("class.end was sent before a stop")
	}
	kinds := map[classend.StepKind]classend.StepStatus{}
	for _, s := range v.Steps {
		kinds[s.Kind] = s.Status
	}
	for k, want := range map[classend.StepKind]classend.StepStatus{classend.StepPause: classend.StepDone,
		classend.StepRestrictions: classend.StepDone, classend.StepTimer: classend.StepDone, classend.StepPoll: classend.StepDone,
		classend.StepPower: classend.StepKept} {
		if kinds[k] != want {
			t.Errorf("step %s = %q, want %q (steps %+v)", k, kinds[k], want, v.Steps)
		}
	}

	// PC01 clean, PC02 failed; OLD03 partial, AWAY04 away.
	end := v.EndID
	for _, d := range ds {
		if d.p.MessageType() != protocol.TypeClassEnd {
			continue
		}
		ack := protocol.ClassEndAck{RefID: d.envID, EndID: end, Status: protocol.ClassEndClean}
		if d.sessionID == "s2" {
			ack.Status, ack.Reason = protocol.ClassEndFailed, protocol.ClassEndRestrictions
		}
		h.hooks.onClassEnd(d.sessionID, ack)
	}
	states := map[string]classend.State{}
	for _, s := range a.ClassEnd().Students {
		states[s.Hostname] = s.State
	}
	want := map[string]classend.State{"PC01": classend.StateClean, "PC02": classend.StateFailed,
		"OLD03": classend.StatePartial, "AWAY04": classend.StateAway}
	for host, st := range want {
		if states[host] != st {
			t.Errorf("%s = %s, want %s", host, states[host], st)
		}
	}
	if ev, ok := h.rec.last(EventClassEnd).(classend.View); !ok || ev.Clean != 2 || ev.Missing != 2 {
		t.Errorf("last class-end event = %+v", h.rec.last(EventClassEnd))
	}

	// The console is «sin clase».
	vw := a.View()
	if len(vw.History) != 0 || len(vw.Deliveries) != 0 || a.Timer().Active || a.Polls().Current != nil ||
		len(a.Polls().Closed) != 0 || a.Pauses().Paused != 0 || a.Restrictions().Apps != nil {
		t.Fatalf("the console still has the class: %+v", vw)
	}

	// Retry: only PC02 (failed); AWAY04 waits for its reconnection.
	if _, err := a.RetryClassEnd(); err != nil {
		t.Fatal(err)
	}
	if got := sentTo(h.class.takeDirect(), protocol.TypeClassEnd); !slices.Equal(got, []string{"s2"}) {
		t.Fatalf("retry sent to %v", got)
	}

	// AWAY04 reconnects as s5: class.end comes first.
	h.class.mu.Lock()
	back := student("s5", "AWAY04", endCaps...)
	h.class.sessions = append(h.class.sessions[:3], back)
	h.class.mu.Unlock()
	h.app.onChange()
	ds = h.class.takeDirect()
	if len(ds) == 0 || ds[0].sessionID != "s5" || ds[0].p.MessageType() != protocol.TypeClassEnd {
		t.Fatalf("on reconnecting: %v", sentTypes(ds))
	}
	h.hooks.onClassEnd("s5", protocol.ClassEndAck{RefID: ds[0].envID, EndID: end, Status: protocol.ClassEndClean})
	h.app.onChange()
	if got := sentTo(h.class.takeDirect(), protocol.TypeClassEnd); len(got) != 0 {
		t.Fatalf("an acknowledged computer got class.end again: %v", got)
	}

	// A second end: nothing to stop, every window again.
	if _, err := a.FinishClass(ClassEndRequest{}); err != nil {
		t.Fatal(err)
	}
	v = waitEnded(t, a)
	ds = h.class.takeDirect()
	if got := sentTo(ds, protocol.TypeClassEnd); !slices.Equal(got, []string{"s1", "s2", "s5"}) {
		t.Fatalf("second end sent class.end to %v", got)
	}
	if len(sentTo(ds, protocol.TypeScreenUnfreeze))+len(sentTo(ds, protocol.TypeTimerStop)) != 0 || len(v.Steps) != 1 {
		t.Fatalf("second end: %v, steps %+v", sentTypes(ds), v.Steps) // only the kept power order
	}
}

// Ticking «Cancelar el apagado programado» sends power.cancel; without a
// class, the bindings answer noClass.
func TestFinishClassCancelsPowerOnlyWhenAsked(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", endCaps...))
	if _, err := h.app.PowerAction(PowerRequest{Action: "reboot", DelayS: 600}); err != nil {
		t.Fatal(err)
	}
	h.class.takeDirect()
	if _, err := h.app.FinishClass(ClassEndRequest{CancelPower: true}); err != nil {
		t.Fatal(err)
	}
	v := waitEnded(t, h.app)
	if got := sentTo(h.class.takeDirect(), protocol.TypePowerCancel); !slices.Equal(got, []string{"s1"}) {
		t.Fatalf("power.cancel sent to %v", got)
	}
	if len(v.Steps) != 1 || v.Steps[0].Kind != classend.StepPower || v.Steps[0].Status != classend.StepDone {
		t.Fatalf("steps %+v", v.Steps)
	}

	h.app.EndClass()
	if _, err := h.app.FinishClass(ClassEndRequest{}); errorCode(err) != codeNoClass {
		t.Fatalf("without a class: %v", err)
	}
	if _, err := h.app.ClassEndPreview(); errorCode(err) != codeNoClass {
		t.Fatalf("preview without a class: %v", err)
	}
	if v := h.app.ClassEnd(); v.EndID != "" {
		t.Fatalf("closing the class must forget the end: %+v", v)
	}
	if !strings.HasPrefix(errorHelp[codeEndBusy], "tareas-frecuentes#") {
		t.Fatal("endBusy must link to its task")
	}
}
