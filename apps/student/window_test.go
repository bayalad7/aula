package main

import (
	"slices"
	"sync"
	"testing"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// fakeShell records what the window code asks of Wails.
type fakeShell struct {
	mu        sync.Mutex
	calls     []string
	maximised bool
	visible   bool
	onTopNow  bool
}

func (s *fakeShell) log(c string) { s.mu.Lock(); s.calls = append(s.calls, c); s.mu.Unlock() }
func (s *fakeShell) raise(func() bool) {
	s.log("raise")
	s.mu.Lock()
	s.visible = true
	s.mu.Unlock()
}
func (s *fakeShell) isMaximised() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.maximised }
func (s *fakeShell) maximise(on bool) {
	s.log(map[bool]string{true: "maximise", false: "unmaximise"}[on])
	s.mu.Lock()
	s.maximised = on
	s.mu.Unlock()
}
func (s *fakeShell) fullscreen(on bool) {
	s.log(map[bool]string{true: "fullscreen", false: "unfullscreen"}[on])
}
func (s *fakeShell) onTop(on bool)    { s.log("ontop"); s.mu.Lock(); s.onTopNow = on; s.mu.Unlock() }
func (s *fakeShell) hide()            { s.log("hide"); s.mu.Lock(); s.visible = false; s.mu.Unlock() }
func (s *fakeShell) minimise()        { s.log("minimise") }
func (s *fakeShell) setSize(int, int) { s.log("size") }
func (s *fakeShell) setMinSize(int, int) {
}

func (s *fakeShell) did(c string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Contains(s.calls, c)
}

// fakeNative is platform.AppWindow: it records the holds and whether the
// window is back to its placement.
type fakeNative struct {
	mu      sync.Mutex
	holds   []platform.HoldMode
	held    bool
	compact bool
}

func (n *fakeNative) Flash() error { return nil }
func (n *fakeNative) Compact(on bool, _, _ int) error {
	n.mu.Lock()
	n.compact = on
	n.mu.Unlock()
	return nil
}
func (n *fakeNative) Hold(m platform.HoldMode) error {
	n.mu.Lock()
	n.holds = append(n.holds, m)
	n.held = true
	n.mu.Unlock()
	return nil
}
func (n *fakeNative) Release() error { n.mu.Lock(); n.held = false; n.mu.Unlock(); return nil }

type fakeTray struct{ marked bool }

func (t *fakeTray) SetTooltip(string) error { return nil }
func (t *fakeTray) SetMarked(m bool) error  { t.marked = m; return nil }
func (t *fakeTray) Close() error            { return nil }
func (n *fakeNative) isHeld() bool          { n.mu.Lock(); defer n.mu.Unlock(); return n.held }
func (n *fakeNative) lastHold() platform.HoldMode {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.holds[len(n.holds)-1]
}

func testWindow(t *testing.T) (*App, *appWindow, *fakeShell, *fakeNative) {
	t.Helper()
	a, _ := newTestApp(nil)
	sh, nat := &fakeShell{visible: true}, &fakeNative{}
	w := &appWindow{app: a, log: quiet, sh: sh, win: nat, tray: &fakeTray{}}
	a.raise, a.hold, a.compactHook = w.raise, w.hold, w.setCompact
	return a, w, sh, nat
}

// TestHoldModes: in both modes the window comes forward and is held by the
// native control; closing it does nothing (the exception to rule 2) and
// the compact strip is refused. v0.4's view ("") is a plain maximize that
// the student may still close.
func TestHoldModes(t *testing.T) {
	for _, tt := range []struct {
		mode protocol.BroadcastMode
		want platform.HoldMode
	}{{protocol.BroadcastWindow, platform.HoldWindow}, {protocol.BroadcastFullscreen, platform.HoldFullscreen}} {
		_, w, sh, nat := testWindow(t)
		w.hold(true, tt.mode)
		if !sh.did("raise") || !nat.isHeld() || nat.lastHold() != tt.want {
			t.Fatalf("%s: raise %v, holds %v", tt.mode, sh.did("raise"), nat.holds)
		}
		if !w.beforeClose() || sh.did("hide") || sh.did("minimise") {
			t.Fatalf("%s: closing hid the window: %v", tt.mode, sh.calls)
		}
		if _ = w.setCompact(true); w.compact {
			t.Fatalf("%s: compact strip while held", tt.mode)
		}
		w.hold(false, "")
		if nat.isHeld() {
			t.Fatalf("%s: not released", tt.mode)
		}
		if !w.beforeClose() || !sh.did("hide") {
			t.Fatalf("%s: after the broadcast closing must hide the window again (rule 2)", tt.mode)
		}
	}

	_, w, sh, nat := testWindow(t)
	w.hold(true, "")
	if !sh.did("maximise") || nat.isHeld() {
		t.Fatalf("v0.4 view: %v, native held %v", sh.calls, nat.isHeld())
	}
	if !w.beforeClose() || !sh.did("hide") {
		t.Fatal("v0.4 view: the student may close it to the notification area")
	}
}

// TestHoldReleaseRestoresTheWindow: when the broadcast ends the window goes
// back exactly as it was: compact, closed to the notification area or
// maximized (docs/privacidad.md rule 12).
func TestHoldReleaseRestoresTheWindow(t *testing.T) {
	// Compact before: back to the strip, and the page is told.
	a, w, _, nat := testWindow(t)
	rec := &recorder{}
	a.emit = rec.emit
	_ = w.setCompact(true)
	w.hold(true, protocol.BroadcastFullscreen)
	if w.compact || nat.compact {
		t.Fatal("still compact during the broadcast")
	}
	w.hold(false, "")
	if !w.compact || !nat.compact {
		t.Fatal("not back to the compact strip")
	}
	if c, ok := rec.last(EventCompact).(Compact); !ok || !c.On {
		t.Fatalf("page not told: %+v", rec.last(EventCompact))
	}

	// Closed to the notification area before: hidden again.
	_, w, sh, _ := testWindow(t)
	w.beforeClose()
	w.hold(true, protocol.BroadcastWindow)
	if w.hidden || !sh.visible {
		t.Fatal("hidden during the broadcast")
	}
	w.hold(false, "")
	if !w.hidden || sh.visible {
		t.Fatal("not back to the notification area")
	}

	// Maximized before, v0.4 view: it stays maximized.
	_, w, sh, _ = testWindow(t)
	sh.maximised = true
	w.hold(true, "")
	w.hold(false, "")
	if !sh.maximised {
		t.Fatal("a window that was maximized was restored")
	}

	// A second broadcast changes the mode; the first state is what comes back.
	_, w, _, nat = testWindow(t)
	_ = w.setCompact(true)
	w.hold(true, protocol.BroadcastWindow)
	w.hold(true, protocol.BroadcastFullscreen)
	if nat.lastHold() != platform.HoldFullscreen {
		t.Fatalf("holds %v", nat.holds)
	}
	w.hold(false, "")
	if !w.compact {
		t.Fatal("the state before the first broadcast was lost")
	}
}

// TestHoldWithoutNativeControl: where platform.AppWindow is not supported,
// Wails keeps the window on top or in full screen, and takes it back.
func TestHoldWithoutNativeControl(t *testing.T) {
	_, w, sh, _ := testWindow(t)
	w.win = nil
	w.hold(true, protocol.BroadcastFullscreen)
	if !sh.did("fullscreen") || !sh.onTopNow {
		t.Fatalf("calls %v", sh.calls)
	}
	w.hold(false, "")
	if !sh.did("unfullscreen") || sh.onTopNow {
		t.Fatalf("calls %v", sh.calls)
	}
}
