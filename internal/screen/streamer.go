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
// and screen.thumb.stop.
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

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	viewed bool
}

// Capabilities lists what this streamer lets the agent declare in hello.
func (s *Streamer) Capabilities() []protocol.Capability {
	if s.Capture == nil || s.Indicator == nil {
		return nil
	}
	return []protocol.Capability{protocol.CapScreenThumb}
}

// Handle processes screen.thumb.start and screen.thumb.stop. It reports false
// for any other message. reply is the message to send back, if any.
func (s *Streamer) Handle(ctx context.Context, send Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	switch p := env.Payload.(type) {
	case protocol.ScreenThumbStart:
		if s.Capture == nil || s.Indicator == nil {
			return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta screen.thumb"), true
		}
		if err := s.start(ctx, send, env.ID, p); err != nil {
			s.log().Warn("screen stream not started", "err", err)
			return protocol.NewError(env.ID, protocol.CodeInternal, "no se pudo mostrar el aviso de pantalla vista"), true
		}
		return nil, true
	case protocol.ScreenThumbStop:
		s.Stop()
		return nil, true
	}
	return nil, false
}

// Stop ends the stream, if any, and removes the indicator. The agent calls it
// when the connection closes; it is idempotent.
func (s *Streamer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.halt()
	if s.viewed {
		s.viewed = false
		if err := s.Indicator.Hide(); err != nil {
			s.log().Warn("cannot hide the screen indicator", "err", err)
		}
		if s.OnViewed != nil {
			s.OnViewed(false)
		}
		s.log().Info("screen stream stopped")
	}
}

// start replaces the running stream, if any. The indicator is shown before
// the first capture; if it cannot be shown, nothing is captured.
func (s *Streamer) start(ctx context.Context, send Sender, refID string, p protocol.ScreenThumbStart) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.halt()
	if err := s.Indicator.Show(IndicatorText); err != nil {
		return err
	}
	if !s.viewed {
		s.viewed = true
		if s.OnViewed != nil {
			s.OnViewed(true)
		}
	}
	s.log().Info("screen stream started", "interval_ms", p.IntervalMS, "max_width", p.MaxWidth, "quality", p.Quality)
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.cancel, s.done = cancel, done
	go func() {
		defer close(done)
		s.loop(ctx, send, refID, p)
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
// the moment the previous one was queued. A failed capture is reported once
// and retried; a failed send means the connection is gone.
func (s *Streamer) loop(ctx context.Context, send Sender, refID string, p protocol.ScreenThumbStart) {
	interval := time.Duration(p.IntervalMS) * time.Millisecond
	seq, reported := 0, false
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		f, err := s.capture(ctx, p)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			s.log().Debug("screen capture failed", "err", err)
			if !reported {
				reported = true
				if send(ctx, protocol.NewError(refID, protocol.CodeInternal, "no se pudo capturar la pantalla")) != nil {
					return
				}
			}
		default:
			seq++
			frame := protocol.ScreenThumbFrame{RefID: refID, Seq: seq, Width: f.Width, Height: f.Height, JPEG: f.JPEG}
			if err := send(ctx, frame); err != nil {
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
