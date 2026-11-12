package broadcast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// ConsoleViewID is the receiver that stands for the console's own view of a
// presentation (its WebView): it is not a session and is not counted.
const ConsoleViewID = "consola"

// Outcomes of a presentation that ended, for the console's notice.
const (
	ProjectStopped    = "stopped"     // the teacher pressed «Detener» (or started another)
	ProjectFailed     = "failed"      // see Reason
	ProjectSourceLeft = "source_left" // the source's connection closed
)

// ReasonNoAnswer is the Reason of a presentation whose source never
// acknowledged the start.
const ReasonNoAnswer = "no_answer"

// DefaultStartWait bounds the wait for the source's first ack.
const DefaultStartWait = 15 * time.Second

// Phases of a presentation in the console. The student is not asked: it is
// connecting from the start.
const (
	ProjectConnecting = "connecting" // the source's video is not here yet
	ProjectLive       = "live"       // the source's video arrives
)

// ErrPresentUnsupported is returned by Start for an agent without
// screen.present.
var ErrPresentUnsupported = errors.New("ese equipo no puede mostrar su pantalla a la clase")

// ProjectStatus is the presentation as the console shows it. When not
// Active, Ended (and Hostname, Reason) say how the last one ended.
type ProjectStatus struct {
	Active    bool   `json:"active"`
	PresentID string `json:"presentId,omitempty"`
	SourceID  string `json:"sourceId,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	Phase     string `json:"phase,omitempty"`
	Receivers int    `json:"receivers"` // students whose video is connected
	Eligible  int    `json:"eligible"`  // students it is offered to
	Ended     string `json:"ended,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// RelayMedia is the console's WebRTC side of a presentation: *Media with
// Relay set; tests use a double.
type RelayMedia interface {
	NewPeer(ev PeerEvents) (Peer, error)
	NewSourcePeer(ev PeerEvents) (SourcePeer, error)
	Close() error
}

// Projector is the console side of showing a student's screen to the class
// (docs/protocol.md §5.24): it asks the source for consent, receives its
// video on one connection and forwards it to every other student with
// screen.present.view (a Caster), and to the console's own view.
type Projector struct {
	Send SendFunc
	// OnStatus is called when the presentation starts, changes phase or
	// counts, or ends. It may run on any goroutine.
	OnStatus func(ProjectStatus)
	// OnView and OnSignal drive the console's own view of the video, as the
	// agent's Viewer does for its window.
	OnView   func(View)
	OnSignal func(Signal)
	Media    MediaOptions
	// OpenMedia opens the relay; nil means NewMedia.
	OpenMedia func(MediaOptions) (RelayMedia, error)
	// FPS, MaxWidth and MaxKbps ask the source for its video; 0 means the
	// defaults of the teacher's broadcast.
	FPS, MaxWidth, MaxKbps int
	// StartWait bounds the wait for the source's first ack; 0 means
	// DefaultStartWait.
	StartWait       time.Duration
	Backoff         func(failures int) time.Duration
	DisconnectGrace time.Duration
	ConnectTimeout  time.Duration
	Logger          *slog.Logger

	mu  sync.Mutex
	cur *projection

	viewMu     sync.Mutex
	viewOffers map[int]string // attempt → offer ID of the console's view
	viewID     string         // broadcast ID shown in the console's view
}

// Start asks source to show its screen to the other sessions (and to those
// that join later, Sync). A presentation in force ends first. It fails
// before sending anything if the source cannot present or the UDP port is
// busy.
func (p *Projector) Start(source session.Session, sessions []session.Session) error {
	if source.State != session.StateConnected || !source.Can(protocol.CapScreenPresent) {
		return ErrPresentUnsupported
	}
	p.Stop()
	p.mu.Lock()
	defer p.mu.Unlock()
	opt := p.Media
	opt.Relay, opt.Loopback = true, true // the console's own view connects from this machine
	if opt.Logger == nil {
		opt.Logger = p.Logger
	}
	open := p.OpenMedia
	if open == nil {
		open = func(o MediaOptions) (RelayMedia, error) { return NewMedia(o) }
	}
	media, err := open(opt)
	if err != nil {
		return fmt.Errorf("present: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	pr := &projection{
		p: p, id: protocol.NewID(), sourceID: source.ID, hostname: source.Hostname,
		media: media, cancel: cancel, events: make(chan func(), 64), done: make(chan struct{}), closing: make(chan struct{}),
		sessions: sessions, phase: ProjectConnecting,
	}
	p.cur = pr
	go pr.run(ctx)
	pr.post(pr.begin)
	return nil
}

// Sync follows the class: a source that leaves ends the presentation;
// students that join receive it, those that leave are dropped.
func (p *Projector) Sync(sessions []session.Session) {
	if pr := p.current(); pr != nil {
		pr.post(func() { pr.sync(sessions) })
	}
}

// Handle takes the agents' presentation messages: the source's acks,
// answers, candidates and errors, and the receivers' screen.broadcast.*
// answers and candidates. It reports whether the message belonged to the
// presentation in force.
func (p *Projector) Handle(sessionID string, env protocol.Envelope) bool {
	pr := p.current()
	if pr == nil {
		return false
	}
	return pr.handle(sessionID, env)
}

// Stop ends the presentation in force, if any: the source gets
// student.present.stop, every receiver screen.broadcast.stop, and the UDP
// port is released. Idempotent.
func (p *Projector) Stop() {
	if pr := p.current(); pr != nil {
		pr.post(func() { pr.end(ProjectStopped, "", true) })
		<-pr.done
	}
}

// Status returns the presentation in force, or an inactive Status.
func (p *Projector) Status() ProjectStatus {
	pr := p.current()
	if pr == nil {
		return ProjectStatus{}
	}
	res := make(chan ProjectStatus, 1)
	pr.post(func() { res <- pr.status() })
	select {
	case s := <-res:
		return s
	case <-pr.done:
		return ProjectStatus{}
	}
}

// OpenView adds the console's own view as a receiver; CloseView removes it
// and hides it.
func (p *Projector) OpenView() { p.setView(true) }

// CloseView implements the console's «Cerrar» of its view.
func (p *Projector) CloseView() { p.setView(false) }

func (p *Projector) setView(open bool) {
	pr := p.current()
	if pr == nil {
		return
	}
	pr.post(func() {
		pr.viewOpen = open
		if pr.caster != nil {
			pr.caster.Sync(pr.receivers())
		}
	})
	if !open && p.OnView != nil {
		p.OnView(View{})
	}
}

// ViewAnswer hands the console view's SDP answer to the presentation.
func (p *Projector) ViewAnswer(broadcastID string, attempt int, sdp string) error {
	ref, err := p.viewOffer(broadcastID, attempt)
	if err != nil {
		return err
	}
	return p.viewHandle(protocol.ScreenBroadcastAnswer{RefID: ref, BroadcastID: broadcastID, Attempt: attempt, SDP: sdp})
}

// ViewCandidate hands one of the console view's ICE candidates over.
func (p *Projector) ViewCandidate(broadcastID string, attempt int, candidate, mid string, mline int) error {
	if _, err := p.viewOffer(broadcastID, attempt); err != nil {
		return err
	}
	return p.viewHandle(protocol.ScreenBroadcastICE{BroadcastID: broadcastID, Attempt: attempt,
		Candidate: candidate, SDPMid: mid, SDPMLineIndex: mline})
}

// ViewFailed says the console's view could not open this offer; a new one
// follows.
func (p *Projector) ViewFailed(broadcastID string, attempt int) error {
	ref, err := p.viewOffer(broadcastID, attempt)
	if err != nil {
		return err
	}
	return p.viewHandle(protocol.ErrorPayload{RefID: ref, Code: protocol.CodeInternal, Message: "la vista de la consola no abrió el video"})
}

func (p *Projector) viewOffer(broadcastID string, attempt int) (string, error) {
	p.viewMu.Lock()
	defer p.viewMu.Unlock()
	ref, ok := p.viewOffers[attempt]
	if !ok || broadcastID != p.viewID {
		return "", ErrNoBroadcast
	}
	return ref, nil
}

func (p *Projector) viewHandle(m protocol.Payload) error {
	if err := m.Validate(); err != nil {
		return err
	}
	pr := p.current()
	if pr == nil {
		return ErrNoBroadcast
	}
	pr.post(func() {
		if pr.caster != nil {
			pr.caster.Handle(ConsoleViewID, protocol.New(m))
		}
	})
	return nil
}

// sendView plays the transport for the console's own view: what a session
// would receive becomes OnView and OnSignal. It runs on the caster's
// goroutine.
func (p *Projector) sendView(m protocol.Payload) (string, error) {
	id := protocol.NewID()
	switch m := m.(type) {
	case protocol.ScreenBroadcastStart:
		p.viewMu.Lock()
		p.viewID, p.viewOffers = m.BroadcastID, map[int]string{}
		p.viewMu.Unlock()
		v := View{Active: true, BroadcastID: m.BroadcastID, Width: m.Width, Height: m.Height, FPS: m.FPS}
		if m.Source != nil {
			v.Source = m.Source.Hostname
		}
		if p.OnView != nil {
			p.OnView(v)
		}
	case protocol.ScreenBroadcastOffer:
		p.viewMu.Lock()
		p.viewOffers[m.Attempt] = id
		p.viewMu.Unlock()
		if p.OnSignal != nil {
			p.OnSignal(Signal{Kind: SignalOffer, BroadcastID: m.BroadcastID, Attempt: m.Attempt, SDP: m.SDP})
		}
	case protocol.ScreenBroadcastICE:
		if p.OnSignal != nil {
			p.OnSignal(Signal{Kind: SignalICE, BroadcastID: m.BroadcastID, Attempt: m.Attempt,
				Candidate: m.Candidate, SDPMid: m.SDPMid, SDPMLineIndex: m.SDPMLineIndex})
		}
	case protocol.ScreenBroadcastStop:
		p.viewMu.Lock()
		p.viewID, p.viewOffers = "", nil
		p.viewMu.Unlock()
		if p.OnView != nil {
			p.OnView(View{})
		}
	}
	return id, nil
}

func (p *Projector) current() *projection {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cur
}

// finished forgets pr once it ended by itself.
func (p *Projector) finished(pr *projection) {
	p.mu.Lock()
	if p.cur == pr {
		p.cur = nil
	}
	p.mu.Unlock()
}

func (p *Projector) log() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.New(slog.DiscardHandler)
}

// projection is one presentation. All its state lives on its run goroutine.
type projection struct {
	p        *Projector
	id       string
	sourceID string
	hostname string
	media    RelayMedia
	cancel   context.CancelFunc
	events   chan func()
	done     chan struct{}
	closing  chan struct{} // closed when end starts: late events are dropped
	sessions []session.Session
	viewOpen bool

	phase    string
	startID  string      // envelope ID of the start
	guard    *time.Timer // the wait for the source's first ack
	ended    bool
	width    int
	height   int
	caster   *Caster
	castStop context.CancelFunc
	counts   Status // the caster's last report

	// The source leg: like one receiver of a Caster, the other way round.
	attempt  int
	offerID  string
	peer     SourcePeer
	state    PeerState
	failures int
	timer    *time.Timer
	lastKey  time.Time
	last     ProjectStatus
}

func (pr *projection) run(ctx context.Context) {
	defer close(pr.done)
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-pr.events:
			f()
			if pr.ended {
				return
			}
			pr.report()
		}
	}
}

// post runs f on the run goroutine; it is dropped once the presentation
// is ending (end waits for the caster, whose callbacks post).
func (pr *projection) post(f func()) {
	select {
	case pr.events <- f:
	case <-pr.closing:
	case <-pr.done:
	}
}

func (pr *projection) handle(sessionID string, env protocol.Envelope) bool {
	if sessionID == pr.sourceID {
		switch m := env.Payload.(type) {
		case protocol.StudentPresentAck:
			if m.PresentID != pr.id {
				return false
			}
			pr.post(func() { pr.ack(m) })
			return true
		case protocol.StudentPresentAnswer:
			if m.PresentID != pr.id {
				return false
			}
			pr.post(func() { pr.answer(m) })
			return true
		case protocol.StudentPresentICE:
			if m.PresentID != pr.id {
				return false
			}
			pr.post(func() { pr.remoteCandidate(m) })
			return true
		case protocol.ErrorPayload:
			pr.post(func() { pr.sourceError(m) })
			return false
		}
		return false
	}
	switch env.Payload.(type) {
	case protocol.ScreenBroadcastAnswer, protocol.ScreenBroadcastICE, protocol.ErrorPayload:
		handled := make(chan bool, 1)
		pr.post(func() {
			handled <- pr.caster != nil && pr.caster.Handle(sessionID, env)
		})
		select {
		case h := <-handled:
			return h
		case <-pr.done:
		}
	}
	return false
}

// --- run goroutine -----------------------------------------------------------------

func (pr *projection) begin() {
	start := protocol.StudentPresentStart{PresentID: pr.id, MaxWidth: pr.maxWidth(), FPS: pr.fps(), MaxKbps: pr.maxKbps()}
	id, err := pr.send(pr.sourceID, start)
	if err != nil {
		pr.log().Info("presentation start not sent", "conn", pr.sourceID, "err", err)
		pr.end(ProjectSourceLeft, "", false)
		return
	}
	pr.startID = id
	wait := pr.p.StartWait
	if wait <= 0 {
		wait = DefaultStartWait
	}
	pr.guard = time.AfterFunc(wait, func() {
		pr.post(func() {
			if pr.caster == nil {
				pr.log().Info("presentation source never answered", "present", pr.id)
				pr.end(ProjectFailed, ReasonNoAnswer, true)
			}
		})
	})
	pr.log().Info("presentation started", "present", pr.id, "conn", pr.sourceID)
}

func (pr *projection) sync(sessions []session.Session) {
	pr.sessions = sessions
	present := false
	for _, s := range sessions {
		if s.ID == pr.sourceID && s.State == session.StateConnected {
			present = true
		}
	}
	if !present {
		pr.log().Info("presentation source left", "present", pr.id, "conn", pr.sourceID)
		pr.end(ProjectSourceLeft, "", false)
		return
	}
	if pr.caster != nil {
		pr.caster.Sync(pr.receivers())
	}
}

// receivers are the sessions the caster offers the video to, plus the
// console's view when open; Eligible leaves the source out.
func (pr *projection) receivers() []session.Session {
	list := pr.sessions
	if pr.viewOpen {
		list = append(append([]session.Session{}, list...), session.Session{ID: ConsoleViewID, State: session.StateConnected,
			Capabilities: []protocol.Capability{protocol.CapScreenBroadcast, protocol.CapScreenPresentView}})
	}
	return list
}

func (pr *projection) ack(m protocol.StudentPresentAck) {
	switch m.Status {
	case protocol.PresentAccepted:
		if pr.caster != nil {
			return // a resend
		}
		pr.guard.Stop()
		pr.width, pr.height = m.Width, m.Height
		pr.log().Info("presentation on the source's screen", "present", pr.id, "width", m.Width, "height", m.Height)
		pr.openCaster()
		pr.offer()
	case protocol.PresentFailed:
		pr.end(ProjectFailed, string(m.Reason), false)
	default: // ended
		pr.end(ProjectStopped, "", false)
	}
}

func (pr *projection) sourceError(m protocol.ErrorPayload) {
	switch {
	case m.RefID != "" && m.RefID == pr.startID:
		pr.log().Info("source refused the presentation", "present", pr.id, "code", m.Code)
		pr.end(ProjectFailed, string(m.Code), false)
	case m.RefID != "" && m.RefID == pr.offerID:
		pr.log().Info("source could not open the presentation connection", "present", pr.id, "attempt", pr.attempt, "code", m.Code)
		pr.retry()
	}
}

func (pr *projection) openCaster() {
	ctx, cancel := context.WithCancel(context.Background())
	pr.castStop = cancel
	pr.caster = NewCaster(Caster{
		BroadcastID: protocol.NewID(),
		Start: protocol.ScreenBroadcastStart{Width: pr.width, Height: pr.height, FPS: pr.fps(),
			Source: &protocol.BroadcastSource{Kind: protocol.BroadcastSourceStudent, Hostname: pr.hostname}},
		NewPeer: pr.media.NewPeer,
		Send: func(ctx context.Context, sessionID string, m protocol.Payload) (string, error) {
			if sessionID == ConsoleViewID {
				return pr.p.sendView(m)
			}
			return pr.p.Send(ctx, sessionID, m)
		},
		OnStatus:   func(s Status) { pr.post(func() { pr.counts = s }) },
		OnKeyframe: func() { pr.post(pr.keyframe) },
		Eligible: func(s session.Session) bool {
			return s.ID != pr.sourceID && s.Can(protocol.CapScreenPresentView)
		},
		Uncounted:       ConsoleViewID,
		Backoff:         pr.p.Backoff,
		DisconnectGrace: pr.p.DisconnectGrace,
		ConnectTimeout:  pr.p.ConnectTimeout,
		Logger:          pr.p.Logger,
	})
	go pr.caster.Run(ctx)
	pr.caster.Sync(pr.receivers())
}

// offer opens a new source connection for the next attempt.
func (pr *projection) offer() {
	pr.closePeer()
	if pr.attempt >= protocol.MaxBroadcastAttempt {
		pr.end(ProjectFailed, "attempts", true)
		return
	}
	pr.attempt++
	pr.offerID, pr.state = "", PeerConnecting
	attempt := pr.attempt
	var self SourcePeer
	current := func() bool { return pr.attempt == attempt && pr.peer == self && self != nil }
	peer, err := pr.media.NewSourcePeer(PeerEvents{
		OnCandidate: func(c Candidate) {
			pr.post(func() {
				if current() {
					pr.localCandidate(c)
				}
			})
		},
		OnState: func(s PeerState) {
			pr.post(func() {
				if current() {
					pr.peerState(s)
				}
			})
		},
	})
	if err != nil {
		pr.log().Warn("presentation connection not created", "err", err)
		pr.retry()
		return
	}
	pr.peer, self = peer, peer
	sdp, err := peer.Offer()
	if err != nil {
		pr.log().Warn("presentation offer not created", "err", err)
		pr.retry()
		return
	}
	id, err := pr.send(pr.sourceID, protocol.StudentPresentOffer{PresentID: pr.id, Attempt: attempt, SDP: sdp})
	if err != nil {
		pr.log().Debug("presentation offer not sent", "err", err)
		pr.retry()
		return
	}
	pr.offerID = id
	pr.arm(pr.connectTimeout(), "connect timeout")
}

func (pr *projection) answer(m protocol.StudentPresentAnswer) {
	if pr.peer == nil || m.Attempt != pr.attempt || m.RefID != pr.offerID {
		return
	}
	if err := pr.peer.SetAnswer(m.SDP); err != nil {
		pr.log().Info("presentation answer refused", "attempt", m.Attempt, "err", err)
		pr.retry()
	}
}

func (pr *projection) remoteCandidate(m protocol.StudentPresentICE) {
	if pr.peer == nil || m.Attempt != pr.attempt || m.Candidate == "" || strings.Contains(m.Candidate, ".local ") {
		return // the ICE-lite side learns the source's address from its checks
	}
	if err := pr.peer.AddCandidate(Candidate{Candidate: m.Candidate, SDPMid: m.SDPMid, SDPMLineIndex: m.SDPMLineIndex}); err != nil {
		pr.log().Debug("presentation candidate refused", "err", err)
	}
}

func (pr *projection) localCandidate(c Candidate) {
	if pr.offerID == "" {
		return
	}
	if _, err := pr.send(pr.sourceID, protocol.StudentPresentICE{PresentID: pr.id, Attempt: pr.attempt,
		Candidate: c.Candidate, SDPMid: c.SDPMid, SDPMLineIndex: c.SDPMLineIndex}); err != nil {
		pr.log().Debug("presentation candidate not sent", "err", err)
	}
}

func (pr *projection) peerState(s PeerState) {
	prev := pr.state
	pr.state = s
	switch s {
	case PeerConnected:
		pr.failures = 0
		pr.disarm()
		pr.phase = ProjectLive
		if prev != PeerConnected {
			pr.log().Info("presentation source connected", "attempt", pr.attempt)
			pr.keyframe()
		}
	case PeerDisconnected:
		pr.phase = ProjectConnecting
		pr.arm(pr.disconnectGrace(), "disconnected")
	case PeerFailed:
		pr.phase = ProjectConnecting
		pr.retry()
	}
}

// keyframe forwards a receiver's request to the source, at most once a
// second for all of them.
func (pr *projection) keyframe() {
	if pr.peer == nil || time.Since(pr.lastKey) < minKeyframeSpacing {
		return
	}
	pr.lastKey = time.Now()
	if err := pr.peer.RequestKeyframe(); err != nil {
		pr.log().Debug("presentation keyframe request", "err", err)
	}
}

func (pr *projection) retry() {
	pr.closePeer()
	pr.phase = ProjectConnecting
	wait := pr.backoff(pr.failures)
	pr.failures++
	pr.arm(wait, "")
}

func (pr *projection) arm(after time.Duration, reason string) {
	pr.disarm()
	attempt := pr.attempt
	pr.timer = time.AfterFunc(after, func() {
		pr.post(func() {
			if pr.attempt != attempt {
				return
			}
			if reason == "" {
				pr.offer()
				return
			}
			if pr.state != PeerConnected {
				pr.log().Info("presentation attempt expired", "attempt", pr.attempt, "why", reason)
				pr.retry()
			}
		})
	})
}

func (pr *projection) disarm() {
	if pr.timer != nil {
		pr.timer.Stop()
		pr.timer = nil
	}
}

func (pr *projection) closePeer() {
	if pr.peer != nil {
		if err := pr.peer.Close(); err != nil {
			pr.log().Debug("presentation connection close", "err", err)
		}
		pr.peer = nil
	}
	pr.state = PeerClosed
}

// end closes everything. tell sends student.present.stop to the source
// (not when the source itself ended it or is gone).
func (pr *projection) end(outcome, reason string, tell bool) {
	if pr.ended {
		return
	}
	pr.ended = true
	close(pr.closing)
	if pr.guard != nil {
		pr.guard.Stop()
	}
	pr.disarm()
	if tell {
		if _, err := pr.send(pr.sourceID, protocol.StudentPresentStop{PresentID: pr.id}); err != nil {
			pr.log().Debug("presentation stop not sent", "err", err)
		}
	}
	if pr.caster != nil {
		pr.castStop()
		pr.caster.Wait() // every receiver got screen.broadcast.stop
	}
	pr.closePeer()
	if err := pr.media.Close(); err != nil {
		pr.log().Debug("presentation media close", "err", err)
	}
	pr.p.finished(pr)
	pr.cancel()
	pr.log().Info("presentation ended", "present", pr.id, "outcome", outcome, "reason", reason)
	if pr.p.OnStatus != nil {
		pr.p.OnStatus(ProjectStatus{Hostname: pr.hostname, SourceID: pr.sourceID, Ended: outcome, Reason: reason})
	}
}

func (pr *projection) status() ProjectStatus {
	return ProjectStatus{Active: true, PresentID: pr.id, SourceID: pr.sourceID, Hostname: pr.hostname,
		Phase: pr.phase, Receivers: pr.counts.Receivers, Eligible: pr.counts.Eligible}
}

func (pr *projection) report() {
	s := pr.status()
	if s != pr.last {
		pr.last = s
		if pr.p.OnStatus != nil {
			pr.p.OnStatus(s)
		}
	}
}

func (pr *projection) send(sessionID string, m protocol.Payload) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	return pr.p.Send(ctx, sessionID, m)
}

func (pr *projection) fps() int {
	if pr.p.FPS > 0 {
		return min(pr.p.FPS, protocol.MaxPresentFPS)
	}
	return DefaultFPS
}

func (pr *projection) maxWidth() int {
	if pr.p.MaxWidth > 0 {
		return max(protocol.MinPresentWidth, min(pr.p.MaxWidth, protocol.MaxPresentWidth))
	}
	return DefaultMaxWidth
}

func (pr *projection) maxKbps() int {
	if pr.p.MaxKbps > 0 {
		return max(protocol.MinPresentKbps, min(pr.p.MaxKbps, protocol.MaxPresentKbps))
	}
	return DefaultMaxKbps
}

func (pr *projection) backoff(failures int) time.Duration {
	if pr.p.Backoff != nil {
		return pr.p.Backoff(failures)
	}
	return min(30*time.Second, time.Second<<min(failures, 5))
}

func (pr *projection) disconnectGrace() time.Duration {
	if pr.p.DisconnectGrace > 0 {
		return pr.p.DisconnectGrace
	}
	return DefaultDisconnectGrace
}

func (pr *projection) connectTimeout() time.Duration {
	if pr.p.ConnectTimeout > 0 {
		return pr.p.ConnectTimeout
	}
	return DefaultConnectTimeout
}

func (pr *projection) log() *slog.Logger { return pr.p.log() }
