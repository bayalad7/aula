package broadcast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/bayalad7/aula/internal/platform"
)

// Why a presentation could not open (docs/protocol.md §5.24): they map to
// the reasons of student.present.ack failed.
var (
	ErrCaptureUnavailable = errors.New("no se puede capturar la pantalla de este equipo")
	ErrEncoderUnavailable = errors.New("este equipo no tiene codificador H.264")
)

// PublishOptions are the video limits the console asked for.
type PublishOptions struct {
	MaxWidth int
	FPS      int
	MaxKbps  int
}

// PublishEvents are the callbacks of a Publication. They may run on any
// goroutine.
type PublishEvents struct {
	// OnCandidate hands a local ICE candidate of the given attempt to the
	// console; an empty Candidate ends them.
	OnCandidate func(attempt int, c Candidate)
	// OnFail reports that the publication cannot go on (the encoder
	// failed); the agent ends the presentation.
	OnFail func(error)
}

// Publisher opens the media side of a presentation on the source agent: the
// capture, the encoder and the send-only WebRTC connection. LocalPublisher
// implements it in the process that sees the screen; in service mode a
// proxy reaches the helper's (internal/agentsvc).
type Publisher interface {
	// Open excludes the agent's own windows from the capture, captures one
	// frame and opens the encoder, so a presentation that cannot work fails
	// before it is accepted. Errors wrap ErrCaptureUnavailable or
	// ErrEncoderUnavailable.
	Open(opt PublishOptions, ev PublishEvents) (Publication, error)
}

// Publication is an open presentation: one encoder and, per attempt, one
// connection to the console.
type Publication interface {
	// Size is the size of the video.
	Size() (w, h int)
	// Answer replaces the connection with one for the console's offer of
	// this attempt and returns the send-only answer.
	Answer(attempt int, offer string) (string, error)
	// AddCandidate applies a remote candidate of the attempt in force;
	// older attempts are ignored.
	AddCandidate(attempt int, c Candidate) error
	// Close stops the capture, closes the connection and lets the windows
	// be captured again. Idempotent.
	Close() error
}

// LocalPublisher publishes this process's view of the primary monitor with
// pion (full ICE, host candidates only, no STUN or TURN): the source only
// connects out to the console's UDP port.
type LocalPublisher struct {
	Capture    platform.DisplayCapture
	NewEncoder func(platform.VideoEncoderConfig) (platform.VideoEncoder, error)
	// Exclude hides this process's windows from the capture (the Aula
	// window: a private message must not reach the class, docs/privacidad.md
	// rule 78). Required: without it nothing is published.
	Exclude func(exclude bool) error
	// Loopback also gathers 127.0.0.1 (tests on one machine).
	Loopback bool
	Logger   *slog.Logger
}

// Available reports whether the pieces to publish are there.
func (l *LocalPublisher) Available() bool {
	return l != nil && l.Capture != nil && l.NewEncoder != nil && l.Exclude != nil
}

// Open implements Publisher.
func (l *LocalPublisher) Open(opt PublishOptions, ev PublishEvents) (Publication, error) {
	if !l.Available() {
		return nil, fmt.Errorf("present: %w", ErrCaptureUnavailable)
	}
	displays, err := l.Capture.Displays()
	if err != nil || len(displays) == 0 {
		return nil, fmt.Errorf("present: %w: %v", ErrCaptureUnavailable, err)
	}
	display := displays[0]
	for _, d := range displays {
		if d.Primary {
			display = d
			break
		}
	}
	if err := l.Exclude(true); err != nil {
		return nil, fmt.Errorf("present: %w: %w", ErrCaptureUnavailable, err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = l.Exclude(false)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	src := &Source{Capture: l.Capture, NewEncoder: l.NewEncoder, Display: display.Index,
		FPS: opt.FPS, MaxWidth: opt.MaxWidth, MaxKbps: opt.MaxKbps, Logger: l.Logger}
	if err := src.Prepare(ctx); err != nil {
		cancel()
		if errors.Is(err, errEncoder) {
			return nil, fmt.Errorf("present: %w: %w", ErrEncoderUnavailable, err)
		}
		return nil, fmt.Errorf("present: %w: %w", ErrCaptureUnavailable, err)
	}
	track, err := webrtc.NewTrackLocalStaticSample(h264Capability, "pantalla", "aula-alumno")
	if err != nil {
		src.closeEncoder()
		cancel()
		return nil, fmt.Errorf("present: %w", err)
	}
	me, ir, err := h264Engine()
	if err != nil {
		src.closeEncoder()
		cancel()
		return nil, err
	}
	var se webrtc.SettingEngine
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetIncludeLoopbackCandidate(l.Loopback)
	w, h := VideoSize(display.Bounds, src.maxWidth())
	p := &publication{
		api:   webrtc.NewAPI(webrtc.WithMediaEngine(me), webrtc.WithInterceptorRegistry(ir), webrtc.WithSettingEngine(se)),
		track: track, src: src, ev: ev, cancel: cancel, exclude: l.Exclude, w: w, h: h,
		done: make(chan struct{}), log: l.log(),
	}
	src.Write = func(au []byte, d time.Duration) error {
		return track.WriteSample(media.Sample{Data: au, Duration: d})
	}
	go func() {
		defer close(p.done)
		if err := src.Run(ctx); err != nil {
			p.log.Error("presentation source stopped", "err", err)
			if ev.OnFail != nil {
				// Not on this goroutine: the agent ends the presentation, and
				// Close waits for it.
				go ev.OnFail(fmt.Errorf("present: %w: %w", ErrEncoderUnavailable, err))
			}
		}
	}()
	ok = true
	p.log.Info("presentation opened", "width", w, "height", h, "fps", src.fps())
	return p, nil
}

func (l *LocalPublisher) log() *slog.Logger {
	if l.Logger != nil {
		return l.Logger
	}
	return slog.New(slog.DiscardHandler)
}

type publication struct {
	api     *webrtc.API
	track   *webrtc.TrackLocalStaticSample
	src     *Source
	ev      PublishEvents
	cancel  context.CancelFunc
	exclude func(bool) error
	w, h    int
	done    chan struct{}
	log     *slog.Logger

	mu      sync.Mutex
	pc      *webrtc.PeerConnection
	attempt int
	closed  bool
}

func (p *publication) Size() (int, int) { return p.w, p.h }

func (p *publication) Answer(attempt int, offer string) (string, error) {
	if err := checkSDP(offer, "recvonly"); err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return "", errors.New("present: publication closed")
	}
	if attempt <= p.attempt {
		return "", fmt.Errorf("present: attempt %d is not newer than %d", attempt, p.attempt)
	}
	p.closePC()
	pc, err := p.api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return "", fmt.Errorf("peer connection: %w", err)
	}
	tr, err := pc.AddTransceiverFromTrack(p.track, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly})
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("add track: %w", err)
	}
	wireEvents(pc, PeerEvents{
		OnCandidate: func(c Candidate) {
			if p.ev.OnCandidate != nil {
				p.ev.OnCandidate(attempt, c)
			}
		},
		OnState: func(s PeerState) {
			p.log.Info("presentation connection", "attempt", attempt, "state", int(s))
		},
	})
	// Keyframe requests that the console forwards from the receivers.
	go func() {
		for {
			pkts, _, err := tr.Sender().ReadRTCP()
			if err != nil {
				return
			}
			for _, pk := range pkts {
				switch pk.(type) {
				case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
					p.src.RequestKeyframe()
				}
			}
		}
	}()
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		pc.Close()
		return "", fmt.Errorf("set remote description: %w", err)
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("create answer: %w", err)
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return "", fmt.Errorf("set local description: %w", err)
	}
	p.pc, p.attempt = pc, attempt
	p.src.RequestKeyframe()
	return pc.LocalDescription().SDP, nil
}

func (p *publication) AddCandidate(attempt int, c Candidate) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pc == nil || attempt != p.attempt || c.Candidate == "" {
		return nil
	}
	idx := uint16(c.SDPMLineIndex)
	if err := p.pc.AddICECandidate(webrtc.ICECandidateInit{Candidate: c.Candidate, SDPMid: &c.SDPMid, SDPMLineIndex: &idx}); err != nil {
		return fmt.Errorf("add candidate: %w", err)
	}
	return nil
}

func (p *publication) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.closePC()
	p.mu.Unlock()
	p.cancel()
	<-p.done
	err := p.exclude(false)
	p.log.Info("presentation closed")
	if err != nil {
		return fmt.Errorf("present: windows not shown to captures again: %w", err)
	}
	return nil
}

// closePC closes the connection of the attempt in force. p.mu is held.
func (p *publication) closePC() {
	if p.pc != nil {
		if err := p.pc.Close(); err != nil && !errors.Is(err, webrtc.ErrConnectionClosed) {
			p.log.Debug("presentation connection close", "err", err)
		}
		p.pc = nil
	}
}
