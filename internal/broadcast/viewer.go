package broadcast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Sender sends one message on the current connection to the console.
type Sender func(ctx context.Context, p protocol.Payload) error

// View is what the agent window shows while a broadcast is on.
type View struct {
	Active      bool   `json:"active"`
	BroadcastID string `json:"broadcastId"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps"`
}

// Signal kinds handed to the frontend.
const (
	SignalOffer = "offer"
	SignalICE   = "ice"
)

// Signal is an offer or a remote ICE candidate for the frontend's
// RTCPeerConnection. A new offer (higher Attempt) replaces the connection.
type Signal struct {
	Kind          string `json:"kind"`
	BroadcastID   string `json:"broadcastId"`
	Attempt       int    `json:"attempt"`
	SDP           string `json:"sdp,omitempty"`
	Candidate     string `json:"candidate,omitempty"`
	SDPMid        string `json:"sdpMid,omitempty"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
}

// Errors returned to the frontend.
var (
	ErrNoBroadcast = errors.New("no broadcast in force with that ID and attempt")
	ErrNoViewer    = errors.New("this agent cannot show a broadcast")
)

// replyTimeout bounds a message the frontend asks the agent to send.
const replyTimeout = 5 * time.Second

// Viewer is the agent side of screen.broadcast. The WebRTC connection lives
// in the frontend; the viewer checks the signaling, shows the view first and
// relays answers and candidates back to the console.
type Viewer struct {
	// Supported is set when the frontend can play the broadcast
	// (RTCPeerConnection and an H.264 decoder). Without it the agent does not
	// declare screen.broadcast and refuses its messages.
	Supported bool
	// OnView shows or hides the broadcast view. It is called, and returns,
	// before the offer is handed over: the student sees the notice first.
	// Returning an error refuses the broadcast. OnView and OnSignal run with
	// the viewer locked: they must not call back into it.
	OnView func(View) error
	// OnSignal hands an offer or a candidate to the frontend.
	OnSignal func(Signal)
	Logger   *slog.Logger

	mu   sync.Mutex
	cur  *watching // nil when no broadcast is on
	send Sender
	ctx  context.Context
}

type watching struct {
	view    View
	attempt int    // of the offer in force; 0 before the first
	offerID string // envelope ID of that offer
	remote  int    // candidates received in this attempt
	local   int    // candidates sent in this attempt
}

// Capabilities lists what this viewer lets the agent declare in hello.
func (v *Viewer) Capabilities() []protocol.Capability {
	if !v.Supported || v.OnView == nil || v.OnSignal == nil {
		return nil
	}
	return []protocol.Capability{protocol.CapScreenBroadcast}
}

// Handle processes the screen.broadcast.* messages from the console and
// reports false for any other. reply is the message to send back, if any.
func (v *Viewer) Handle(ctx context.Context, send Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	switch env.Payload.(type) {
	case protocol.ScreenBroadcastStart, protocol.ScreenBroadcastOffer, protocol.ScreenBroadcastICE, protocol.ScreenBroadcastStop:
	default:
		return nil, false
	}
	if v.Capabilities() == nil {
		return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta screen.broadcast"), true
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	switch p := env.Payload.(type) {
	case protocol.ScreenBroadcastStart:
		return v.start(ctx, send, env.ID, p), true
	case protocol.ScreenBroadcastOffer:
		return v.offer(env.ID, p), true
	case protocol.ScreenBroadcastICE:
		v.remoteCandidate(p)
		return nil, true
	case protocol.ScreenBroadcastStop:
		if v.cur != nil && v.cur.view.BroadcastID == p.BroadcastID {
			v.end("stop")
		}
		return nil, true
	}
	return nil, true
}

// start shows the view, replacing any broadcast in force. v.mu is held.
func (v *Viewer) start(ctx context.Context, send Sender, envID string, p protocol.ScreenBroadcastStart) protocol.Payload {
	if v.cur != nil {
		v.end("replaced")
	}
	view := View{Active: true, BroadcastID: p.BroadcastID, Width: p.Width, Height: p.Height, FPS: p.FPS}
	if err := v.OnView(view); err != nil {
		v.log().Warn("broadcast view not shown", "err", err)
		return protocol.NewError(envID, protocol.CodeInternal, "no se pudo mostrar la transmisión")
	}
	v.cur, v.send, v.ctx = &watching{view: view}, send, ctx
	v.log().Info("broadcast started", "broadcast", p.BroadcastID, "width", p.Width, "height", p.Height)
	return nil
}

// offer hands a valid offer of a newer attempt to the frontend. v.mu is held.
func (v *Viewer) offer(envID string, p protocol.ScreenBroadcastOffer) protocol.Payload {
	if v.cur == nil || v.cur.view.BroadcastID != p.BroadcastID {
		return protocol.NewError(envID, protocol.CodeInvalidMessage, "oferta sin screen.broadcast.start")
	}
	if p.Attempt <= v.cur.attempt {
		v.log().Debug("stale broadcast offer dropped", "attempt", p.Attempt, "current", v.cur.attempt)
		return nil
	}
	if err := checkSDP(p.SDP, "sendonly"); err != nil {
		v.log().Warn("broadcast offer refused", "err", err)
		return protocol.NewError(envID, protocol.CodeInvalidMessage, "la oferta debe ser solo video de recepción")
	}
	v.cur.attempt, v.cur.offerID, v.cur.remote, v.cur.local = p.Attempt, envID, 0, 0
	v.log().Info("broadcast offer", "broadcast", p.BroadcastID, "attempt", p.Attempt, "bytes", len(p.SDP))
	v.OnSignal(Signal{Kind: SignalOffer, BroadcastID: p.BroadcastID, Attempt: p.Attempt, SDP: p.SDP})
	return nil
}

// remoteCandidate relays a console candidate of the attempt in force. v.mu
// is held.
func (v *Viewer) remoteCandidate(p protocol.ScreenBroadcastICE) {
	if v.cur == nil || v.cur.view.BroadcastID != p.BroadcastID || p.Attempt != v.cur.attempt {
		return // old attempts are dropped without error (§5.12.4)
	}
	if v.cur.remote >= protocol.MaxICECandidates {
		return
	}
	v.cur.remote++
	v.OnSignal(Signal{Kind: SignalICE, BroadcastID: p.BroadcastID, Attempt: p.Attempt,
		Candidate: p.Candidate, SDPMid: p.SDPMid, SDPMLineIndex: p.SDPMLineIndex})
}

// Answer sends the frontend's SDP answer for the offer in force.
func (v *Viewer) Answer(broadcastID string, attempt int, answer string) error {
	if err := checkSDP(answer, "recvonly"); err != nil {
		return fmt.Errorf("broadcast answer: %w", err)
	}
	return v.reply(broadcastID, attempt, func(w *watching) protocol.Payload {
		return protocol.ScreenBroadcastAnswer{RefID: w.offerID, BroadcastID: broadcastID, Attempt: attempt, SDP: answer}
	})
}

// LocalCandidate sends one of the frontend's ICE candidates; "" ends them.
func (v *Viewer) LocalCandidate(broadcastID string, attempt int, candidate, mid string, mline int) error {
	return v.reply(broadcastID, attempt, func(w *watching) protocol.Payload {
		if w.local >= protocol.MaxICECandidates {
			return nil
		}
		w.local++
		return protocol.ScreenBroadcastICE{BroadcastID: broadcastID, Attempt: attempt,
			Candidate: candidate, SDPMid: mid, SDPMLineIndex: mline}
	})
}

// Failed tells the console that the frontend could not set up the
// connection for this offer; it retries with a new one.
func (v *Viewer) Failed(broadcastID string, attempt int) error {
	return v.reply(broadcastID, attempt, func(w *watching) protocol.Payload {
		return protocol.NewError(w.offerID, protocol.CodeInternal, "no se pudo abrir la conexión de video")
	})
}

// reply builds a message for the attempt in force and sends it outside the
// lock, so a slow connection never blocks the console's messages.
func (v *Viewer) reply(broadcastID string, attempt int, build func(*watching) protocol.Payload) error {
	v.mu.Lock()
	if v.cur == nil || v.cur.view.BroadcastID != broadcastID || v.cur.attempt != attempt {
		v.mu.Unlock()
		return ErrNoBroadcast
	}
	p := build(v.cur)
	send, ctx := v.send, v.ctx
	v.mu.Unlock()
	if p == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, replyTimeout)
	defer cancel()
	if err := send(ctx, p); err != nil {
		return fmt.Errorf("broadcast %s: %w", p.MessageType(), err)
	}
	return nil
}

// Current returns the view in force, for the first render of the window.
func (v *Viewer) Current() View {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cur == nil {
		return View{}
	}
	return v.cur.view
}

// Stop ends the broadcast, if any. The agent calls it when the connection
// closes: the broadcast belongs to the session. Idempotent.
func (v *Viewer) Stop() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cur != nil {
		v.end("disconnected")
	}
}

// end hides the view; the frontend closes its connection on Active=false.
// v.mu is held.
func (v *Viewer) end(why string) {
	id := v.cur.view.BroadcastID
	v.cur, v.send, v.ctx = nil, nil, nil
	if err := v.OnView(View{BroadcastID: id}); err != nil {
		v.log().Warn("broadcast view not hidden", "err", err)
	}
	v.log().Info("broadcast ended", "broadcast", id, "why", why)
}

func (v *Viewer) log() *slog.Logger {
	if v.Logger != nil {
		return v.Logger
	}
	return slog.New(slog.DiscardHandler)
}
