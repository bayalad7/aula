package broadcast

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// TestPresentationEndToEnd runs the whole path with pion over loopback: the
// source's Presenter and LocalPublisher (fake capture and encoder), the
// console's Projector with the real relay, and two receivers behind the
// real Viewer. Messages round-trip through the protocol encoder.
func TestPresentationEndToEnd(t *testing.T) {
	var keys atomic.Int64
	var excluded atomic.Int32
	var smu sync.Mutex
	students := map[string]*fakeStudent{}
	var proj *Projector
	toConsole := func(sessionID string, p protocol.Payload) { proj.Handle(sessionID, roundTrip(t, p)) }

	// The source agent: the teacher decides, nobody is asked.
	badge := &fakeIndicator{}
	presenter := &Presenter{
		Publisher: &LocalPublisher{
			Capture: &fakeDisplays{},
			NewEncoder: func(cfg platform.VideoEncoderConfig) (platform.VideoEncoder, error) {
				return &fakeEncoder{cfg: cfg, keys: &keys}, nil
			},
			Exclude: func(on bool) error {
				if on {
					excluded.Add(1)
				} else {
					excluded.Add(-1)
				}
				return nil
			},
			Loopback: true,
		},
		Indicator: badge,
		OnView:    func(PresentView) {},
	}
	srcInbox := make(chan protocol.Envelope, 64)
	srcSend := func(_ context.Context, p protocol.Payload) error { toConsole("s", p); return nil }
	go func() {
		for e := range srcInbox {
			if reply, _ := presenter.Handle(context.Background(), srcSend, e); reply != nil {
				toConsole("s", reply)
			}
		}
	}()
	t.Cleanup(func() { presenter.Stop(); close(srcInbox) })

	proj = &Projector{
		Send: func(_ context.Context, sessionID string, p protocol.Payload) (string, error) {
			e := roundTrip(t, p)
			if sessionID == "s" {
				srcInbox <- e
				return e.ID, nil
			}
			smu.Lock()
			s := students[sessionID]
			smu.Unlock()
			if s == nil || !s.deliver(e) {
				return "", errors.New("no such session")
			}
			return e.ID, nil
		},
		Media: MediaOptions{UDPPort: 0},
	}
	t.Cleanup(proj.Stop)
	for _, id := range []string{"a", "b"} {
		s := newFakeStudent(t, id, toConsole)
		smu.Lock()
		students[id] = s
		smu.Unlock()
	}
	source := student("s", "LAB3-PC07", sourceCaps...)
	class := []session.Session{source, student("a", "LAB3-PC01", viewCaps...), student("b", "LAB3-PC02", viewCaps...)}
	if err := proj.Start(source, class); err != nil {
		t.Fatal(err)
	}
	a, b := students["a"], students["b"]
	waitFor(t, 30*time.Second, "both receivers get the source's video", func() bool {
		return a.packets.Load() > 10 && b.packets.Load() > 10
	})
	if s := proj.Status(); s.Phase != ProjectLive || s.Receivers != 2 {
		t.Fatalf("status %+v", s)
	}
	if v := a.lastView(); !v.Active || v.Source != "LAB3-PC07" || v.Width != 640 || v.Height != 360 {
		t.Fatalf("receiver view %+v", v)
	}
	if badge.shown() != PresentIndicatorText || excluded.Load() != 1 {
		t.Fatalf("badge %q, windows excluded %d", badge.shown(), excluded.Load())
	}
	if keys.Load() == 0 {
		t.Fatal("no keyframe")
	}

	proj.Stop()
	waitFor(t, 5*time.Second, "everything cleared", func() bool {
		return !a.lastView().Active && !b.lastView().Active && badge.shown() == "" &&
			presenter.Current().Phase == PresentOff && excluded.Load() == 0
	})
}
