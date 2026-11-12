package broadcast

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// fakeSourcePeer is the console's connection with the source.
type fakeSourcePeer struct {
	fakePeer
	keys int
}

func (p *fakeSourcePeer) Offer() (string, error) { return sdpWith("recvonly"), nil }

func (p *fakeSourcePeer) RequestKeyframe() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys++
	return nil
}

func (p *fakeSourcePeer) keyframes() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.keys
}

// fakeRelay is the relay media: it hands out fake peers.
type fakeRelay struct {
	mu       sync.Mutex
	peers    []*fakePeer
	sources  []*fakeSourcePeer
	closed   bool
	openOpts MediaOptions
}

func (r *fakeRelay) NewPeer(ev PeerEvents) (Peer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := &fakePeer{ev: ev, n: len(r.peers) + 1}
	r.peers = append(r.peers, p)
	return p, nil
}

func (r *fakeRelay) NewSourcePeer(ev PeerEvents) (SourcePeer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := &fakeSourcePeer{fakePeer: fakePeer{ev: ev, n: len(r.sources) + 1}}
	r.sources = append(r.sources, p)
	return p, nil
}

func (r *fakeRelay) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func (r *fakeRelay) source(i int) *fakeSourcePeer {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i >= len(r.sources) {
		return nil
	}
	return r.sources[i]
}

func (r *fakeRelay) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

type projectorRig struct {
	t     *testing.T
	p     *Projector
	relay *fakeRelay
	out   chan sent

	mu       sync.Mutex
	statuses []ProjectStatus
	views    []View
	signals  []Signal
}

func newProjectorRig(t *testing.T) *projectorRig {
	r := &projectorRig{t: t, relay: &fakeRelay{}, out: make(chan sent, 512)}
	r.p = &Projector{
		Send: func(_ context.Context, sessionID string, m protocol.Payload) (string, error) {
			e := roundTrip(t, m)
			r.out <- sent{sessionID, e}
			return e.ID, nil
		},
		OnStatus: func(s ProjectStatus) { r.mu.Lock(); r.statuses = append(r.statuses, s); r.mu.Unlock() },
		OnView:   func(v View) { r.mu.Lock(); r.views = append(r.views, v); r.mu.Unlock() },
		OnSignal: func(s Signal) { r.mu.Lock(); r.signals = append(r.signals, s); r.mu.Unlock() },
		OpenMedia: func(o MediaOptions) (RelayMedia, error) {
			r.relay.openOpts = o
			return r.relay, nil
		},
		Backoff:         func(int) time.Duration { return 10 * time.Millisecond },
		DisconnectGrace: 50 * time.Millisecond,
		ConnectTimeout:  time.Minute,
	}
	t.Cleanup(r.p.Stop)
	return r
}

// expect waits for the next message of type typ to sessionID, skipping the
// others (they are kept for other expectations).
func (r *projectorRig) expect(sessionID string, typ protocol.Type) protocol.Envelope {
	r.t.Helper()
	deadline := time.After(5 * time.Second)
	var skipped []sent
	defer func() {
		for _, s := range skipped {
			r.out <- s
		}
	}()
	for {
		select {
		case s := <-r.out:
			if s.session == sessionID && s.env.Type == typ {
				return s.env
			}
			skipped = append(skipped, s)
		case <-deadline:
			r.t.Fatalf("no %s to %s", typ, sessionID)
			return protocol.Envelope{}
		}
	}
}

// none checks that nothing of type typ went to sessionID so far.
func (r *projectorRig) none(sessionID string, typ protocol.Type) {
	r.t.Helper()
	var keep []sent
	for {
		select {
		case s := <-r.out:
			if s.session == sessionID && s.env.Type == typ {
				r.t.Fatalf("unexpected %s to %s", typ, sessionID)
			}
			keep = append(keep, s)
			continue
		default:
		}
		break
	}
	for _, s := range keep {
		r.out <- s
	}
}

func (r *projectorRig) handle(sessionID string, m protocol.Payload) {
	r.p.Handle(sessionID, roundTrip(r.t, m))
}

func (r *projectorRig) lastStatus() ProjectStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.statuses) == 0 {
		return ProjectStatus{}
	}
	return r.statuses[len(r.statuses)-1]
}

func student(id, host string, caps ...protocol.Capability) session.Session {
	s := agent(id, caps...)
	s.Hostname = host
	return s
}

var (
	viewCaps   = []protocol.Capability{protocol.CapScreenBroadcast, protocol.CapScreenPresentView}
	sourceCaps = []protocol.Capability{protocol.CapScreenBroadcast, protocol.CapScreenPresentView, protocol.CapScreenPresent}
)

func classroom() (source session.Session, all []session.Session) {
	source = student("s", "LAB3-PC07", sourceCaps...)
	return source, []session.Session{source, student("a", "LAB3-PC01", viewCaps...), student("b", "LAB3-PC02", viewCaps...),
		student("old", "LAB3-PC03", protocol.CapScreenBroadcast), student("c", "LAB3-PC04")}
}

// accept plays the source: the band is up and the capture ready.
func (r *projectorRig) accept(t *testing.T) protocol.Envelope {
	t.Helper()
	start := r.expect("s", protocol.TypeStudentPresentStart)
	id := start.Payload.(protocol.StudentPresentStart).PresentID
	r.handle("s", protocol.StudentPresentAck{RefID: start.ID, PresentID: id, Status: protocol.PresentAccepted, Width: 1280, Height: 720})
	return start
}

func TestProjectorSignaling(t *testing.T) {
	r := newProjectorRig(t)
	src, class := classroom()
	if err := r.p.Start(student("old", "PC", protocol.CapScreenBroadcast), class); !errors.Is(err, ErrPresentUnsupported) {
		t.Fatalf("start without screen.present: %v", err)
	}
	if err := r.p.Start(src, class); err != nil {
		t.Fatal(err)
	}
	if !r.relay.openOpts.Relay || !r.relay.openOpts.Loopback {
		t.Fatalf("media options %+v", r.relay.openOpts)
	}
	start := r.expect("s", protocol.TypeStudentPresentStart)
	ps := start.Payload.(protocol.StudentPresentStart)
	if ps.FPS != DefaultFPS || ps.MaxWidth != DefaultMaxWidth || ps.MaxKbps != DefaultMaxKbps {
		t.Fatalf("start %+v", ps)
	}
	waitFor(t, time.Second, "connecting", func() bool {
		s := r.p.Status()
		return s.Active && s.Phase == ProjectConnecting && s.Hostname == "LAB3-PC07"
	})
	// The classmates get nothing until the source's band is up (its ack).
	r.none("a", protocol.TypeScreenBroadcastStart)

	r.handle("s", protocol.StudentPresentAck{RefID: start.ID, PresentID: ps.PresentID, Status: protocol.PresentAccepted, Width: 1280, Height: 720})
	// The receivers: every other student with screen.present.view, never
	// the source nor an older agent.
	for _, id := range []string{"a", "b"} {
		e := r.expect(id, protocol.TypeScreenBroadcastStart)
		bs := e.Payload.(protocol.ScreenBroadcastStart)
		if bs.Source == nil || bs.Source.Hostname != "LAB3-PC07" || bs.Width != 1280 || bs.Height != 720 {
			t.Fatalf("broadcast start to %s: %+v", id, bs)
		}
	}
	r.none("s", protocol.TypeScreenBroadcastStart)
	r.none("old", protocol.TypeScreenBroadcastStart)
	r.none("c", protocol.TypeScreenBroadcastStart)

	// The source leg: a receive-only offer, the answer, the candidates.
	offer := r.expect("s", protocol.TypeStudentPresentOffer)
	po := offer.Payload.(protocol.StudentPresentOffer)
	if po.Attempt != 1 || checkSDP(po.SDP, "recvonly") != nil {
		t.Fatalf("offer %+v", po)
	}
	r.handle("s", protocol.StudentPresentAnswer{RefID: offer.ID, PresentID: ps.PresentID, Attempt: 1, SDP: sdpWith("sendonly")})
	r.handle("s", protocol.StudentPresentICE{PresentID: ps.PresentID, Attempt: 1, Candidate: "candidate:1 1 udp 1 192.168.1.27 5000 typ host", SDPMid: "0"})
	waitFor(t, time.Second, "source answer applied", func() bool {
		a, rem, _ := r.relay.source(0).snapshot()
		return a != "" && len(rem) == 1
	})
	srcPeer := r.relay.source(0)
	srcPeer.ev.OnState(PeerConnected)
	waitFor(t, time.Second, "live", func() bool { return r.p.Status().Phase == ProjectLive })
	if srcPeer.keyframes() == 0 {
		t.Error("no keyframe asked when the source connected")
	}

	// The receivers answer and connect: two watching.
	for _, id := range []string{"a", "b"} {
		e := r.expect(id, protocol.TypeScreenBroadcastOffer)
		o := e.Payload.(protocol.ScreenBroadcastOffer)
		r.handle(id, protocol.ScreenBroadcastAnswer{RefID: e.ID, BroadcastID: o.BroadcastID, Attempt: o.Attempt, SDP: answerSDP})
	}
	r.relay.mu.Lock()
	for _, p := range r.relay.peers {
		p.ev.OnState(PeerConnected)
	}
	r.relay.mu.Unlock()
	waitFor(t, time.Second, "two receivers", func() bool { s := r.p.Status(); return s.Receivers == 2 && s.Eligible == 2 })

	// The source's connection fails: a new offer to the source only; the
	// receivers keep theirs.
	srcPeer.ev.OnState(PeerFailed)
	offer2 := r.expect("s", protocol.TypeStudentPresentOffer)
	if a := offer2.Payload.(protocol.StudentPresentOffer).Attempt; a != 2 {
		t.Fatalf("retry attempt %d", a)
	}
	r.none("a", protocol.TypeScreenBroadcastStart)
	if _, _, closed := srcPeer.snapshot(); !closed {
		t.Fatal("the failed source connection stayed open")
	}

	// A student joins and receives; one leaves.
	r.p.Sync(append(class, student("d", "LAB3-PC05", viewCaps...)))
	r.expect("d", protocol.TypeScreenBroadcastStart)

	// Stop: the source and every receiver are told; the relay closes.
	r.p.Stop()
	r.expect("s", protocol.TypeStudentPresentStop)
	for _, id := range []string{"a", "b", "d"} {
		r.expect(id, protocol.TypeScreenBroadcastStop)
	}
	if !r.relay.isClosed() {
		t.Fatal("relay not closed")
	}
	if s := r.p.Status(); s.Active {
		t.Fatalf("status after stop %+v", s)
	}
	if s := r.lastStatus(); s.Ended != ProjectStopped || s.Hostname != "LAB3-PC07" {
		t.Fatalf("final status %+v", s)
	}
}

func TestProjectorEndsWhenTheSourceDoes(t *testing.T) {
	src, class := classroom()
	for _, tt := range []struct {
		name    string
		live    bool
		status  protocol.PresentStatus
		reason  protocol.PresentReason
		outcome string
	}{
		{"failed before", false, protocol.PresentFailed, protocol.PresentCaptureUnavailable, ProjectFailed},
		{"failed while live", true, protocol.PresentFailed, protocol.PresentEncoderUnavailable, ProjectFailed},
		{"ended by the agent", true, protocol.PresentEnded, "", ProjectStopped},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newProjectorRig(t)
			if err := r.p.Start(src, class); err != nil {
				t.Fatal(err)
			}
			var start protocol.Envelope
			if tt.live {
				start = r.accept(t)
				r.expect("a", protocol.TypeScreenBroadcastStart)
			} else {
				start = r.expect("s", protocol.TypeStudentPresentStart)
			}
			id := start.Payload.(protocol.StudentPresentStart).PresentID
			r.handle("s", protocol.StudentPresentAck{RefID: start.ID, PresentID: id, Status: tt.status, Reason: tt.reason})
			waitFor(t, time.Second, "ended", func() bool { return r.lastStatus().Ended == tt.outcome })
			if s := r.lastStatus(); s.Reason != string(tt.reason) || s.Active {
				t.Fatalf("status %+v", s)
			}
			if tt.live {
				r.expect("a", protocol.TypeScreenBroadcastStop)
				r.expect("b", protocol.TypeScreenBroadcastStop)
			}
			r.none("s", protocol.TypeStudentPresentStop) // the source ended it itself
			if !r.relay.isClosed() {
				t.Fatal("relay not closed")
			}
		})
	}

	t.Run("source left", func(t *testing.T) {
		r := newProjectorRig(t)
		if err := r.p.Start(src, class); err != nil {
			t.Fatal(err)
		}
		r.accept(t)
		r.expect("a", protocol.TypeScreenBroadcastStart)
		r.p.Sync(class[1:]) // the source's session closed
		r.expect("a", protocol.TypeScreenBroadcastStop)
		r.expect("b", protocol.TypeScreenBroadcastStop)
		waitFor(t, time.Second, "ended", func() bool { return r.lastStatus().Ended == ProjectSourceLeft })
		r.none("s", protocol.TypeStudentPresentStop)
	})

	t.Run("no answer from the agent", func(t *testing.T) {
		r := newProjectorRig(t)
		r.p.StartWait = 30 * time.Millisecond
		if err := r.p.Start(src, class); err != nil {
			t.Fatal(err)
		}
		r.expect("s", protocol.TypeStudentPresentStart)
		r.expect("s", protocol.TypeStudentPresentStop)
		waitFor(t, time.Second, "ended", func() bool {
			s := r.lastStatus()
			return s.Ended == ProjectFailed && s.Reason == ReasonNoAnswer
		})
	})
}

// Only one source at a time: starting another student ends the first one in
// every computer before the second starts.
func TestProjectorOneSourceAtATime(t *testing.T) {
	r := newProjectorRig(t)
	src, class := classroom()
	if err := r.p.Start(src, class); err != nil {
		t.Fatal(err)
	}
	r.accept(t)
	r.expect("a", protocol.TypeScreenBroadcastStart)
	second := student("a", "LAB3-PC01", sourceCaps...)
	class[1] = second
	if err := r.p.Start(second, class); err != nil {
		t.Fatal(err)
	}
	r.expect("s", protocol.TypeStudentPresentStop)
	r.expect("a", protocol.TypeScreenBroadcastStop)
	r.expect("b", protocol.TypeScreenBroadcastStop)
	r.expect("a", protocol.TypeStudentPresentStart)
	if s := r.p.Status(); !s.Active || s.SourceID != "a" || s.Phase != ProjectConnecting {
		t.Fatalf("status %+v", s)
	}
}

func TestProjectorConsoleView(t *testing.T) {
	r := newProjectorRig(t)
	src, class := classroom()
	if err := r.p.Start(src, class); err != nil {
		t.Fatal(err)
	}
	r.p.OpenView()
	r.accept(t)
	waitFor(t, time.Second, "console view started", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return len(r.views) > 0 && r.views[0].Active && r.views[0].Source == "LAB3-PC07" && len(r.signals) > 0
	})
	r.mu.Lock()
	sig := r.signals[0]
	r.mu.Unlock()
	if sig.Kind != SignalOffer {
		t.Fatalf("signal %+v", sig)
	}
	if err := r.p.ViewAnswer(sig.BroadcastID, sig.Attempt, answerSDP); err != nil {
		t.Fatal(err)
	}
	if err := r.p.ViewAnswer(sig.BroadcastID, sig.Attempt+5, answerSDP); !errors.Is(err, ErrNoBroadcast) {
		t.Fatalf("answer of an unknown attempt: %v", err)
	}
	// Every receiver connects; the console's view is not counted.
	r.expect("a", protocol.TypeScreenBroadcastOffer)
	r.expect("b", protocol.TypeScreenBroadcastOffer)
	waitFor(t, time.Second, "three connections", func() bool {
		r.relay.mu.Lock()
		defer r.relay.mu.Unlock()
		return len(r.relay.peers) == 3
	})
	r.relay.mu.Lock()
	for _, p := range r.relay.peers {
		p.ev.OnState(PeerConnected)
	}
	r.relay.mu.Unlock()
	waitFor(t, time.Second, "counts", func() bool { s := r.p.Status(); return s.Receivers == 2 && s.Eligible == 2 })

	r.p.CloseView()
	r.mu.Lock()
	last := r.views[len(r.views)-1]
	r.mu.Unlock()
	if last.Active {
		t.Fatal("CloseView did not hide the view")
	}
	r.p.Stop()
}
