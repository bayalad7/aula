package main

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// stubPublisher stands in for the capture, the encoder and pion.
type stubPublisher struct{ closed chan struct{} }

func (s *stubPublisher) Open(broadcast.PublishOptions, broadcast.PublishEvents) (broadcast.Publication, error) {
	s.closed = make(chan struct{})
	return stubPublication{s.closed}, nil
}

type stubPublication struct{ closed chan struct{} }

func (stubPublication) Size() (int, int)                            { return 1280, 720 }
func (stubPublication) Answer(int, string) (string, error)          { return "v=0", nil }
func (stubPublication) AddCandidate(int, broadcast.Candidate) error { return nil }
func (p stubPublication) Close() error                              { close(p.closed); return nil }

// The teacher decides: no card and nothing for the student to answer. The
// start brings the window forward (raise) with the band (status presenting)
// and the native badge before the capture opens; the disconnection removes
// them, and the window has no binding to end it.
func TestPresentRaisesShowsTheBandAndEnds(t *testing.T) {
	a, rec := newTestApp(nil)
	pub := &stubPublisher{}
	a.enablePresent(pub) // without the badge: not declared
	if protocol.Has(a.hello.Capabilities, protocol.CapScreenPresent) {
		t.Fatal("screen.present declared without the native badge")
	}
	ind := &stubIndicator{}
	a.enableScreen(stubCapture{}, ind)
	a.enablePresent(pub)
	if !protocol.Has(a.hello.Capabilities, protocol.CapScreenPresent) {
		t.Fatalf("capabilities %v", a.hello.Capabilities)
	}
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ayala"})

	out := make(chan protocol.Payload, 20)
	send := func(_ context.Context, p protocol.Payload) error { out <- p; return nil }
	id := protocol.NewID()
	a.onMessageForTest(send, protocol.StudentPresentStart{PresentID: id, MaxWidth: 1920, FPS: 15, MaxKbps: 2500})
	a.present.Wait()
	if v, _ := rec.last(EventPresent).(broadcast.PresentView); v.Phase != broadcast.PresentLive || v.PresentID != id {
		t.Fatalf("present event %+v", rec.last(EventPresent))
	}
	if rec.raised != 1 {
		t.Fatalf("raised %d, want 1", rec.raised)
	}
	if st := rec.last(EventStatus).(Status); !st.Presenting || !ind.on() {
		t.Fatalf("status %+v, badge %v", st, ind.on())
	}
	if v := a.View(); v.Present.Phase != broadcast.PresentLive {
		t.Fatalf("View().Present %+v", v.Present)
	}
	waitAck(t, out, protocol.PresentAccepted)

	// The connection closes: the band, the badge and the capture go.
	a.onState(session.AgentDisconnected, discovery.Console{Name: "Prof. Ayala"})
	a.present.Wait()
	select {
	case <-pub.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the capture goes on after the disconnection")
	}
	if st := rec.last(EventStatus).(Status); st.Presenting || ind.on() {
		t.Fatalf("after disconnect: status %+v, badge %v", st, ind.on())
	}
	if rec.raised != 1 {
		t.Fatalf("raised again at the end: %d", rec.raised)
	}
}

// The window cannot end a presentation: no binding of App touches it.
func TestNoBindingEndsAPresentation(t *testing.T) {
	app := reflect.TypeOf(&App{})
	for i := range app.NumMethod() {
		if name := app.Method(i).Name; strings.HasPrefix(name, "Present") {
			t.Errorf("binding App.%s: the student must not be able to end or refuse a presentation (rule 77)", name)
		}
	}
}

// The thumbnails and a presentation share the badge: ending one never hides
// what the other shows.
func TestPresentSharesTheBadgeWithTheThumbnails(t *testing.T) {
	a, _ := newTestApp(nil)
	ind := &stubIndicator{}
	a.enableScreen(stubCapture{}, ind)
	a.enablePresent(&stubPublisher{})
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ayala"})
	send := func(context.Context, protocol.Payload) error { return nil }
	id := protocol.NewID()
	a.onMessageForTest(send, protocol.StudentPresentStart{PresentID: id, MaxWidth: 1920, FPS: 15, MaxKbps: 2500})
	_ = a.badge.Stream().Show("vista")
	_ = a.badge.Stream().Hide() // the thumbnails end
	if !ind.on() {
		t.Fatal("the presentation's badge went with the thumbnails")
	}
	a.onMessageForTest(send, protocol.StudentPresentStop{PresentID: id})
	a.present.Wait()
	if ind.on() {
		t.Fatal("the badge stayed after the presentation")
	}
}

// onMessageForTest hands a console message to the presenter the way
// onMessage does, with its reply on the same connection.
func (a *App) onMessageForTest(send func(context.Context, protocol.Payload) error, p protocol.Payload) {
	if reply, ok := a.present.Handle(context.Background(), send, protocol.New(p)); ok && reply != nil {
		_ = send(context.Background(), reply)
	}
}

func waitAck(t *testing.T, out chan protocol.Payload, want protocol.PresentStatus) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case p := <-out:
			if a, ok := p.(protocol.StudentPresentAck); ok && a.Status == want {
				return
			}
		case <-deadline:
			t.Fatalf("no ack %s", want)
		}
	}
}
