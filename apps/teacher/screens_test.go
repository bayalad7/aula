package main

import (
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/screen"
)

// The screens view starts thumbnails only while it is open, switches one
// student to detail and stops everything when it closes.
func TestScreensViewStartsAndStopsStreams(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView, protocol.CapScreenThumb),
		student("s2", "PC02", protocol.CapCodeView, protocol.CapScreenThumb),
		student("s3", "PC03", protocol.CapCodeView)) // no screen.thumb (Linux, older agent)

	if got := h.class.takeDirect(); len(got) != 0 {
		t.Fatalf("sent %v before opening the view", got)
	}
	if st := h.app.View().Students; st.CanThumb != 2 {
		t.Fatalf("canThumb = %d, want 2", st.CanThumb)
	}
	if err := h.app.OpenScreens(); err != nil {
		t.Fatal(err)
	}
	starts := map[string]string{}
	for _, d := range h.class.takeDirect() {
		if d.p != screen.ThumbParams {
			t.Fatalf("sent %#v to %s, want thumbnail start", d.p, d.sessionID)
		}
		starts[d.sessionID] = d.envID
	}
	if len(starts) != 2 || starts["s3"] != "" {
		t.Fatalf("starts = %v, want s1 and s2", starts)
	}

	// A frame of the stream in force reaches the frontend; a stale one does not.
	h.hooks.onFrame("s1", protocol.ScreenThumbFrame{RefID: starts["s1"], Seq: 1, Width: 320, Height: 180, JPEG: []byte{1}})
	th, ok := h.rec.last(EventFrame).(screen.Thumb)
	if !ok || th.SessionID != "s1" || th.Seq != 1 {
		t.Fatalf("frame event = %#v", h.rec.last(EventFrame))
	}
	h.hooks.onFrame("s2", protocol.ScreenThumbFrame{RefID: protocol.NewID(), Seq: 1, Width: 1, Height: 1, JPEG: []byte{1}})
	if th := h.rec.last(EventFrame).(screen.Thumb); th.SessionID != "s1" {
		t.Fatal("stale frame forwarded")
	}

	h.app.ViewScreen("s2")
	if d := h.class.takeDirect(); len(d) != 1 || d[0].sessionID != "s2" || d[0].p != screen.DetailParams {
		t.Fatalf("detail sent %v", d)
	}
	h.app.ViewScreen("")
	if d := h.class.takeDirect(); len(d) != 1 || d[0].p != screen.ThumbParams {
		t.Fatalf("back to grid sent %v", d)
	}

	// A student who joins while the view is open starts too.
	h.class.mu.Lock()
	h.class.sessions = append(h.class.sessions, student("s4", "PC04", protocol.CapScreenThumb))
	h.class.mu.Unlock()
	h.hooks.onChange()
	if d := h.class.takeDirect(); len(d) != 1 || d[0].sessionID != "s4" {
		t.Fatalf("new student got %v", d)
	}

	h.app.CloseScreens()
	stops := map[string]bool{}
	for _, d := range h.class.takeDirect() {
		if _, ok := d.p.(protocol.ScreenThumbStop); !ok {
			t.Fatalf("sent %#v on close", d.p)
		}
		stops[d.sessionID] = true
	}
	if len(stops) != 3 || !stops["s1"] || !stops["s2"] || !stops["s4"] {
		t.Fatalf("stops = %v, want s1, s2 and s4", stops)
	}
	h.hooks.onChange()
	if d := h.class.takeDirect(); len(d) != 0 {
		t.Fatalf("sent %v after closing the view", d)
	}
}

func TestEndClassStopsStreams(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapScreenThumb))
	if err := h.app.OpenScreens(); err != nil {
		t.Fatal(err)
	}
	h.class.takeDirect()
	h.app.EndClass()
	d := h.class.takeDirect()
	if len(d) != 1 {
		t.Fatalf("sent %v on EndClass, want one stop", d)
	}
	if _, ok := d[0].p.(protocol.ScreenThumbStop); !ok {
		t.Fatalf("sent %#v", d[0].p)
	}
	if err := h.app.OpenScreens(); err != errNoClass {
		t.Fatalf("OpenScreens without class = %v", err)
	}
}
