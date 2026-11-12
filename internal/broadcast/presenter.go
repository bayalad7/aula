package broadcast

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// PresentIndicatorText is the always-visible notice while this screen is
// shown to the class (docs/privacidad.md rule 76).
const PresentIndicatorText = "Tu pantalla se está mostrando a la clase"

// Phases of a presentation on the source agent, as its window shows them.
const (
	PresentOff  = ""     // nothing on screen
	PresentLive = "live" // the band «Tu pantalla se está mostrando a la clase»
)

// PresentView is what the source agent's window shows. The window words it
// (Go sends states, not sentences).
type PresentView struct {
	Phase     string `json:"phase"`
	PresentID string `json:"presentId,omitempty"`
}

// ErrNoPresentation is returned to the helper when there is no presentation.
var ErrNoPresentation = errors.New("no hay una proyección de tu pantalla en curso")

// Presenter is the agent side of student.present.*. The student is not asked
// (docs/privacidad.md rule 75, like a remote control): at start it raises the
// band and the native badge, and only then opens the capture; it answers the
// console's offers through a Publication and ends when the console stops it,
// the capture fails or the connection closes. The student cannot end it.
type Presenter struct {
	// Publisher opens the capture, the encoder and the connection; nil when
	// this agent cannot present, and then it does not declare
	// screen.present.
	Publisher Publisher
	// Indicator is the native badge (docs/privacidad.md rule 76). Required:
	// without a visible badge nothing is published.
	Indicator platform.ScreenIndicator
	// OnView shows or removes the band (and brings the window forward). It
	// runs without the presenter's lock, in order.
	OnView func(PresentView)
	Logger *slog.Logger

	mu     sync.Mutex
	cur    *presenting
	outbox chan func() // window updates and messages, in order
	once   sync.Once
}

type presenting struct {
	id      string
	startID string // envelope ID of the start
	pub     Publication
	shown   bool // the badge is on screen
	send    Sender
	attempt int // of the offer in force
	remote  int // candidates received in this attempt
	local   int // candidates sent in this attempt
}

// Capabilities lists what the presenter lets the agent declare in hello.
func (p *Presenter) Capabilities() []protocol.Capability {
	if !p.Available() {
		return nil
	}
	return []protocol.Capability{protocol.CapScreenPresent}
}

// Available reports whether the agent can present: a publisher, the badge
// and a window for the band.
func (p *Presenter) Available() bool {
	if p.Publisher == nil || p.Indicator == nil || p.OnView == nil {
		return false
	}
	if l, ok := p.Publisher.(*LocalPublisher); ok {
		return l.Available()
	}
	return true
}

// Handle processes the student.present.* messages and reports false for any
// other. reply is the message to send back, if any.
func (p *Presenter) Handle(_ context.Context, send Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	switch env.Payload.(type) {
	case protocol.StudentPresentStart, protocol.StudentPresentOffer, protocol.StudentPresentICE, protocol.StudentPresentStop:
	default:
		return nil, false
	}
	if !p.Available() {
		return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta screen.present"), true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	switch m := env.Payload.(type) {
	case protocol.StudentPresentStart:
		return p.start(send, env.ID, m), true
	case protocol.StudentPresentOffer:
		return p.offer(env.ID, m), true
	case protocol.StudentPresentICE:
		p.remoteCandidate(m)
		return nil, true
	case protocol.StudentPresentStop:
		if p.cur == nil || p.cur.id != m.PresentID {
			return nil, true
		}
		return p.end(protocol.PresentEnded, "", env.ID, false), true
	}
	return nil, true
}

// start shows the band and the badge, then opens the capture, replacing any
// presentation in force. p.mu is held.
func (p *Presenter) start(send Sender, envID string, m protocol.StudentPresentStart) protocol.Payload {
	if p.cur != nil {
		if p.cur.id == m.PresentID {
			// A resend (the console did not get the ack): it is already on.
			w, h := p.cur.pub.Size()
			return protocol.StudentPresentAck{RefID: envID, PresentID: m.PresentID, Status: protocol.PresentAccepted, Width: w, Height: h}
		}
		old := p.cur
		p.sendLater(old, old.send, p.end(protocol.PresentEnded, "", envID, false))
	}
	c := &presenting{id: m.PresentID, startID: envID, send: send}
	p.cur = c
	// The notice first (rule 76): nothing is captured before it is on screen.
	if err := p.Indicator.Show(PresentIndicatorText); err != nil {
		p.log().Warn("presentation badge not shown", "err", err)
		p.sendLater(c, send, p.end(protocol.PresentFailed, protocol.PresentCaptureUnavailable, envID, false))
		return nil
	}
	c.shown = true
	p.show(PresentView{Phase: PresentLive, PresentID: c.id})
	pub, err := p.Publisher.Open(PublishOptions{MaxWidth: m.MaxWidth, FPS: m.FPS, MaxKbps: m.MaxKbps}, PublishEvents{
		OnCandidate: func(attempt int, cand Candidate) { p.localCandidate(c, attempt, cand) },
		OnFail:      func(error) { p.fail(c, protocol.PresentEncoderUnavailable) },
	})
	if err != nil {
		reason := protocol.PresentCaptureUnavailable
		if errors.Is(err, ErrEncoderUnavailable) {
			reason = protocol.PresentEncoderUnavailable
		}
		p.log().Warn("presentation not opened", "present", c.id, "err", err)
		p.sendLater(c, send, p.end(protocol.PresentFailed, reason, envID, false))
		return nil
	}
	c.pub = pub
	w, h := pub.Size()
	p.log().Info("presentation started", "present", c.id, "width", w, "height", h)
	// Queued, not returned: after the old presentation's ended.
	p.sendLater(c, send, protocol.StudentPresentAck{RefID: envID, PresentID: c.id, Status: protocol.PresentAccepted, Width: w, Height: h})
	return nil
}

// Stop ends the presentation without telling the console: the connection
// closed. Idempotent.
func (p *Presenter) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cur != nil {
		p.end(protocol.PresentEnded, "", "", true)
	}
}

// Current returns what the window shows.
func (p *Presenter) Current() PresentView {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.view()
}

// Wait blocks until every window update and message queued so far has run
// (tests).
func (p *Presenter) Wait() {
	done := make(chan struct{})
	p.mu.Lock()
	p.later(func() { close(done) })
	p.mu.Unlock()
	<-done
}

func (p *Presenter) view() PresentView {
	if p.cur == nil {
		return PresentView{}
	}
	return PresentView{Phase: PresentLive, PresentID: p.cur.id}
}

// offer answers the console's offer of a newer attempt. p.mu is held.
func (p *Presenter) offer(envID string, m protocol.StudentPresentOffer) protocol.Payload {
	c := p.cur
	if c == nil || c.id != m.PresentID || c.pub == nil {
		return protocol.NewError(envID, protocol.CodeInvalidMessage, "oferta sin una proyección en curso")
	}
	if m.Attempt <= c.attempt {
		return nil // an old offer
	}
	if err := checkSDP(m.SDP, "recvonly"); err != nil {
		p.log().Warn("presentation offer refused", "present", c.id, "attempt", m.Attempt, "err", err)
		return protocol.NewError(envID, protocol.CodeInvalidMessage, "la oferta debe pedir solo recibir un video")
	}
	c.attempt, c.remote, c.local = m.Attempt, 0, 0
	answer, err := c.pub.Answer(m.Attempt, m.SDP)
	if err != nil {
		p.log().Warn("presentation answer not created", "present", c.id, "attempt", m.Attempt, "err", err)
		return protocol.NewError(envID, protocol.CodeInternal, "no se pudo abrir la conexión de video")
	}
	return protocol.StudentPresentAnswer{RefID: envID, PresentID: c.id, Attempt: m.Attempt, SDP: answer}
}

// remoteCandidate applies a console candidate of the attempt in force.
// p.mu is held.
func (p *Presenter) remoteCandidate(m protocol.StudentPresentICE) {
	c := p.cur
	if c == nil || c.id != m.PresentID || c.pub == nil || m.Attempt != c.attempt || c.remote >= protocol.MaxICECandidates {
		return
	}
	c.remote++
	if err := c.pub.AddCandidate(m.Attempt, Candidate{Candidate: m.Candidate, SDPMid: m.SDPMid, SDPMLineIndex: m.SDPMLineIndex}); err != nil {
		p.log().Debug("presentation candidate refused", "err", err)
	}
}

func (p *Presenter) localCandidate(c *presenting, attempt int, cand Candidate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cur != c || attempt != c.attempt || c.local >= protocol.MaxICECandidates {
		return
	}
	c.local++
	p.sendLater(c, c.send, protocol.StudentPresentICE{PresentID: c.id, Attempt: attempt,
		Candidate: cand.Candidate, SDPMid: cand.SDPMid, SDPMLineIndex: cand.SDPMLineIndex})
}

func (p *Presenter) fail(c *presenting, reason protocol.PresentReason) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cur != c {
		return
	}
	p.sendLater(c, c.send, p.end(protocol.PresentFailed, reason, c.startID, false))
}

// end closes the presentation in force and returns its final ack (nil when
// quiet: the connection is gone). p.mu is held.
func (p *Presenter) end(status protocol.PresentStatus, reason protocol.PresentReason, refID string, quiet bool) protocol.Payload {
	c := p.cur
	p.cur = nil
	if c.pub != nil {
		if err := c.pub.Close(); err != nil {
			p.log().Warn("presentation close", "err", err)
		}
	}
	if c.shown {
		if err := p.Indicator.Hide(); err != nil {
			p.log().Warn("cannot hide the presentation badge", "err", err)
		}
	}
	p.show(PresentView{})
	p.log().Info("presentation ended", "present", c.id, "status", status, "reason", reason)
	if quiet {
		return nil
	}
	return protocol.StudentPresentAck{RefID: refID, PresentID: c.id, Status: status, Reason: reason}
}

// show queues an OnView call. p.mu is held.
func (p *Presenter) show(v PresentView) { p.later(func() { p.OnView(v) }) }

// sendLater queues m for c's connection: window updates and messages run in
// order on one goroutine, without the lock (the console may answer at once).
// p.mu is held.
func (p *Presenter) sendLater(c *presenting, send Sender, m protocol.Payload) {
	if m == nil || send == nil {
		return
	}
	id := c.id
	p.later(func() {
		ctx, cancel := context.WithTimeout(context.Background(), replyTimeout)
		defer cancel()
		if err := send(ctx, m); err != nil {
			p.log().Debug("presentation message not sent", "present", id, "type", m.MessageType(), "err", err)
		}
	})
}

// later queues f on the presenter's goroutine. p.mu is held.
func (p *Presenter) later(f func()) {
	p.once.Do(func() {
		p.outbox = make(chan func(), 256)
		go func() {
			for f := range p.outbox {
				f()
			}
		}()
	})
	p.outbox <- f
}

func (p *Presenter) log() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.New(slog.DiscardHandler)
}
