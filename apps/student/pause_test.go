package main

import (
	"strings"
	"sync"
	"testing"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/remote"
	"github.com/bayalad7/aula/internal/session"
)

// stubPause stands in for the pause window: tests never cover the desktop.
type stubPause struct {
	mu    sync.Mutex
	shown bool
	text  platform.PauseText
	yield bool
	shows int
}

func (p *stubPause) Show(t platform.PauseText) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shown, p.text = true, t
	p.shows++
	return nil
}

func (p *stubPause) Yield(on bool) error { p.mu.Lock(); p.yield = on; p.mu.Unlock(); return nil }
func (p *stubPause) Hide() error         { p.mu.Lock(); p.shown = false; p.mu.Unlock(); return nil }
func (p *stubPause) state() (bool, bool, platform.PauseText) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shown, p.yield, p.text
}

// A pause covers the screen, raises the window and shows the band; a second
// one only changes the message; the disconnection removes it (rule 10).
func TestPauseShowsTheBandAndEndsWithTheConnection(t *testing.T) {
	a, rec := newTestApp(nil)
	if protocol.Has(a.hello.Capabilities, protocol.CapScreenFreeze) {
		t.Fatal("no pause window, no capability")
	}
	o := &stubPause{}
	a.enableFreeze(o)
	if !protocol.Has(a.hello.Capabilities, protocol.CapScreenFreeze) {
		t.Fatalf("capabilities %v", a.hello.Capabilities)
	}
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ayala"})

	env := protocol.New(protocol.ScreenFreeze{Message: "Miren al pizarrón"})
	reply, ok := a.freeze.Handle(t.Context(), env)
	if !ok || reply != (protocol.ScreenFreezeAck{RefID: env.ID, Frozen: true}) {
		t.Fatalf("reply %#v", reply)
	}
	shown, _, text := o.state()
	st := rec.last(EventStatus).(Status)
	if !shown || !st.Paused || rec.raised != 1 {
		t.Fatalf("shown %v, status %+v, raised %d", shown, st, rec.raised)
	}
	if text.Message != "Miren al pizarrón" || text.Detail == "" || !strings.Contains(text.Detail, "Prof. Ayala · canal ") {
		t.Fatalf("text %+v", text)
	}
	a.freeze.Handle(t.Context(), protocol.New(protocol.ScreenFreeze{Message: "Ya casi"}))
	if _, _, text := o.state(); text.Message != "Ya casi" || rec.raised != 1 {
		t.Fatalf("the second pause only changes the message: %+v, raised %d", text, rec.raised)
	}

	a.onState(session.AgentDisconnected, discovery.Console{Name: "Prof. Ayala"})
	if shown, _, _ := o.state(); shown || rec.last(EventStatus).(Status).Paused {
		t.Fatal("the pause must go away with the connection")
	}
}

// What the student must see stays in front of the pause: the broadcast, the
// power countdown and the remote control.
func TestPauseYieldsToWhatTheStudentMustSee(t *testing.T) {
	a, _ := newTestApp(nil)
	o := &stubPause{}
	a.enableFreeze(o)
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ayala"})
	a.freeze.Handle(t.Context(), protocol.New(protocol.ScreenFreeze{}))

	if err := a.onBroadcastView(broadcast.View{Active: true, BroadcastID: protocol.NewID()}); err != nil {
		t.Fatal(err)
	}
	if _, y, _ := o.state(); !y {
		t.Fatal("the broadcast must stay in front of the pause")
	}
	if err := a.onBroadcastView(broadcast.View{}); err != nil {
		t.Fatal(err)
	}
	if _, y, _ := o.state(); y {
		t.Fatal("after the broadcast the pause covers everything again")
	}
	a.onControl(remote.Active)
	if _, y, _ := o.state(); !y {
		t.Fatal("the control notice must stay in front of the pause")
	}
	a.onControl(remote.Off)
	if _, y, _ := o.state(); y {
		t.Fatal("after the control the pause covers everything again")
	}
}

// As a service, screen.freeze is declared only with a helper that has the
// pause window, and the window goes through it.
func TestServiceDeclaresFreezeOnlyWithItsHelper(t *testing.T) {
	sa := newServiceAgent(Config{Channel: "1"}, nil, nil, quiet)
	for _, tc := range []struct {
		ready *agentsvc.Ready
		want  bool
	}{
		{nil, false},
		{&agentsvc.Ready{Capture: true}, false},
		{&agentsvc.Ready{Pause: true}, true},
	} {
		sa.mu.Lock()
		sa.ready = tc.ready
		sa.mu.Unlock()
		if got := protocol.Has(sa.hello().Capabilities, protocol.CapScreenFreeze); got != tc.want {
			t.Errorf("ready %+v: screen.freeze declared %v, want %v", tc.ready, got, tc.want)
		}
	}
	if _, ok := sa.app.freeze.Overlay.(agentsvc.Pause); !ok {
		t.Fatalf("the service's pause must go through the helper: %T", sa.app.freeze.Overlay)
	}
}
