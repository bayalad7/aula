package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// IndicatorText is what the native indicator says while a screenshot is
// taken (docs/privacidad.md rule 44). Like screen.IndicatorText, it is the
// only text of this feature that lives in Go: the indicator is not a page.
const IndicatorText = "Tu profesor está tomando una captura de tu pantalla"

// busyMessage is the error the agent sends while another screenshot is in
// progress; the console recognizes it (ReasonBusy).
const busyMessage = "ya hay una captura en curso"

// Defaults of Taker.
const (
	DefaultAnnounceFor = 5 * time.Second
	// DefaultSettle lets the indicator be painted before the capture, so it
	// shows in the image: the teacher sees what the student knows.
	DefaultSettle = 300 * time.Millisecond
)

// Announcer shows the native indicator for a while. screen.Streamer
// implements it: it owns the indicator, and while a stream or a remote
// control runs, the indicator keeps its own text.
type Announcer interface {
	Announce(text string, d time.Duration) error
}

// Sender sends one message on the current connection to the console.
type Sender func(ctx context.Context, p protocol.Payload) error

// Taker is the agent side of screen.snapshot.request.
type Taker struct {
	// Capture grabs the screen; nil when the OS has none, and then the agent
	// does not declare screen.snapshot.
	Capture platform.ScreenCapture
	// Indicator shows the notice before the capture. Required with Capture.
	Indicator Announcer
	// Identity is this computer's name and the session's user, at the time
	// of the capture.
	Identity func() (hostname, username string)
	// OnTaken runs right after the capture, before the image goes out: the
	// window shows its notice (notify, never raise). Optional.
	OnTaken func(at time.Time)

	AnnounceFor time.Duration // 0: DefaultAnnounceFor
	Settle      time.Duration // 0: DefaultSettle; negative: no wait (tests)
	Now         func() time.Time
	Logger      *slog.Logger

	mu   sync.Mutex
	busy bool
	wg   sync.WaitGroup
}

// Available reports whether a screenshot can be taken with its notice.
func (t *Taker) Available() bool { return t.Capture != nil && t.Indicator != nil }

// Capabilities lists what this taker lets the agent declare in hello.
func (t *Taker) Capabilities() []protocol.Capability {
	if !t.Available() {
		return nil
	}
	return []protocol.Capability{protocol.CapScreenSnapshot}
}

// Handle processes screen.snapshot.request and reports false for any other
// message. The capture runs in the background, tied to ctx (the connection):
// reply is set only when the request is refused right away.
func (t *Taker) Handle(ctx context.Context, send Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	p, ok := env.Payload.(protocol.ScreenSnapshotRequest)
	if !ok {
		return nil, false
	}
	if !t.Available() {
		return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta screen.snapshot"), true
	}
	t.mu.Lock()
	if t.busy {
		t.mu.Unlock()
		t.log().Info("screenshot refused: another one in progress", "snapshot_id", p.SnapshotID)
		return protocol.NewError(env.ID, protocol.CodeInternal, busyMessage), true
	}
	t.busy = true
	t.wg.Add(1)
	t.mu.Unlock()
	go func() {
		defer t.wg.Done()
		defer func() {
			t.mu.Lock()
			t.busy = false
			t.mu.Unlock()
		}()
		t.take(ctx, send, env.ID, p)
	}()
	return nil, true
}

// Wait blocks until the screenshot in progress, if any, is sent or dropped.
func (t *Taker) Wait() { t.wg.Wait() }

var errTooLarge = errors.New("screenshot exceeds the protocol limit even at the lowest quality")

func (t *Taker) take(ctx context.Context, send Sender, refID string, p protocol.ScreenSnapshotRequest) {
	log := t.log().With("snapshot_id", p.SnapshotID)
	fail := func(code protocol.ErrorCode, msg string) {
		if err := send(ctx, protocol.NewError(refID, code, msg)); err != nil {
			log.Debug("screenshot error not sent", "err", err)
		}
	}
	// Rule 44: the student is told before anything is captured.
	if err := t.Indicator.Announce(IndicatorText, t.announceFor()); err != nil {
		log.Warn("screenshot not taken: the indicator cannot be shown", "err", err)
		fail(protocol.CodeInternal, "no se pudo mostrar el aviso de captura")
		return
	}
	if d := t.settle(); d > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(d):
		}
	}
	f, err := t.capture(ctx, p)
	switch {
	case ctx.Err() != nil:
		return
	case errors.Is(err, errTooLarge):
		log.Warn("screenshot too large", "err", err)
		fail(protocol.CodeMessageTooLarge, "la captura no cabe en el límite del protocolo")
		return
	case err != nil:
		log.Warn("screenshot capture failed", "err", err)
		fail(protocol.CodeInternal, "no se pudo capturar la pantalla")
		return
	}
	at := t.now()
	if t.OnTaken != nil {
		t.OnTaken(at)
	}
	host, user := "", ""
	if t.Identity != nil {
		host, user = t.Identity()
	}
	sum := sha256.Sum256(f.JPEG)
	parts := protocol.SplitParts(f.JPEG, protocol.SnapshotPartSize)
	for i, data := range parts {
		r := protocol.ScreenSnapshotResponse{RefID: refID, SnapshotID: p.SnapshotID, Part: i + 1, Parts: len(parts), Data: data}
		if i == 0 {
			r.TakenAt, r.Hostname, r.Username = at.UnixMilli(), host, user
			r.Width, r.Height, r.Size, r.SHA256 = f.Width, f.Height, len(f.JPEG), hex.EncodeToString(sum[:])
		}
		if err := send(ctx, r); err != nil {
			// The connection is gone: the image is dropped, never kept.
			log.Info("screenshot not delivered", "part", i+1, "parts", len(parts), "err", err)
			return
		}
	}
	log.Info("screenshot sent", "parts", len(parts), "bytes", len(f.JPEG), "width", f.Width, "height", f.Height)
}

// capture grabs the display, lowering the quality by 10 down to the minimum
// until the JPEG fits (docs/protocol.md §5.17).
func (t *Taker) capture(ctx context.Context, p protocol.ScreenSnapshotRequest) (platform.Frame, error) {
	width := p.MaxWidth
	if width == 0 {
		width = protocol.MaxSnapshotWidth // native: the capture never enlarges
	}
	for q := p.Quality; ; q -= 10 {
		q = max(q, protocol.MinSnapshotQuality)
		f, err := t.Capture.Capture(ctx, width, q)
		if err != nil {
			return platform.Frame{}, err
		}
		if len(f.JPEG) == 0 || f.Width < 1 || f.Height < 1 || f.Width > protocol.MaxSnapshotSide || f.Height > protocol.MaxSnapshotSide {
			return platform.Frame{}, errors.New("capture returned an unusable image")
		}
		if len(f.JPEG) <= protocol.MaxSnapshotBytes {
			return f, nil
		}
		if q == protocol.MinSnapshotQuality {
			return platform.Frame{}, errTooLarge
		}
	}
}

func (t *Taker) announceFor() time.Duration {
	if t.AnnounceFor > 0 {
		return t.AnnounceFor
	}
	return DefaultAnnounceFor
}

func (t *Taker) settle() time.Duration {
	if t.Settle == 0 {
		return DefaultSettle
	}
	return t.Settle
}

func (t *Taker) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

func (t *Taker) log() *slog.Logger {
	if t.Logger != nil {
		return t.Logger
	}
	return slog.New(slog.DiscardHandler)
}
