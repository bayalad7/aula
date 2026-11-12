package main

import (
	"errors"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

var presentCaps = []protocol.Capability{protocol.CapScreenBroadcast, protocol.CapScreenPresentView, protocol.CapScreenPresent}

func presentHarness(t *testing.T) *harness {
	h, _ := broadcastHarness(t)
	h.class.mu.Lock()
	h.class.sessions = append(h.class.sessions,
		student("src", "LAB3-PC07", presentCaps...),
		student("v1", "LAB3-PC01", protocol.CapScreenBroadcast, protocol.CapScreenPresentView))
	h.class.mu.Unlock()
	h.app.proj.StartWait = time.Minute
	t.Cleanup(func() { h.app.StopPresent() })
	return h
}

func lastPresent(h *harness) PresentState {
	s, _ := h.rec.last(EventPresent).(PresentState)
	return s
}

// The source is asked first; once it allows, the other students with
// screen.present.view get the view with its name, never the source itself
// nor an agent that would call it the teacher's screen.
func TestPresentAsksTheSourceAndReachesTheOthers(t *testing.T) {
	h := presentHarness(t)
	if v := h.app.View(); v.Present.Active {
		t.Fatalf("present before %+v", v.Present)
	}
	if _, err := h.app.StartPresent("s1"); errorCode(err) != codePresentUnsupported {
		t.Fatalf("an agent without screen.present: %v", err)
	}
	if _, err := h.app.StartPresent("nadie"); errorCode(err) != codeStudentGone {
		t.Fatalf("unknown student: %v", err)
	}
	st, err := h.app.StartPresent("src")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Active || st.Hostname != "LAB3-PC07" || st.Phase != "connecting" {
		t.Fatalf("state %+v", st)
	}
	got := waitDirect(t, h, "src", 1)
	var start protocol.StudentPresentStart
	var startID string
	for _, d := range got {
		if p, ok := d.p.(protocol.StudentPresentStart); ok {
			start, startID = p, d.envID
		}
	}
	if startID == "" {
		t.Fatalf("src got %v", types(got, "src"))
	}
	if ts := types(got, "v1"); len(ts) != 0 {
		t.Fatalf("a classmate heard of it before the source showed its band: %v", ts)
	}

	h.hooks.onBroadcast("src", protocol.New(protocol.StudentPresentAck{RefID: startID, PresentID: start.PresentID,
		Status: protocol.PresentAccepted, Width: 1280, Height: 720}))
	got = waitDirect(t, h, "v1", 1)
	var bs protocol.ScreenBroadcastStart
	for _, d := range got {
		if p, ok := d.p.(protocol.ScreenBroadcastStart); ok && d.sessionID == "v1" {
			bs = p
		}
	}
	if bs.Source == nil || bs.Source.Hostname != "LAB3-PC07" {
		t.Fatalf("v1 got %+v", bs)
	}
	for _, id := range []string{"s1", "s2", "src"} {
		for _, d := range got {
			if _, ok := d.p.(protocol.ScreenBroadcastStart); ok && d.sessionID == id {
				t.Fatalf("%s got the view", id)
			}
		}
	}

	// Stop: the source and every receiver are told.
	if s := h.app.StopPresent(); s.Active {
		t.Fatalf("after stop %+v", s)
	}
	stops := map[string]protocol.Type{}
	for _, d := range h.class.takeDirect() {
		switch d.p.(type) {
		case protocol.StudentPresentStop, protocol.ScreenBroadcastStop:
			stops[d.sessionID] = d.p.MessageType()
		}
	}
	if stops["src"] != protocol.TypeStudentPresentStop || stops["v1"] != protocol.TypeScreenBroadcastStop {
		t.Fatalf("stops %v", stops)
	}
}

// One thing on the students' view at a time: starting the teacher's
// broadcast ends the presentation, and the other way round.
func TestPresentAndBroadcastReplaceEachOther(t *testing.T) {
	h := presentHarness(t)
	if _, err := h.app.StartBroadcast(0); err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.StartPresent("src"); err != nil {
		t.Fatal(err)
	}
	if h.app.BroadcastState().Active {
		t.Fatal("the teacher's broadcast goes on while presenting")
	}
	if _, err := h.app.StartBroadcast(0); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for h.app.PresentState().Active || lastPresent(h).Ended != "stopped" {
		if time.Now().After(deadline) {
			t.Fatalf("presentation still on: %+v / %+v", h.app.PresentState(), lastPresent(h))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A source that cannot capture ends it and the console says why; the class
// closing ends it too.
func TestPresentEndsWhenTheSourceFailsOrTheClassCloses(t *testing.T) {
	h := presentHarness(t)
	if _, err := h.app.StartPresent("src"); err != nil {
		t.Fatal(err)
	}
	var start protocol.StudentPresentStart
	var startID string
	for _, d := range waitDirect(t, h, "src", 1) {
		if p, ok := d.p.(protocol.StudentPresentStart); ok {
			start, startID = p, d.envID
		}
	}
	h.hooks.onBroadcast("src", protocol.New(protocol.StudentPresentAck{RefID: startID, PresentID: start.PresentID, Status: protocol.PresentFailed, Reason: protocol.PresentCaptureUnavailable}))
	deadline := time.Now().Add(3 * time.Second)
	for lastPresent(h).Ended != "failed" || lastPresent(h).Reason != "capture_unavailable" {
		if time.Now().After(deadline) {
			t.Fatalf("last %+v", lastPresent(h))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if s := lastPresent(h); s.Hostname != "LAB3-PC07" || s.Active {
		t.Fatalf("ended %+v", s)
	}

	if _, err := h.app.StartPresent("src"); err != nil {
		t.Fatal(err)
	}
	h.app.EndClass()
	if h.app.PresentState().Active {
		t.Fatal("presentation outlived the class")
	}
	if _, err := h.app.StartPresent("src"); !errors.Is(err, errNoClass) {
		t.Fatalf("StartPresent without class = %v", err)
	}
}
