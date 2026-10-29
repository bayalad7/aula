package screen

import (
	"context"
	"errors"
	"image"
	"image/color"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// events records, in order, what the doubles saw.
type events struct {
	mu  sync.Mutex
	log []string
}

func (e *events) add(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.log = append(e.log, s)
}

func (e *events) list() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.log...)
}

type fakeCapture struct {
	ev   *events
	err  error
	size int // JPEG bytes returned; 0 = a real encoded image
}

func (c *fakeCapture) Capture(_ context.Context, maxWidth, quality int) (platform.Frame, error) {
	c.ev.add("capture")
	if c.err != nil {
		return platform.Frame{}, c.err
	}
	if c.size > 0 {
		// Pretend the size halves as the quality drops.
		return platform.Frame{JPEG: make([]byte, c.size*quality/100), Width: maxWidth, Height: maxWidth / 2}, nil
	}
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for i := range img.Pix {
		img.Pix[i] = byte(i)
	}
	img.Set(0, 0, color.White)
	return platform.EncodeFrame(img, maxWidth, quality)
}

type fakeIndicator struct {
	ev      *events
	showErr error
}

func (i *fakeIndicator) Show(text string) error {
	if i.showErr != nil {
		return i.showErr
	}
	i.ev.add("show:" + text)
	return nil
}

func (i *fakeIndicator) Hide() error {
	i.ev.add("hide")
	return nil
}

// sink collects what the agent sends.
type sink struct {
	mu   sync.Mutex
	sent []protocol.Payload
	ch   chan protocol.Payload
}

func newSink() *sink { return &sink{ch: make(chan protocol.Payload, 100)} }

func (s *sink) send(_ context.Context, p protocol.Payload) error {
	s.mu.Lock()
	s.sent = append(s.sent, p)
	s.mu.Unlock()
	s.ch <- p
	return nil
}

func (s *sink) next(t *testing.T) protocol.Payload {
	t.Helper()
	select {
	case p := <-s.ch:
		return p
	case <-time.After(3 * time.Second):
		t.Fatal("nothing sent")
		return nil
	}
}

func (s *sink) quiet(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case p := <-s.ch:
		t.Fatalf("unexpected %s", p.MessageType())
	case <-time.After(d):
	}
}

func startEnv(p protocol.ScreenThumbStart) protocol.Envelope { return protocol.New(p) }

var (
	fast = protocol.ScreenThumbStart{IntervalMS: protocol.MinThumbInterval, Quality: 50, MaxWidth: 320}
	tick = time.Duration(protocol.MinThumbInterval) * time.Millisecond
)

func TestStartShowsIndicatorBeforeCapturingAndStopEndsIt(t *testing.T) {
	ev := &events{}
	var viewed []bool
	s := &Streamer{Capture: &fakeCapture{ev: ev}, Indicator: &fakeIndicator{ev: ev},
		OnViewed: func(v bool) { viewed = append(viewed, v) }}
	out := newSink()
	env := startEnv(fast)
	if reply, handled := s.Handle(t.Context(), out.send, env); !handled || reply != nil {
		t.Fatalf("Handle(start) = %v, %v", reply, handled)
	}
	for want := 1; want <= 2; want++ {
		f, ok := out.next(t).(protocol.ScreenThumbFrame)
		if !ok || f.Seq != want || f.RefID != env.ID || f.Width != 320 || f.Height != 180 {
			t.Fatalf("frame %d = %+v", want, f)
		}
		if err := f.Validate(); err != nil {
			t.Fatalf("frame %d invalid: %v", want, err)
		}
	}
	if _, handled := s.Handle(t.Context(), out.send, protocol.New(protocol.ScreenThumbStop{})); !handled {
		t.Fatal("stop not handled")
	}
	out.quiet(t, 2*tick)
	got := ev.list()
	if got[0] != "show:"+IndicatorText || got[1] != "capture" || got[len(got)-1] != "hide" {
		t.Fatalf("events = %v; want the indicator before the first capture and hidden at the end", got)
	}
	if len(viewed) != 2 || !viewed[0] || viewed[1] {
		t.Fatalf("OnViewed = %v, want [true false]", viewed)
	}
}

func TestStopOnDisconnectAndIdempotent(t *testing.T) {
	ev := &events{}
	s := &Streamer{Capture: &fakeCapture{ev: ev}, Indicator: &fakeIndicator{ev: ev}}
	out := newSink()
	s.Handle(t.Context(), out.send, startEnv(fast))
	out.next(t)
	s.Stop() // what the agent does when the connection closes
	s.Stop()
	out.quiet(t, 2*tick)
	hides := 0
	for _, e := range ev.list() {
		if e == "hide" {
			hides++
		}
	}
	if hides != 1 {
		t.Fatalf("indicator hidden %d times, want 1", hides)
	}
}

func TestNewStartReplacesParameters(t *testing.T) {
	ev := &events{}
	s := &Streamer{Capture: &fakeCapture{ev: ev}, Indicator: &fakeIndicator{ev: ev}}
	defer s.Stop()
	out := newSink()
	s.Handle(t.Context(), out.send, startEnv(fast))
	out.next(t)
	detail := fast
	detail.MaxWidth = 1280
	env := startEnv(detail)
	s.Handle(t.Context(), out.send, env)
	for {
		f := out.next(t).(protocol.ScreenThumbFrame)
		if f.RefID != env.ID {
			continue // a frame of the first start already queued
		}
		if f.Seq != 1 || f.Width != 1280 {
			t.Fatalf("first frame of the new start = seq %d width %d", f.Seq, f.Width)
		}
		break
	}
	for _, e := range ev.list() {
		if e == "hide" {
			t.Fatal("the indicator must stay on while the stream is replaced")
		}
	}
}

func TestNoIndicatorNoCapture(t *testing.T) {
	ev := &events{}
	s := &Streamer{Capture: &fakeCapture{ev: ev}, Indicator: &fakeIndicator{ev: ev, showErr: errors.New("no desktop")}}
	out := newSink()
	reply, _ := s.Handle(t.Context(), out.send, startEnv(fast))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInternal {
		t.Fatalf("reply = %#v, want internal error", reply)
	}
	out.quiet(t, 300*time.Millisecond)
	if len(ev.list()) != 0 {
		t.Fatalf("captured without an indicator: %v", ev.list())
	}
}

func TestUnsupportedWithoutCapture(t *testing.T) {
	s := &Streamer{}
	if len(s.Capabilities()) != 0 {
		t.Fatal("must not declare screen.thumb without capture")
	}
	reply, handled := s.Handle(t.Context(), newSink().send, startEnv(fast))
	if e, ok := reply.(protocol.ErrorPayload); !handled || !ok || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply = %#v", reply)
	}
	if _, handled := s.Handle(t.Context(), nil, protocol.New(protocol.MsgAck{RefID: protocol.NewID()})); handled {
		t.Fatal("other messages must not be handled")
	}
	ok := &Streamer{Capture: &fakeCapture{}, Indicator: &fakeIndicator{}}
	if c := ok.Capabilities(); len(c) != 1 || c[0] != protocol.CapScreenThumb {
		t.Fatalf("Capabilities = %v", c)
	}
}

func TestCaptureErrorReportedOnce(t *testing.T) {
	ev := &events{}
	s := &Streamer{Capture: &fakeCapture{ev: ev, err: errors.New("locked")}, Indicator: &fakeIndicator{ev: ev}}
	defer s.Stop()
	out := newSink()
	env := startEnv(fast)
	s.Handle(t.Context(), out.send, env)
	if e, ok := out.next(t).(protocol.ErrorPayload); !ok || e.RefID != env.ID || e.Code != protocol.CodeInternal {
		t.Fatalf("first message = %#v", e)
	}
	out.quiet(t, 3*tick)
}

func TestOversizedFrameLowersQuality(t *testing.T) {
	ev := &events{}
	c := &fakeCapture{ev: ev, size: 2 * protocol.MaxFrameJPEG}
	s := &Streamer{Capture: c, Indicator: &fakeIndicator{ev: ev}}
	defer s.Stop()
	out := newSink()
	s.Handle(t.Context(), out.send, startEnv(protocol.ScreenThumbStart{IntervalMS: 1000, Quality: 90, MaxWidth: 320}))
	f, ok := out.next(t).(protocol.ScreenThumbFrame)
	if !ok || len(f.JPEG) > protocol.MaxFrameJPEG {
		t.Fatalf("got %T with %d bytes", f, len(f.JPEG))
	}
}

func TestWall(t *testing.T) {
	thumbCap := []protocol.Capability{protocol.CapCodeView, protocol.CapScreenThumb}
	sessions := []session.Session{
		{ID: "a", State: session.StateConnected, Capabilities: thumbCap},
		{ID: "b", State: session.StateConnected, Capabilities: thumbCap},
		{ID: "old", State: session.StateConnected, Capabilities: []protocol.Capability{protocol.CapCodeView}},
		{ID: "gone", State: session.StateDisconnected, Capabilities: thumbCap},
	}
	w := NewWall()
	if cmds := w.Plan(sessions); len(cmds) != 0 {
		t.Fatalf("closed wall planned %v", cmds)
	}
	w.Open()
	cmds := w.Plan(sessions)
	if len(cmds) != 2 {
		t.Fatalf("Plan = %v, want starts for a and b only", cmds)
	}
	for _, c := range cmds {
		w.Started(c.SessionID, "start-"+c.SessionID, c.Payload.(protocol.ScreenThumbStart))
	}
	if cmds := w.Plan(sessions); len(cmds) != 0 {
		t.Fatalf("second Plan = %v, want nothing new", cmds)
	}

	if _, ok := w.Frame("a", protocol.ScreenThumbFrame{RefID: "other", Seq: 1}); ok {
		t.Fatal("frame of an old start accepted")
	}
	if th, ok := w.Frame("a", protocol.ScreenThumbFrame{RefID: "start-a", Seq: 3, Width: 320, Height: 180, JPEG: []byte{1}}); !ok || th.Seq != 3 {
		t.Fatalf("Frame = %+v, %v", th, ok)
	}

	w.SetDetail("b")
	cmds = w.Plan(sessions)
	if len(cmds) != 1 || cmds[0].SessionID != "b" || cmds[0].Payload != DetailParams {
		t.Fatalf("detail Plan = %v", cmds)
	}
	w.Started("b", "start-b2", DetailParams)
	w.SetDetail("")
	if cmds = w.Plan(sessions); len(cmds) != 1 || cmds[0].Payload != ThumbParams {
		t.Fatalf("back to grid Plan = %v", cmds)
	}
	w.Started("b", "start-b3", ThumbParams)

	// "a" leaves: forgotten, frames dropped.
	w.Plan(sessions[1:])
	if len(w.Thumbs()) != 0 {
		t.Fatalf("thumbs of a departed session kept: %v", w.Thumbs())
	}

	stops := w.Close()
	if len(stops) != 1 || stops[0].SessionID != "b" {
		t.Fatalf("Close = %v, want a stop for b", stops)
	}
	if _, ok := stops[0].Payload.(protocol.ScreenThumbStop); !ok {
		t.Fatalf("Close payload = %T", stops[0].Payload)
	}
	if w.IsOpen() || len(w.Thumbs()) != 0 {
		t.Fatal("closed wall keeps state")
	}
	if _, ok := w.Frame("b", protocol.ScreenThumbFrame{RefID: "start-b3", Seq: 1}); ok {
		t.Fatal("frame accepted after Close")
	}
}
