package broadcast

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// Candidate is one ICE candidate; an empty Candidate ends them.
type Candidate struct {
	Candidate     string
	SDPMid        string
	SDPMLineIndex int
}

// PeerState is the state of one WebRTC connection.
type PeerState int

// Peer states, as pion's PeerConnectionState.
const (
	PeerConnecting PeerState = iota + 1
	PeerConnected
	PeerDisconnected
	PeerFailed
	PeerClosed
)

// PeerEvents are the callbacks of one Peer. They may run on any goroutine.
type PeerEvents struct {
	OnCandidate func(Candidate) // a local candidate to trickle
	OnState     func(PeerState)
	OnKeyframe  func() // the receiver asked for a keyframe (PLI/FIR)
}

// Peer is one WebRTC connection from the console to an agent, sending the
// shared video track. pionPeer implements it; tests use a double.
type Peer interface {
	// Offer creates the SDP offer and sets it as the local description.
	Offer() (string, error)
	SetAnswer(sdp string) error
	AddCandidate(Candidate) error
	Close() error
}

// PeerFactory opens a new Peer wired to ev.
type PeerFactory func(ev PeerEvents) (Peer, error)

// SendFunc delivers p to one session and returns the envelope ID.
type SendFunc func(ctx context.Context, sessionID string, p protocol.Payload) (string, error)

// Status is the broadcast as the console shows it.
type Status struct {
	Active      bool   `json:"active"`
	BroadcastID string `json:"broadcastId"`
	Receivers   int    `json:"receivers"` // connections in PeerConnected
	Eligible    int    `json:"eligible"`  // connected agents with screen.broadcast
}

// Timing of the retries (docs/protocol.md §5.12).
const (
	DefaultDisconnectGrace = 5 * time.Second
	// DefaultConnectTimeout bounds an attempt, from the offer to connected.
	DefaultConnectTimeout = 15 * time.Second
	sendTimeout           = 5 * time.Second
)

// Caster is the console's signaling for one broadcast: it sends start and an
// offer to every connected agent with screen.broadcast, applies answers and
// candidates, and starts a new attempt when a connection fails, stays
// disconnected or never connects. All its state lives on one goroutine (Run).
type Caster struct {
	BroadcastID string
	Start       protocol.ScreenBroadcastStart // BroadcastID is filled in
	NewPeer     PeerFactory
	Send        SendFunc
	// OnStatus is called on the Run goroutine when the counts change.
	OnStatus func(Status)
	// OnKeyframe is called when a receiver asks for a keyframe or connects.
	OnKeyframe func()
	// Backoff gives the wait before attempt n+1 after n failures in a row;
	// nil means 1, 2, 4… 30 s.
	Backoff         func(failures int) time.Duration
	DisconnectGrace time.Duration // 0 means DefaultDisconnectGrace
	ConnectTimeout  time.Duration // 0 means DefaultConnectTimeout
	Logger          *slog.Logger

	events  chan func()
	done    chan struct{}
	peers   map[string]*receiver // session ID → its connection
	last    Status
	stopped bool
}

// receiver is the connection to one agent. Only touched on the Run goroutine.
type receiver struct {
	sessionID string
	attempt   int
	offerID   string
	peer      Peer
	state     PeerState
	failures  int
	started   bool // screen.broadcast.start sent
	remote    int  // candidates received in this attempt
	timer     *time.Timer
}

// NewCaster returns a caster ready to Run.
func NewCaster(c Caster) *Caster {
	c.Start.BroadcastID = c.BroadcastID
	c.events = make(chan func(), 64)
	c.done = make(chan struct{})
	c.peers = map[string]*receiver{}
	return &c
}

// Run processes the caster's events until ctx is canceled; then it sends
// screen.broadcast.stop to every agent and closes every connection.
func (c *Caster) Run(ctx context.Context) {
	defer close(c.done)
	c.report()
	for {
		select {
		case <-ctx.Done():
			c.shutdown()
			return
		case f := <-c.events:
			f()
			c.report()
		}
	}
}

// post runs f on the Run goroutine; it is dropped once Run has returned.
func (c *Caster) post(f func()) {
	select {
	case c.events <- f:
	case <-c.done:
	}
}

// Sync starts the broadcast for the sessions that can receive it and have no
// connection yet, and drops those that left.
func (c *Caster) Sync(sessions []session.Session) {
	c.post(func() {
		live := map[string]bool{}
		for _, s := range sessions {
			if s.State != session.StateConnected || !s.Can(protocol.CapScreenBroadcast) {
				continue
			}
			live[s.ID] = true
			if _, ok := c.peers[s.ID]; !ok {
				c.join(s.ID)
			}
		}
		for id, r := range c.peers {
			if !live[id] {
				c.drop(r)
				delete(c.peers, id)
				c.log().Info("broadcast receiver left", "conn", id)
			}
		}
	})
}

// Handle takes the agent's answers, candidates and errors for this
// broadcast. It reports whether the message belonged to the broadcast
// (errors are acted upon but always reported false).
func (c *Caster) Handle(sessionID string, env protocol.Envelope) bool {
	switch p := env.Payload.(type) {
	case protocol.ScreenBroadcastAnswer:
		if p.BroadcastID != c.BroadcastID {
			return false
		}
		c.post(func() { c.answer(sessionID, p) })
	case protocol.ScreenBroadcastICE:
		if p.BroadcastID != c.BroadcastID {
			return false
		}
		c.post(func() { c.remoteCandidate(sessionID, p) })
	case protocol.ErrorPayload:
		// Only an error about the offer in force matters; the caller may log
		// errors as usual, so this one is not reported as handled.
		c.post(func() {
			if r := c.peers[sessionID]; r != nil && r.offerID != "" && p.RefID == r.offerID {
				c.log().Info("agent could not open the broadcast", "conn", sessionID, "attempt", r.attempt, "code", p.Code)
				c.retry(r)
			}
		})
		return false
	default:
		return false
	}
	return true
}

// Status returns the counts as of the last event.
func (c *Caster) Status() Status {
	res := make(chan Status, 1)
	c.post(func() { res <- c.status() })
	select {
	case s := <-res:
		return s
	case <-c.done:
		return Status{}
	}
}

// Wait blocks until Run has returned.
func (c *Caster) Wait() { <-c.done }

// --- Run goroutine ---------------------------------------------------------------

func (c *Caster) join(sessionID string) {
	r := &receiver{sessionID: sessionID}
	c.peers[sessionID] = r
	c.begin(r)
}

// begin sends start the first time (the agent shows its notice) and then
// the offer of the next attempt.
func (c *Caster) begin(r *receiver) {
	if !r.started {
		if _, err := c.send(r.sessionID, c.Start); err != nil {
			c.log().Debug("broadcast start not sent", "conn", r.sessionID, "err", err)
			c.retry(r)
			return
		}
		r.started = true
		c.log().Info("broadcast receiver joined", "conn", r.sessionID)
	}
	c.offer(r)
}

// offer opens a new connection for the next attempt and sends its offer.
func (c *Caster) offer(r *receiver) {
	c.closePeer(r)
	if r.attempt >= protocol.MaxBroadcastAttempt {
		c.log().Warn("broadcast receiver gave up", "conn", r.sessionID)
		return
	}
	r.attempt++
	r.offerID, r.state, r.remote = "", PeerConnecting, 0
	attempt, id := r.attempt, r.sessionID
	// Events of a closed connection or an old attempt are ignored. They are
	// posted, so they run after this function has set r.peer.
	var self Peer
	current := func() *receiver {
		if cur := c.peers[id]; cur == r && r.attempt == attempt && r.peer == self && self != nil {
			return r
		}
		return nil
	}
	peer, err := c.NewPeer(PeerEvents{
		OnCandidate: func(cand Candidate) {
			c.post(func() {
				if r := current(); r != nil {
					c.localCandidate(r, cand)
				}
			})
		},
		OnState: func(s PeerState) {
			c.post(func() {
				if r := current(); r != nil {
					c.peerState(r, s)
				}
			})
		},
		OnKeyframe: c.keyframe,
	})
	if err != nil {
		c.log().Warn("broadcast connection not created", "conn", id, "err", err)
		c.retry(r)
		return
	}
	r.peer, self = peer, peer
	sdp, err := peer.Offer()
	if err != nil {
		c.log().Warn("broadcast offer not created", "conn", id, "err", err)
		c.retry(r)
		return
	}
	envID, err := c.send(id, protocol.ScreenBroadcastOffer{BroadcastID: c.BroadcastID, Attempt: attempt, SDP: sdp})
	if err != nil {
		c.log().Debug("broadcast offer not sent", "conn", id, "err", err)
		c.retry(r)
		return
	}
	r.offerID = envID
	c.arm(r, c.connectTimeout(), "connect timeout")
}

func (c *Caster) answer(sessionID string, p protocol.ScreenBroadcastAnswer) {
	r := c.peers[sessionID]
	if r == nil || r.peer == nil || p.Attempt != r.attempt || p.RefID != r.offerID {
		return // an answer to an old offer
	}
	if err := r.peer.SetAnswer(p.SDP); err != nil {
		c.log().Info("broadcast answer refused", "conn", sessionID, "attempt", p.Attempt, "err", err)
		c.retry(r)
	}
}

func (c *Caster) remoteCandidate(sessionID string, p protocol.ScreenBroadcastICE) {
	r := c.peers[sessionID]
	if r == nil || r.peer == nil || p.Attempt != r.attempt || r.remote >= protocol.MaxICECandidates {
		return
	}
	r.remote++
	if p.Candidate == "" || strings.Contains(p.Candidate, ".local ") {
		// End of candidates, or an mDNS name: the ICE-lite side learns the
		// agent's address from its connectivity checks anyway.
		return
	}
	if err := r.peer.AddCandidate(Candidate{Candidate: p.Candidate, SDPMid: p.SDPMid, SDPMLineIndex: p.SDPMLineIndex}); err != nil {
		c.log().Debug("broadcast candidate refused", "conn", sessionID, "err", err)
	}
}

func (c *Caster) localCandidate(r *receiver, cand Candidate) {
	if r.offerID == "" {
		return // the offer was not sent; this attempt is being retried
	}
	_, err := c.send(r.sessionID, protocol.ScreenBroadcastICE{BroadcastID: c.BroadcastID, Attempt: r.attempt,
		Candidate: cand.Candidate, SDPMid: cand.SDPMid, SDPMLineIndex: cand.SDPMLineIndex})
	if err != nil {
		c.log().Debug("broadcast candidate not sent", "conn", r.sessionID, "err", err)
	}
}

func (c *Caster) peerState(r *receiver, s PeerState) {
	prev := r.state
	r.state = s
	switch s {
	case PeerConnected:
		r.failures = 0
		c.disarm(r)
		if prev != PeerConnected {
			c.log().Info("broadcast receiver connected", "conn", r.sessionID, "attempt", r.attempt)
			c.keyframe()
		}
	case PeerDisconnected:
		c.arm(r, c.disconnectGrace(), "disconnected")
	case PeerFailed:
		c.log().Info("broadcast connection failed", "conn", r.sessionID, "attempt", r.attempt)
		c.retry(r)
	}
}

// retry schedules a new attempt after the backoff.
func (c *Caster) retry(r *receiver) {
	c.closePeer(r)
	wait := c.backoff(r.failures)
	r.failures++
	c.arm(r, wait, "")
}

// arm (re)starts the receiver's timer. With a reason, it fires a retry if the
// connection is still not up; without one, it starts the next attempt.
func (c *Caster) arm(r *receiver, after time.Duration, reason string) {
	c.disarm(r)
	attempt := r.attempt
	r.timer = time.AfterFunc(after, func() {
		c.post(func() {
			if c.peers[r.sessionID] != r || r.attempt != attempt {
				return
			}
			if reason == "" {
				c.begin(r)
				return
			}
			if r.state != PeerConnected {
				c.log().Info("broadcast attempt expired", "conn", r.sessionID, "attempt", r.attempt, "why", reason)
				c.retry(r)
			}
		})
	})
}

func (c *Caster) disarm(r *receiver) {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

func (c *Caster) closePeer(r *receiver) {
	if r.peer != nil {
		if err := r.peer.Close(); err != nil {
			c.log().Debug("broadcast connection close", "conn", r.sessionID, "err", err)
		}
		r.peer = nil
	}
	r.state = PeerClosed
}

// drop forgets a receiver whose session is gone; nothing is sent to it.
func (c *Caster) drop(r *receiver) {
	c.disarm(r)
	c.closePeer(r)
}

func (c *Caster) shutdown() {
	for id, r := range c.peers {
		c.drop(r)
		if _, err := c.send(id, protocol.ScreenBroadcastStop{BroadcastID: c.BroadcastID}); err != nil {
			c.log().Debug("broadcast stop not sent", "conn", id, "err", err)
		}
	}
	clear(c.peers)
	c.stopped = true
	c.report()
}

func (c *Caster) send(sessionID string, p protocol.Payload) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	return c.Send(ctx, sessionID, p)
}

func (c *Caster) keyframe() {
	if c.OnKeyframe != nil {
		c.OnKeyframe()
	}
}

func (c *Caster) status() Status {
	if c.stopped {
		return Status{}
	}
	s := Status{Active: true, BroadcastID: c.BroadcastID, Eligible: len(c.peers)}
	for _, r := range c.peers {
		if r.state == PeerConnected {
			s.Receivers++
		}
	}
	return s
}

// report calls OnStatus when the counts changed.
func (c *Caster) report() {
	s := c.status()
	if s != c.last {
		c.last = s
		if c.OnStatus != nil {
			c.OnStatus(s)
		}
	}
}

func (c *Caster) backoff(failures int) time.Duration {
	if c.Backoff != nil {
		return c.Backoff(failures)
	}
	return min(30*time.Second, time.Second<<min(failures, 5))
}

func (c *Caster) disconnectGrace() time.Duration {
	if c.DisconnectGrace > 0 {
		return c.DisconnectGrace
	}
	return DefaultDisconnectGrace
}

func (c *Caster) connectTimeout() time.Duration {
	if c.ConnectTimeout > 0 {
		return c.ConnectTimeout
	}
	return DefaultConnectTimeout
}

func (c *Caster) log() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.New(slog.DiscardHandler)
}
