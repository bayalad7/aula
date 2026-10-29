package broadcast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// ErrAlreadyBroadcasting is returned by Start while a broadcast is on.
var ErrAlreadyBroadcasting = errors.New("ya hay una transmisión en curso")

// Broadcaster is the console side of a broadcast of the teacher's screen:
// capture and encode one monitor (Source), one shared track and UDP port
// (Media) and the signaling with every agent (Caster).
type Broadcaster struct {
	Capture    platform.DisplayCapture
	NewEncoder func(platform.VideoEncoderConfig) (platform.VideoEncoder, error)
	Send       SendFunc
	// OnStatus is called when the broadcast starts, stops or its counts
	// change. It may run on any goroutine.
	OnStatus func(Status)
	Media    MediaOptions
	MaxKbps  int // 0 means DefaultMaxKbps
	Logger   *slog.Logger

	mu  sync.Mutex
	cur *onAir
}

type onAir struct {
	caster  *Caster
	media   *Media
	cancel  context.CancelFunc
	srcDone chan struct{}
	display int
}

// Displays lists the monitors that can be broadcast.
func (b *Broadcaster) Displays() ([]platform.Display, error) {
	if b.Capture == nil {
		return nil, platform.ErrNotSupported
	}
	return b.Capture.Displays()
}

// Start broadcasts monitor display to the given sessions and to those that
// join later (Sync). It fails before sending anything if the monitor cannot
// be captured or encoded, or the UDP port is busy.
func (b *Broadcaster) Start(display int, sessions []session.Session) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cur != nil {
		return ErrAlreadyBroadcasting
	}
	if b.Capture == nil || b.NewEncoder == nil {
		return fmt.Errorf("broadcast: %w", platform.ErrNotSupported)
	}
	displays, err := b.Capture.Displays()
	if err != nil {
		return fmt.Errorf("broadcast: %w", err)
	}
	if display < 0 || display >= len(displays) {
		return fmt.Errorf("broadcast: monitor %d does not exist", display)
	}
	ctx, cancel := context.WithCancel(context.Background())
	src := &Source{Capture: b.Capture, NewEncoder: b.NewEncoder, Display: display, MaxKbps: b.MaxKbps, Logger: b.Logger}
	if err := src.Prepare(ctx); err != nil {
		cancel()
		return err
	}
	media, err := NewMedia(b.mediaOptions())
	if err != nil {
		src.closeEncoder()
		cancel()
		return err
	}
	src.Write = media.WriteSample
	w, h := VideoSize(displays[display].Bounds, DefaultMaxWidth)
	caster := NewCaster(Caster{
		BroadcastID: protocol.NewID(),
		Start:       protocol.ScreenBroadcastStart{Width: w, Height: h, FPS: src.fps()},
		NewPeer:     media.NewPeer, Send: b.Send, OnStatus: b.OnStatus, OnKeyframe: src.RequestKeyframe,
		Logger: b.Logger,
	})
	cur := &onAir{caster: caster, media: media, cancel: cancel, srcDone: make(chan struct{}), display: display}
	go caster.Run(ctx)
	go func() {
		defer close(cur.srcDone)
		if err := src.Run(ctx); err != nil {
			b.log().Error("broadcast source stopped", "err", err)
			go b.Stop() // nothing to send any more
		}
	}()
	b.cur = cur
	b.log().Info("broadcast started", "broadcast", caster.BroadcastID, "display", display,
		"width", w, "height", h, "udp_port", media.Port())
	caster.Sync(sessions)
	return nil
}

// Sync follows the class: agents that join receive the broadcast, those that
// leave are dropped. A no-op without a broadcast.
func (b *Broadcaster) Sync(sessions []session.Session) {
	if c := b.caster(); c != nil {
		c.Sync(sessions)
	}
}

// Handle takes the agents' broadcast messages (answer, ICE, errors about an
// offer); it reports whether the message was an answer or ICE candidate of
// the broadcast in force.
func (b *Broadcaster) Handle(sessionID string, env protocol.Envelope) bool {
	if c := b.caster(); c != nil {
		return c.Handle(sessionID, env)
	}
	return false
}

// Stop ends the broadcast: every agent gets screen.broadcast.stop, the
// connections close and the UDP port is released. Idempotent.
func (b *Broadcaster) Stop() {
	b.mu.Lock()
	cur := b.cur
	b.cur = nil
	b.mu.Unlock()
	if cur == nil {
		return
	}
	cur.cancel()
	cur.caster.Wait()
	<-cur.srcDone
	if err := cur.media.Close(); err != nil {
		b.log().Debug("broadcast media close", "err", err)
	}
	b.log().Info("broadcast stopped", "broadcast", cur.caster.BroadcastID)
}

// Status returns the broadcast in force, or an inactive Status.
func (b *Broadcaster) Status() Status {
	if c := b.caster(); c != nil {
		return c.Status()
	}
	return Status{}
}

// Display returns the monitor on air, or -1.
func (b *Broadcaster) Display() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cur == nil {
		return -1
	}
	return b.cur.display
}

func (b *Broadcaster) caster() *Caster {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cur == nil {
		return nil
	}
	return b.cur.caster
}

func (b *Broadcaster) mediaOptions() MediaOptions {
	o := b.Media
	if o.Logger == nil {
		o.Logger = b.Logger
	}
	return o
}

func (b *Broadcaster) log() *slog.Logger {
	if b.Logger != nil {
		return b.Logger
	}
	return slog.New(slog.DiscardHandler)
}
