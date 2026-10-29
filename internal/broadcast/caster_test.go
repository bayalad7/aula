package broadcast

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// fakePeer is the PeerConnection double: it records what the caster does
// with it and lets the test fire its events.
type fakePeer struct {
	ev PeerEvents
	n  int // creation order

	mu     sync.Mutex
	answer string
	remote []Candidate
	closed bool
}

func (p *fakePeer) Offer() (string, error) {
	// Like pion, the local candidates are gathered once the offer is set.
	go p.ev.OnCandidate(Candidate{Candidate: fmt.Sprintf("candidate:%d 1 udp 1 192.168.1.20 47831 typ host", p.n), SDPMid: "0"})
	return offerSDP, nil
}

func (p *fakePeer) SetAnswer(sdp string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.answer = sdp
	return nil
}

func (p *fakePeer) AddCandidate(c Candidate) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.remote = append(p.remote, c)
	return nil
}

func (p *fakePeer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

func (p *fakePeer) snapshot() (answer string, remote []Candidate, closed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.answer, append([]Candidate(nil), p.remote...), p.closed
}

type sent struct {
	session string
	env     protocol.Envelope
}

// casterHarness runs a Caster against fake peers and a fake connection that
// round-trips every message through the protocol encoder.
type casterHarness struct {
	t      *testing.T
	c      *Caster
	out    chan sent
	cancel context.CancelFunc

	mu        sync.Mutex
	peers     []*fakePeer
	statuses  []Status
	keyframes int
}

func newCasterHarness(t *testing.T, tweak func(*Caster)) *casterHarness {
	h := &casterHarness{t: t, out: make(chan sent, 256)}
	cfg := Caster{
		BroadcastID: testID,
		Start:       protocol.ScreenBroadcastStart{Width: 1920, Height: 1080, FPS: 15},
		NewPeer: func(ev PeerEvents) (Peer, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			p := &fakePeer{ev: ev, n: len(h.peers) + 1}
			h.peers = append(h.peers, p)
			return p, nil
		},
		Send: func(_ context.Context, sessionID string, p protocol.Payload) (string, error) {
			e := protocol.New(p)
			data, err := protocol.Encode(e)
			if err != nil {
				return "", err
			}
			back, err := protocol.Decode(data)
			if err != nil {
				return "", err
			}
			h.out <- sent{sessionID, back}
			return e.ID, nil
		},
		OnStatus: func(s Status) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.statuses = append(h.statuses, s)
		},
		OnKeyframe: func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.keyframes++
		},
		Backoff:         func(int) time.Duration { return 10 * time.Millisecond },
		DisconnectGrace: 50 * time.Millisecond,
		ConnectTimeout:  time.Minute,
	}
	if tweak != nil {
		tweak(&cfg)
	}
	h.c = NewCaster(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go h.c.Run(ctx)
	t.Cleanup(func() {
		cancel()
		h.c.Wait()
	})
	return h
}

// next returns the next message sent, which must be of type typ.
func (h *casterHarness) next(typ protocol.Type) sent {
	h.t.Helper()
	select {
	case s := <-h.out:
		if s.env.Type != typ {
			h.t.Fatalf("sent %s to %s, want %s", s.env.Type, s.session, typ)
		}
		return s
	case <-time.After(2 * time.Second):
		h.t.Fatalf("nothing sent, want %s", typ)
	}
	return sent{}
}

func (h *casterHarness) quiet() {
	h.t.Helper()
	select {
	case s := <-h.out:
		h.t.Fatalf("unexpected %s to %s", s.env.Type, s.session)
	case <-time.After(100 * time.Millisecond):
	}
}

func (h *casterHarness) peer(n int) *fakePeer {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.peers[n-1]
}

func (h *casterHarness) agentSays(sessionID string, p protocol.Payload) protocol.Envelope {
	e := protocol.New(p)
	h.c.Handle(sessionID, e)
	return e
}

// offerTo walks one session through start, offer and the console's first
// candidate, and returns the offer envelope.
func (h *casterHarness) offerTo(sessionID string, attempt int) protocol.Envelope {
	h.t.Helper()
	o := h.next(protocol.TypeScreenBroadcastOffer)
	if o.session != sessionID {
		h.t.Fatalf("offer went to %s, want %s", o.session, sessionID)
	}
	if p := o.env.Payload.(protocol.ScreenBroadcastOffer); p.Attempt != attempt || p.BroadcastID != testID {
		h.t.Fatalf("offer %+v, want attempt %d", p, attempt)
	}
	ice := h.next(protocol.TypeScreenBroadcastICE)
	if p := ice.env.Payload.(protocol.ScreenBroadcastICE); ice.session != sessionID || p.Attempt != attempt {
		h.t.Fatalf("candidate %+v to %s, want attempt %d to %s", p, ice.session, attempt, sessionID)
	}
	return o.env
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func agent(id string, caps ...protocol.Capability) session.Session {
	return session.Session{ID: id, State: session.StateConnected, Capabilities: caps}
}

func TestCasterSignaling(t *testing.T) {
	h := newCasterHarness(t, nil)
	h.c.Sync([]session.Session{
		agent("a", protocol.CapCodeView, protocol.CapScreenBroadcast),
		agent("old", protocol.CapCodeView), // an agent without the capability gets nothing
	})

	start := h.next(protocol.TypeScreenBroadcastStart)
	if p := start.env.Payload.(protocol.ScreenBroadcastStart); start.session != "a" || p.BroadcastID != testID || p.Width != 1920 {
		t.Fatalf("start %+v to %s", p, start.session)
	}
	offer := h.offerTo("a", 1)

	// The agent's candidates reach the connection; mDNS names and the end
	// marker are skipped.
	h.agentSays("a", protocol.ScreenBroadcastICE{BroadcastID: testID, Attempt: 1, Candidate: "candidate:9 1 udp 1 192.168.1.30 50000 typ host", SDPMid: "0"})
	h.agentSays("a", protocol.ScreenBroadcastICE{BroadcastID: testID, Attempt: 1, Candidate: "candidate:8 1 udp 1 4f1c.local 50001 typ host", SDPMid: "0"})
	h.agentSays("a", protocol.ScreenBroadcastICE{BroadcastID: testID, Attempt: 1, SDPMid: "0"})
	// An answer that does not reference the offer is ignored; the right one
	// is applied.
	h.agentSays("a", protocol.ScreenBroadcastAnswer{RefID: otherID, BroadcastID: testID, Attempt: 1, SDP: answerSDP})
	h.agentSays("a", protocol.ScreenBroadcastAnswer{RefID: offer.ID, BroadcastID: testID, Attempt: 1, SDP: answerSDP})
	p1 := h.peer(1)
	eventually(t, "answer applied", func() bool { a, _, _ := p1.snapshot(); return a == answerSDP })
	_, remote, _ := p1.snapshot()
	if len(remote) != 1 || remote[0].Candidate != "candidate:9 1 udp 1 192.168.1.30 50000 typ host" {
		t.Fatalf("remote candidates %+v", remote)
	}

	// Connected: the count goes up and a keyframe is requested for the
	// newcomer.
	p1.ev.OnState(PeerConnected)
	eventually(t, "receiver counted", func() bool { s := h.c.Status(); return s.Receivers == 1 && s.Eligible == 1 })
	h.mu.Lock()
	kf := h.keyframes
	h.mu.Unlock()
	if kf != 1 {
		t.Fatalf("%d keyframe requests on connect, want 1", kf)
	}
	h.quiet()

	// Messages of another broadcast are not ours.
	if h.c.Handle("a", protocol.New(protocol.ScreenBroadcastAnswer{RefID: offer.ID, BroadcastID: otherID, Attempt: 1, SDP: answerSDP})) {
		t.Fatal("handled an answer of another broadcast")
	}

	// Stopping sends stop to every receiver and closes the connections.
	h.cancel()
	stop := h.next(protocol.TypeScreenBroadcastStop)
	if stop.session != "a" || stop.env.Payload.(protocol.ScreenBroadcastStop).BroadcastID != testID {
		t.Fatalf("stop %+v", stop)
	}
	h.c.Wait()
	if _, _, closed := p1.snapshot(); !closed {
		t.Fatal("connection still open after stop")
	}
	h.mu.Lock()
	last := h.statuses[len(h.statuses)-1]
	h.mu.Unlock()
	if last.Active {
		t.Fatalf("last status %+v, want inactive", last)
	}
}

func TestCasterRetriesAFailedConnection(t *testing.T) {
	h := newCasterHarness(t, nil)
	h.c.Sync([]session.Session{agent("a", protocol.CapScreenBroadcast)})
	h.next(protocol.TypeScreenBroadcastStart)
	offer1 := h.offerTo("a", 1)
	h.agentSays("a", protocol.ScreenBroadcastAnswer{RefID: offer1.ID, BroadcastID: testID, Attempt: 1, SDP: answerSDP})
	p1 := h.peer(1)
	p1.ev.OnState(PeerConnected)

	// The connection fails: after the backoff a new offer (attempt 2) goes
	// out on a new connection, without a new start.
	p1.ev.OnState(PeerFailed)
	offer2 := h.offerTo("a", 2)
	if _, _, closed := p1.snapshot(); !closed {
		t.Fatal("failed connection not closed")
	}
	// The late answer to attempt 1 does not touch the new connection.
	h.agentSays("a", protocol.ScreenBroadcastAnswer{RefID: offer1.ID, BroadcastID: testID, Attempt: 1, SDP: answerSDP})
	h.agentSays("a", protocol.ScreenBroadcastAnswer{RefID: offer2.ID, BroadcastID: testID, Attempt: 2, SDP: answerSDP})
	p2 := h.peer(2)
	eventually(t, "answer 2 applied", func() bool { a, _, _ := p2.snapshot(); return a != "" })
	// Events of the old connection are ignored.
	p1.ev.OnState(PeerConnected)
	if s := h.c.Status(); s.Receivers != 0 {
		t.Fatalf("an old connection was counted: %+v", s)
	}
	p2.ev.OnState(PeerConnected)
	eventually(t, "reconnected", func() bool { return h.c.Status().Receivers == 1 })

	// Disconnected for longer than the grace period: attempt 3.
	p2.ev.OnState(PeerDisconnected)
	h.offerTo("a", 3)
}

func TestCasterRecoversFromAgentErrorsAndTimeouts(t *testing.T) {
	h := newCasterHarness(t, func(c *Caster) { c.ConnectTimeout = 80 * time.Millisecond })
	h.c.Sync([]session.Session{agent("a", protocol.CapScreenBroadcast)})
	h.next(protocol.TypeScreenBroadcastStart)
	offer1 := h.offerTo("a", 1)
	// The agent's frontend failed to open the connection for this offer.
	h.agentSays("a", protocol.NewError(offer1.ID, protocol.CodeInternal, "no se pudo abrir la conexión de video"))
	h.offerTo("a", 2)
	// Nobody answers attempt 2: it expires and attempt 3 follows.
	h.offerTo("a", 3)
	// An unrelated error changes nothing.
	h.agentSays("a", protocol.NewError(otherID, protocol.CodeInternal, "otro"))
	if s := h.c.Status(); s.Eligible != 1 || s.Receivers != 0 {
		t.Fatalf("status %+v", s)
	}
}

func TestCasterFollowsTheClass(t *testing.T) {
	h := newCasterHarness(t, nil)
	h.c.Sync([]session.Session{agent("a", protocol.CapScreenBroadcast), agent("b", protocol.CapScreenBroadcast)})
	// Per session, in order: start, offer, candidate. Sessions interleave.
	seq := map[string][]protocol.Type{}
	for range 6 {
		select {
		case m := <-h.out:
			seq[m.session] = append(seq[m.session], m.env.Type)
		case <-time.After(2 * time.Second):
			t.Fatalf("messages so far: %v", seq)
		}
	}
	want := fmt.Sprint([]protocol.Type{protocol.TypeScreenBroadcastStart, protocol.TypeScreenBroadcastOffer, protocol.TypeScreenBroadcastICE})
	if fmt.Sprint(seq["a"]) != want || fmt.Sprint(seq["b"]) != want {
		t.Fatalf("messages %v", seq)
	}
	// Syncing again with nothing new: nothing is sent.
	h.c.Sync([]session.Session{agent("a", protocol.CapScreenBroadcast), agent("b", protocol.CapScreenBroadcast)})
	h.quiet()

	// b drops (its session disconnects): its connection closes and nothing
	// is sent to it. It comes back as a new session: start and attempt 1.
	h.c.Sync([]session.Session{agent("a", protocol.CapScreenBroadcast), {ID: "b", State: session.StateDisconnected}})
	eventually(t, "b dropped", func() bool { return h.c.Status().Eligible == 1 })
	closed := 0
	for i := 1; i <= 2; i++ {
		if _, _, c := h.peer(i).snapshot(); c {
			closed++
		}
	}
	if closed != 1 {
		t.Fatalf("%d connections closed, want 1", closed)
	}
	h.quiet()
	h.c.Sync([]session.Session{agent("a", protocol.CapScreenBroadcast), agent("b2", protocol.CapScreenBroadcast)})
	if s := h.next(protocol.TypeScreenBroadcastStart); s.session != "b2" {
		t.Fatalf("start to %s, want b2", s.session)
	}
	h.offerTo("b2", 1)
}

func TestCasterGivesUpQuietlyWhenSendFails(t *testing.T) {
	var mu sync.Mutex
	tries := 0
	h := newCasterHarness(t, func(c *Caster) {
		c.Send = func(context.Context, string, protocol.Payload) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			tries++
			return "", errors.New("connection closed")
		}
	})
	h.c.Sync([]session.Session{agent("a", protocol.CapScreenBroadcast)})
	// The start keeps being retried with backoff; the session will soon be
	// dropped by Sync when the registry notices the connection is gone.
	eventually(t, "retries", func() bool { mu.Lock(); defer mu.Unlock(); return tries >= 3 })
	h.c.Sync(nil)
	eventually(t, "dropped", func() bool { return h.c.Status().Eligible == 0 })
}
