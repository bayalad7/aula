package screen

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// IndicatorText is the always-visible notice (docs/privacidad.md rule 6).
const IndicatorText = "Tu pantalla está siendo vista por el profesor"

var errFrameTooLarge = errors.New("frame exceeds the protocol limit even at the lowest quality")

// Sender sends one message on the current connection to the console.
type Sender func(ctx context.Context, p protocol.Payload) error

// Streamer is the agent side of screen.thumb: it answers screen.thumb.start
// and screen.thumb.stop. A remote control (internal/remote) borrows it for
// its video with StartControl: then the thumbnails wait until StopControl.
type Streamer struct {
	// Capture grabs the screen; nil when the OS has no implementation, and
	// then the agent does not declare screen.thumb.
	Capture platform.ScreenCapture
	// Indicator shows that the screen is being viewed. Required with Capture:
	// without a visible indicator nothing is captured.
	Indicator platform.ScreenIndicator
	// OnViewed is called with true when a stream starts and false when it
	// ends, so the agent window can say so too. Optional.
	OnViewed func(viewed bool)
	Logger   *slog.Logger

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	viewed  bool
	thumb   *request // the screen.thumb.start in force, kept during a control
	control *request // the remote control that owns the stream, if any
	notice  int      // how many Announce calls showed the indicator (the last one hides it)
}

// request is a stream asked by the console: its connection and parameters.
type request struct {
	ctx   context.Context
	send  Sender
	refID string
	p     protocol.ScreenThumbStart
	text  string // the indicator
	seq   int    // last frame sent, so a retuned control keeps counting
}

// Capabilities lists what this streamer lets the agent declare in hello.
func (s *Streamer) Capabilities() []protocol.Capability {
	if !s.Available() {
		return nil
	}
	return []protocol.Capability{protocol.CapScreenThumb}
}

// Available reports whether the screen can be captured with its indicator.
func (s *Streamer) Available() bool { return s.Capture != nil && s.Indicator != nil }

// Handle processes screen.thumb.start and screen.thumb.stop. It reports false
// for any other message. reply is the message to send back, if any.
func (s *Streamer) Handle(ctx context.Context, send Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	switch p := env.Payload.(type) {
	case protocol.ScreenThumbStart:
		if !s.Available() {
			return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta screen.thumb"), true
		}
		if err := s.start(&request{ctx: ctx, send: send, refID: env.ID, p: p, text: IndicatorText}); err != nil {
			s.log().Warn("screen stream not started", "err", err)
			return protocol.NewError(env.ID, protocol.CodeInternal, "no se pudo mostrar el aviso de pantalla vista"), true
		}
		return nil, true
	case protocol.ScreenThumbStop:
		s.mu.Lock()
		controlled := s.control != nil
		if controlled {
			s.thumb = nil // nothing to resume after the control
		}
		s.mu.Unlock()
		if !controlled {
			s.Stop()
		}
		return nil, true
	}
	return nil, false
}

// Stop ends the stream, if any, a control's included, and removes the
// indicator. The agent calls it when the connection closes; it is idempotent.
func (s *Streamer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.halt()
	s.thumb, s.control = nil, nil
	s.unview()
}

// unview hides the indicator. s.mu is held.
func (s *Streamer) unview() {
	if !s.viewed {
		return
	}
	s.viewed = false
	if err := s.Indicator.Hide(); err != nil {
		s.log().Warn("cannot hide the screen indicator", "err", err)
	}
	if s.OnViewed != nil {
		s.OnViewed(false)
	}
	s.log().Info("screen stream stopped")
}

// Announce shows text on the indicator for d, for a one-off capture (a still
// screenshot, docs/privacidad.md rule 44). While a stream runs (thumbnails
// or a remote control) the indicator is already on screen and keeps its
// text: Announce changes nothing. It fails, and the caller must not capture,
// if the indicator cannot be shown.
func (s *Streamer) Announce(text string, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Available() {
		return errors.New("screen indicator unavailable")
	}
	if s.viewed {
		return nil
	}
	if err := s.Indicator.Show(text); err != nil {
		return err
	}
	s.notice++
	n := s.notice
	time.AfterFunc(d, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		// A later Announce extends it; a stream that started owns it now.
		if s.notice != n || s.viewed {
			return
		}
		if err := s.Indicator.Hide(); err != nil {
			s.log().Warn("cannot hide the screen indicator", "err", err)
		}
	})
	return nil
}

// StartControl streams the screen for a remote control (docs/protocol.md
// §5.16): frames go out as screen.thumb.frame with ref_id refID, at the pace
// of p, and the indicator says text. The thumbnails in force, if any, resume
// at StopControl. It fails, and nothing is captured, if the indicator cannot
// be shown.
func (s *Streamer) StartControl(ctx context.Context, send Sender, refID string, p protocol.ScreenThumbStart, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.halt()
	s.control = &request{ctx: ctx, send: send, refID: refID, p: p, text: text}
	if err := s.run(s.control); err != nil {
		s.control = nil
		if s.thumb != nil && s.thumb.ctx.Err() == nil {
			_ = s.run(s.thumb)
		} else {
			s.unview()
		}
		return err
	}
	return nil
}

// RetuneControl changes the pace of the control's video; frames keep their
// ref_id and their numbering.
func (s *Streamer) RetuneControl(p protocol.ScreenThumbStart) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.control == nil {
		return
	}
	s.halt()
	s.control.p = p
	if err := s.run(s.control); err != nil {
		s.log().Warn("control video not retuned", "err", err)
	}
}

// StopControl ends the control's video and resumes the thumbnails the
// console asked for, if any; otherwise it removes the indicator.
func (s *Streamer) StopControl() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.control == nil {
		return
	}
	s.halt()
	s.control = nil
	if s.thumb != nil && s.thumb.ctx.Err() == nil {
		if err := s.run(s.thumb); err == nil {
			return
		}
		s.thumb = nil
	}
	s.unview()
}

// start replaces the thumbnail stream; during a control it is only kept for
// later. The indicator is shown before the first capture; if it cannot be
// shown, nothing is captured.
func (s *Streamer) start(r *request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.control != nil {
		s.thumb = r
		return nil
	}
	s.halt()
	s.thumb = r
	if err := s.run(r); err != nil {
		s.thumb = nil
		return err
	}
	return nil
}

// run shows the indicator and starts the capture goroutine for r. s.mu is
// held and no goroutine runs.
func (s *Streamer) run(r *request) error {
	if err := s.Indicator.Show(r.text); err != nil {
		return err
	}
	if !s.viewed {
		s.viewed = true
		if s.OnViewed != nil {
			s.OnViewed(true)
		}
	}
	s.log().Info("screen stream started", "interval_ms", r.p.IntervalMS, "max_width", r.p.MaxWidth, "quality", r.p.Quality,
		"control", r == s.control)
	ctx, cancel := context.WithCancel(r.ctx)
	done := make(chan struct{})
	s.cancel, s.done = cancel, done
	go func() {
		defer close(done)
		s.loop(ctx, r)
	}()
	return nil
}

// halt stops the capture goroutine and waits for it. s.mu is held.
func (s *Streamer) halt() {
	if s.cancel != nil {
		s.cancel()
		<-s.done
		s.cancel, s.done = nil, nil
	}
}

// loop sends a frame right away and then one every interval, counted from
// the moment the previous one was queued: a frame never waits behind
// another, so a slow link lowers the pace instead of adding delay. A failed
// capture is reported once and retried; a failed send means the connection
// is gone.
func (s *Streamer) loop(ctx context.Context, r *request) {
	interval := time.Duration(r.p.IntervalMS) * time.Millisecond
	reported := false
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		f, err := s.capture(ctx, r.p)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			s.log().Debug("screen capture failed", "err", err)
			if !reported {
				reported = true
				if r.send(ctx, protocol.NewError(r.refID, protocol.CodeInternal, "no se pudo capturar la pantalla")) != nil {
					return
				}
			}
		default:
			r.seq++
			frame := protocol.ScreenThumbFrame{RefID: r.refID, Seq: r.seq, Width: f.Width, Height: f.Height, JPEG: f.JPEG}
			if err := r.send(ctx, frame); err != nil {
				s.log().Debug("screen frame not sent", "err", err)
				return
			}
		}
		t.Reset(interval)
	}
}

// capture grabs one frame, lowering the quality until it fits the protocol
// limit (docs/protocol.md §5.7).
func (s *Streamer) capture(ctx context.Context, p protocol.ScreenThumbStart) (platform.Frame, error) {
	q := p.Quality
	for {
		f, err := s.Capture.Capture(ctx, p.MaxWidth, q)
		if err != nil || len(f.JPEG) <= protocol.MaxFrameJPEG {
			return f, err
		}
		if q <= protocol.MinThumbQuality {
			return platform.Frame{}, errFrameTooLarge
		}
		q = max(protocol.MinThumbQuality, q-20)
	}
}

func (s *Streamer) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.DiscardHandler)
}
