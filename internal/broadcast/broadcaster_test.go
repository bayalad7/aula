package broadcast

import (
	"context"
	"errors"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

type fakeDisplays struct{ grabs atomic.Int64 }

func (*fakeDisplays) Displays() ([]platform.Display, error) {
	return []platform.Display{{Index: 0, Bounds: image.Rect(0, 0, 640, 360), Primary: true}}, nil
}

// Grab returns a screen that changes every time (a still one would only be
// encoded once per second).
func (d *fakeDisplays) Grab(context.Context, int) (*image.RGBA, error) {
	n := d.grabs.Add(1)
	img := image.NewRGBA(image.Rect(0, 0, 640, 360))
	img.Pix[n%int64(len(img.Pix))] = 0xff
	return img, nil
}

// fakeEncoder emits one small NAL unit per frame: an IDR when asked or on
// the first frame, otherwise a P slice.
type fakeEncoder struct {
	cfg    platform.VideoEncoderConfig
	frames int
	keys   *atomic.Int64
}

func (e *fakeEncoder) Encode(_ *image.RGBA, forceKey bool) ([]byte, error) {
	e.frames++
	nal := byte(0x41) // non-IDR slice
	if forceKey || e.frames == 1 {
		nal = 0x65 // IDR
		e.keys.Add(1)
	}
	return append([]byte{0, 0, 0, 1, nal}, make([]byte, 600)...), nil
}

func (e *fakeEncoder) Close() error { return nil }

// fakeStudent is an agent: the real Viewer plus a pion PeerConnection in
// place of the frontend's RTCPeerConnection.
type fakeStudent struct {
	t       *testing.T
	id      string
	viewer  *Viewer
	inbox   chan protocol.Envelope // console → agent, in order
	packets atomic.Int64

	mu    sync.Mutex
	pc    *webrtc.PeerConnection
	views []View

	// inboxMu guards inbox against the test's cleanup: the broadcast may
	// still send (its stop) after a student's cleanup ran, when an earlier
	// check failed. Then the message is dropped instead of panicking, so the
	// real failure shows.
	inboxMu sync.Mutex
	closed  bool
}

// deliver hands a console message to the student, unless it is gone.
func (s *fakeStudent) deliver(e protocol.Envelope) bool {
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()
	if s.closed {
		return false
	}
	s.inbox <- e
	return true
}

func newFakeStudent(t *testing.T, id string, toConsole func(sessionID string, p protocol.Payload)) *fakeStudent {
	s := &fakeStudent{t: t, id: id, inbox: make(chan protocol.Envelope, 64)}
	send := func(_ context.Context, p protocol.Payload) error {
		toConsole(id, p)
		return nil
	}
	s.viewer = &Viewer{
		Supported: true,
		OnView: func(v View) error {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.views = append(s.views, v)
			if !v.Active && s.pc != nil {
				s.pc.Close()
				s.pc = nil
			}
			return nil
		},
		OnSignal: func(sig Signal) { go s.signal(sig) },
	}
	go func() {
		for e := range s.inbox {
			if reply, _ := s.viewer.Handle(context.Background(), send, e); reply != nil {
				toConsole(id, reply)
			}
		}
	}()
	t.Cleanup(func() {
		s.inboxMu.Lock()
		s.closed = true
		close(s.inbox)
		s.inboxMu.Unlock()
		s.mu.Lock()
		if s.pc != nil {
			s.pc.Close()
		}
		s.mu.Unlock()
	})
	return s
}

// signal plays the frontend: a new offer replaces the connection.
func (s *fakeStudent) signal(sig Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch sig.Kind {
	case SignalOffer:
		if s.pc != nil {
			s.pc.Close()
		}
		var se webrtc.SettingEngine
		se.SetIncludeLoopbackCandidate(true)
		me := &webrtc.MediaEngine{}
		if err := me.RegisterDefaultCodecs(); err != nil {
			s.t.Error(err)
			return
		}
		pc, err := webrtc.NewAPI(webrtc.WithSettingEngine(se), webrtc.WithMediaEngine(me)).NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			s.t.Error(err)
			return
		}
		s.pc = pc
		id, attempt := sig.BroadcastID, sig.Attempt
		pc.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c == nil {
				return
			}
			go func() { _ = s.viewer.LocalCandidate(id, attempt, c.ToJSON().Candidate, "0", 0) }()
		})
		pc.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
			for {
				if _, _, err := tr.ReadRTP(); err != nil {
					return
				}
				s.packets.Add(1)
			}
		})
		if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sig.SDP}); err != nil {
			s.t.Errorf("set offer: %v", err)
			return
		}
		answer, err := pc.CreateAnswer(nil)
		if err == nil {
			err = pc.SetLocalDescription(answer)
		}
		if err != nil {
			s.t.Errorf("answer: %v", err)
			return
		}
		sdp := pc.LocalDescription().SDP
		go func() {
			if err := s.viewer.Answer(id, attempt, sdp); err != nil {
				s.t.Errorf("viewer answer: %v", err)
			}
		}()
	case SignalICE:
		if s.pc != nil && sig.Candidate != "" {
			mid := sig.SDPMid
			_ = s.pc.AddICECandidate(webrtc.ICECandidateInit{Candidate: sig.Candidate, SDPMid: &mid})
		}
	}
}

func (s *fakeStudent) lastView() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.views) == 0 {
		return View{}
	}
	return s.views[len(s.views)-1]
}

// TestBroadcastEndToEnd runs the real console stack (pion, ICE-lite on one
// UDP port, shared track) against pion receivers behind the real Viewer,
// over loopback. Messages round-trip through the protocol encoder.
func TestBroadcastEndToEnd(t *testing.T) {
	var keys atomic.Int64
	students := map[string]*fakeStudent{}
	var smu sync.Mutex
	var b *Broadcaster
	toConsole := func(sessionID string, p protocol.Payload) { b.Handle(sessionID, roundTrip(t, p)) }
	b = &Broadcaster{
		Capture: &fakeDisplays{},
		NewEncoder: func(cfg platform.VideoEncoderConfig) (platform.VideoEncoder, error) {
			if cfg.Width != 640 || cfg.Height != 360 || cfg.FPS != DefaultFPS {
				t.Errorf("encoder config %+v", cfg)
			}
			return &fakeEncoder{cfg: cfg, keys: &keys}, nil
		},
		Send: func(_ context.Context, sessionID string, p protocol.Payload) (string, error) {
			smu.Lock()
			s := students[sessionID]
			smu.Unlock()
			if s == nil {
				return "", errors.New("no such session")
			}
			e := roundTrip(t, p)
			if !s.deliver(e) {
				return "", errors.New("session closed")
			}
			return e.ID, nil
		},
		Media: MediaOptions{UDPPort: 0, Loopback: true},
	}
	join := func(id string) *fakeStudent {
		s := newFakeStudent(t, id, toConsole)
		smu.Lock()
		students[id] = s
		smu.Unlock()
		return s
	}
	a, c := join("a"), join("c")
	class := []session.Session{agent("a", protocol.CapScreenBroadcast), agent("c", protocol.CapScreenBroadcast), agent("old")}

	if err := b.Start(Options{}, class); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Stop)
	if err := b.Start(Options{}, class); !errors.Is(err, ErrAlreadyBroadcasting) {
		t.Fatalf("second Start: %v", err)
	}
	waitFor(t, 30*time.Second, "both students receive video", func() bool {
		return a.packets.Load() > 10 && c.packets.Load() > 10
	})
	if s := b.Status(); !s.Active || s.Receivers != 2 || s.Eligible != 2 {
		t.Fatalf("status %+v", s)
	}
	if v := a.lastView(); !v.Active || v.Width != 640 || v.Height != 360 {
		t.Fatalf("student view %+v", v)
	}

	// c drops (its session closes) and comes back as a new session: it is
	// offered the broadcast again and receives video.
	c.viewer.Stop()
	b.Sync([]session.Session{agent("a", protocol.CapScreenBroadcast)})
	waitFor(t, 5*time.Second, "c dropped", func() bool { s := b.Status(); return s.Eligible == 1 && s.Receivers == 1 })
	before := keys.Load()
	c2 := join("c2")
	b.Sync([]session.Session{agent("a", protocol.CapScreenBroadcast), agent("c2", protocol.CapScreenBroadcast)})
	waitFor(t, 30*time.Second, "c2 receives video", func() bool { return c2.packets.Load() > 10 })
	waitFor(t, 5*time.Second, "two receivers again", func() bool { return b.Status().Receivers == 2 })
	if keys.Load() <= before {
		t.Error("no keyframe for the student who came back")
	}

	// Stop: every student hides the view; the UDP port is released.
	b.Stop()
	waitFor(t, 5*time.Second, "views hidden", func() bool { return !a.lastView().Active && !c2.lastView().Active })
	if s := b.Status(); s.Active {
		t.Fatalf("status after stop %+v", s)
	}
	// A new broadcast can start right away.
	if err := b.Start(Options{}, nil); err != nil {
		t.Fatalf("restart: %v", err)
	}
	b.Stop()
}

func TestBroadcastFailsBeforeSendingWithoutEncoder(t *testing.T) {
	sent := 0
	b := &Broadcaster{
		Capture: &fakeDisplays{},
		NewEncoder: func(platform.VideoEncoderConfig) (platform.VideoEncoder, error) {
			return nil, platform.ErrNotSupported
		},
		Send: func(context.Context, string, protocol.Payload) (string, error) { sent++; return "", nil },
	}
	err := b.Start(Options{}, []session.Session{agent("a", protocol.CapScreenBroadcast)})
	if !errors.Is(err, platform.ErrNotSupported) {
		t.Fatalf("Start = %v", err)
	}
	if sent != 0 || b.Status().Active {
		t.Fatalf("sent %d messages, status %+v", sent, b.Status())
	}
	if err := b.Start(Options{Display: 3}, nil); err == nil {
		t.Fatal("monitor 3 accepted")
	}
}

// TestBroadcastModeAndTargets: the mode reaches every agent in
// screen.broadcast.start, and a broadcast to the selected computers only goes
// to them, also when one reconnects as a new session; a computer that was
// not selected gets nothing when it joins (docs/protocol.md §5.12.1).
func TestBroadcastModeAndTargets(t *testing.T) {
	var mu sync.Mutex
	starts := map[string]protocol.ScreenBroadcastStart{}
	var keys atomic.Int64
	b := &Broadcaster{
		Capture: &fakeDisplays{},
		NewEncoder: func(cfg platform.VideoEncoderConfig) (platform.VideoEncoder, error) {
			return &fakeEncoder{cfg: cfg, keys: &keys}, nil
		},
		Send: func(_ context.Context, sessionID string, p protocol.Payload) (string, error) {
			if st, ok := p.(protocol.ScreenBroadcastStart); ok {
				mu.Lock()
				starts[sessionID] = st
				mu.Unlock()
			}
			return protocol.NewID(), nil
		},
		Media: MediaOptions{UDPPort: 0, Loopback: true},
	}
	host := func(id, hostname string) session.Session {
		s := agent(id, protocol.CapScreenBroadcast)
		s.Hostname = hostname
		return s
	}
	got := func(id string) (protocol.ScreenBroadcastStart, bool) {
		mu.Lock()
		defer mu.Unlock()
		st, ok := starts[id]
		return st, ok
	}
	err := b.Start(Options{Mode: protocol.BroadcastFullscreen, Hostnames: []string{"LAB-PC01"}},
		[]session.Session{host("a", "LAB-PC01"), host("b", "LAB-PC02")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Stop)
	waitFor(t, 5*time.Second, "start sent to the selected computer", func() bool { _, ok := got("a"); return ok })
	if st, _ := got("a"); st.Mode != protocol.BroadcastFullscreen {
		t.Fatalf("start %+v: mode not sent", st)
	}
	if s := b.Status(); s.Mode != protocol.BroadcastFullscreen || s.Targeted != 1 || s.Eligible != 1 {
		t.Fatalf("status %+v", s)
	}
	// a reconnects as a2 (the hostname in another case still matches); c,
	// not selected, joins.
	b.Sync([]session.Session{host("a2", "lab-pc01"), host("b", "LAB-PC02"), host("c", "LAB-PC03")})
	waitFor(t, 5*time.Second, "start sent again on reconnect", func() bool { _, ok := got("a2"); return ok })
	for _, id := range []string{"b", "c"} {
		if _, ok := got(id); ok {
			t.Fatalf("%s was not selected and got the broadcast", id)
		}
	}
	b.Stop()

	// Without a mode, to everyone: as in v0.4.
	mu.Lock()
	clear(starts)
	mu.Unlock()
	if err := b.Start(Options{}, []session.Session{host("a", "LAB-PC01"), host("b", "LAB-PC02")}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, "start sent to everyone", func() bool { _, a := got("a"); _, b := got("b"); return a && b })
	if st, _ := got("b"); st.Mode != "" {
		t.Fatalf("mode %q without asking for one", st.Mode)
	}
	if s := b.Status(); s.Targeted != 0 || s.Eligible != 2 {
		t.Fatalf("status %+v", s)
	}
}

func TestVideoSize(t *testing.T) {
	for _, tt := range []struct {
		w, h, wantW, wantH int
	}{
		{1920, 1080, 1920, 1080},
		{2560, 1440, 1920, 1080},
		{3840, 2160, 1920, 1080},
		{1366, 768, 1366, 768},
		{1365, 767, 1364, 766},
		{1680, 1050, 1680, 1050},
	} {
		w, h := VideoSize(image.Rect(0, 0, tt.w, tt.h), DefaultMaxWidth)
		if w != tt.wantW || h != tt.wantH {
			t.Errorf("VideoSize(%dx%d) = %dx%d, want %dx%d", tt.w, tt.h, w, h, tt.wantW, tt.wantH)
		}
	}
}

func roundTrip(t *testing.T, p protocol.Payload) protocol.Envelope {
	t.Helper()
	data, err := protocol.Encode(protocol.New(p))
	if err != nil {
		t.Fatalf("encode %s: %v", p.MessageType(), err)
	}
	e, err := protocol.Decode(data)
	if err != nil {
		t.Fatalf("decode %s: %v", p.MessageType(), err)
	}
	return e
}

func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
