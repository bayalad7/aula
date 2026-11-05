package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/remote"
	"github.com/bayalad7/aula/internal/session"
)

// fakeInjector stands in for SendInput: tests never type on a real desktop.
type fakeInjector struct {
	mu  sync.Mutex
	got []platform.InputEvent
}

func (f *fakeInjector) Inject(e []platform.InputEvent) error {
	f.mu.Lock()
	f.got = append(f.got, e...)
	f.mu.Unlock()
	return nil
}
func (f *fakeInjector) LocalInput() (time.Time, error) { return time.Time{}, nil }
func (f *fakeInjector) count() int                     { f.mu.Lock(); defer f.mu.Unlock(); return len(f.got) }

// A control raises the window and shows the band (status.control) and the
// native notice before the first event; the end and the disconnection remove
// them and release what stayed pressed.
func TestRemoteControlRaisesShowsTheBandAndEnds(t *testing.T) {
	a, rec := newTestApp(nil)
	ind := &stubIndicator{}
	in := &fakeInjector{}
	a.enableRemote(in) // without the screen: not declared
	if protocol.Has(a.hello.Capabilities, protocol.CapRemote) || a.control.Available() {
		t.Fatal("remote must not be declared without the screen and its notice")
	}
	a.enableScreen(stubCapture{}, ind)
	a.enableRemote(in)
	if !protocol.Has(a.hello.Capabilities, protocol.CapRemote) {
		t.Fatalf("capabilities %v", a.hello.Capabilities)
	}
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ayala"})

	out := make(chan protocol.Payload, 50)
	send := func(_ context.Context, p protocol.Payload) error { out <- p; return nil }
	start := remote.ViewParams
	start.ControlID = protocol.NewID()
	env := protocol.New(start)
	reply, ok := a.control.Handle(t.Context(), send, env)
	if ack, _ := reply.(protocol.RemoteAck); !ok || ack.Status != protocol.RemoteActive {
		t.Fatalf("reply %+v", reply)
	}
	st := rec.last(EventStatus).(Status)
	if st.Control != ControlActive || !st.Viewed || !ind.on() || rec.raised != 1 {
		t.Fatalf("status %+v, indicator %v, raised %d", st, ind.on(), rec.raised)
	}
	a.control.Handle(t.Context(), send, protocol.New(protocol.RemoteInput{ControlID: start.ControlID, Seq: 1,
		Events: []protocol.InputEvent{{T: protocol.InputKey, Code: "KeyA", Down: true}}}))
	if in.count() != 1 {
		t.Fatalf("injected %d", in.count())
	}
	select {
	case p := <-out:
		if f, ok := p.(protocol.ScreenThumbFrame); !ok || f.RefID != env.ID {
			t.Fatalf("video %T, not a frame of the control", p)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no video")
	}
	// The connection closes: the band, the notice and the key go.
	a.onState(session.AgentDisconnected, discovery.Console{Name: "Prof. Ayala"})
	st = rec.last(EventStatus).(Status)
	if st.Control != "" || ind.on() || in.count() != 2 {
		t.Fatalf("after the disconnection: status %+v, indicator %v, events %d", st, ind.on(), in.count())
	}
	if rec.raised != 1 {
		t.Fatalf("ending must not raise the window (raised %d)", rec.raised)
	}
}

// As a service, remote is declared only with a helper that can both capture
// (the screen and its notice) and inject in the student's session.
func TestServiceDeclaresRemoteOnlyWithItsHelper(t *testing.T) {
	sa := newServiceAgent(Config{Channel: "1"}, nil, nil, quiet)
	for _, tc := range []struct {
		ready *agentsvc.Ready
		want  bool
	}{
		{nil, false},
		{&agentsvc.Ready{Capture: true}, false},
		{&agentsvc.Ready{Input: true}, false},
		{&agentsvc.Ready{Capture: true, Input: true}, true},
	} {
		sa.mu.Lock()
		sa.ready = tc.ready
		sa.mu.Unlock()
		if got := protocol.Has(sa.hello().Capabilities, protocol.CapRemote); got != tc.want {
			t.Errorf("ready %+v: remote declared %v, want %v", tc.ready, got, tc.want)
		}
	}
	if !sa.app.control.Available() {
		t.Fatal("the service's control must inject through the helper")
	}
}
