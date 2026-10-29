package broadcast

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/image/draw"

	"github.com/bayalad7/aula/internal/platform"
)

// Video parameters of the broadcast (D-014).
const (
	DefaultFPS         = 15
	DefaultMaxWidth    = 1920
	DefaultMaxKbps     = 2500
	MinKbps            = 800
	KeyframeEverySecs  = 5
	minKeyframeSpacing = time.Second // on-demand keyframes, for all receivers
	// staticResend is how often an unchanged screen is still encoded: most of
	// a programming class is a still editor, and skipping identical frames
	// saves most of the CPU.
	staticResend = time.Second
)

// VideoSize is the size the broadcast sends for a monitor of the given
// bounds: native up to maxWidth, scaled down keeping the proportions above
// it, and rounded down to even sides as H.264 4:2:0 needs.
func VideoSize(bounds image.Rectangle, maxWidth int) (w, h int) {
	w, h = bounds.Dx(), bounds.Dy()
	if w > maxWidth {
		h = h * maxWidth / w
		w = maxWidth
	}
	return w &^ 1, h &^ 1
}

// Source captures one monitor at a fixed rate, encodes each frame once and
// hands the access units to Write (the shared track).
type Source struct {
	Capture    platform.DisplayCapture
	NewEncoder func(platform.VideoEncoderConfig) (platform.VideoEncoder, error)
	Display    int
	FPS        int // 0 means DefaultFPS
	MaxWidth   int // 0 means DefaultMaxWidth
	MaxKbps    int // 0 means DefaultMaxKbps
	// Write sends one access unit lasting d. Called on the Run goroutine.
	Write  func(au []byte, d time.Duration) error
	Logger *slog.Logger

	mu       sync.Mutex
	wantKey  bool
	lastKey  time.Time
	scaled   *image.RGBA
	encoder  platform.VideoEncoder
	encW     int
	encH     int
	reported bool
	prev     []byte    // pixels of the last encoded frame
	lastSent time.Time // when it was encoded
}

// RequestKeyframe asks for an IDR on the next frame; requests closer than a
// second to the previous keyframe wait for it to pass (every receiver gets
// the same stream, so one PLI costs everybody bandwidth).
func (s *Source) RequestKeyframe() {
	s.mu.Lock()
	s.wantKey = true
	s.mu.Unlock()
}

// Run captures and encodes until ctx is canceled. A failed capture (locked
// session) is logged once and retried on the next tick; a failed encoder is
// fatal.
func (s *Source) Run(ctx context.Context) error {
	fps := s.fps()
	frame := time.Second / time.Duration(fps)
	t := time.NewTicker(frame)
	defer t.Stop()
	defer s.closeEncoder()
	for {
		if err := s.tick(ctx, fps, frame); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (s *Source) tick(ctx context.Context, fps int, frame time.Duration) error {
	img, err := s.Capture.Grab(ctx, s.Display)
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		if !s.reported {
			s.reported = true
			s.log().Warn("broadcast capture failed; retrying", "display", s.Display, "err", err)
		}
		return nil
	}
	s.reported = false
	img = s.fit(img)
	if err := s.ensureEncoder(img, fps); err != nil {
		return err
	}
	now := time.Now()
	s.mu.Lock()
	force := s.wantKey && now.Sub(s.lastKey) >= minKeyframeSpacing
	if force {
		s.wantKey, s.lastKey = false, now
	}
	s.mu.Unlock()
	if !force && now.Sub(s.lastSent) < staticResend && s.unchanged(img) {
		return nil
	}
	s.remember(img)
	s.lastSent = now
	au, err := s.encoder.Encode(img, force)
	if err != nil {
		return fmt.Errorf("broadcast encode: %w", err)
	}
	if len(au) == 0 {
		return nil
	}
	if err := s.Write(au, frame); err != nil {
		s.log().Debug("broadcast sample not written", "err", err)
	}
	return nil
}

// Prepare grabs one frame and opens the encoder for it, so a broadcast that
// cannot work (no H.264 encoder, no capture) fails before anything is sent.
func (s *Source) Prepare(ctx context.Context) error {
	img, err := s.Capture.Grab(ctx, s.Display)
	if err != nil {
		return fmt.Errorf("broadcast capture: %w", err)
	}
	return s.ensureEncoder(s.fit(img), s.fps())
}

// ensureEncoder opens the encoder for the first frame, or again when the
// monitor changes resolution.
func (s *Source) ensureEncoder(img *image.RGBA, fps int) error {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if s.encoder != nil && w == s.encW && h == s.encH {
		return nil
	}
	s.closeEncoder()
	kbps := s.MaxKbps
	if kbps <= 0 {
		kbps = DefaultMaxKbps
	}
	enc, err := s.NewEncoder(platform.VideoEncoderConfig{Width: w, Height: h, FPS: fps,
		BitrateKbps: kbps, KeyframeEvery: fps * KeyframeEverySecs})
	if err != nil {
		return fmt.Errorf("broadcast encoder: %w", err)
	}
	s.encoder, s.encW, s.encH = enc, w, h
	s.log().Info("broadcast encoding", "width", w, "height", h, "fps", fps, "kbps", kbps)
	return nil
}

func (s *Source) fps() int {
	if s.FPS <= 0 {
		return DefaultFPS
	}
	return s.FPS
}

// unchanged reports whether img has the same pixels as the last encoded
// frame.
func (s *Source) unchanged(img *image.RGBA) bool {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if len(s.prev) != w*h*4 {
		return false
	}
	for y := range h {
		if !bytes.Equal(img.Pix[y*img.Stride:y*img.Stride+w*4], s.prev[y*w*4:(y+1)*w*4]) {
			return false
		}
	}
	return true
}

// remember keeps a copy of the pixels of the frame about to be encoded.
func (s *Source) remember(img *image.RGBA) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if len(s.prev) != w*h*4 {
		s.prev = make([]byte, w*h*4)
	}
	for y := range h {
		copy(s.prev[y*w*4:(y+1)*w*4], img.Pix[y*img.Stride:y*img.Stride+w*4])
	}
}

// fit scales img down to MaxWidth and crops it to even sides.
func (s *Source) fit(img *image.RGBA) *image.RGBA {
	maxW := s.MaxWidth
	if maxW <= 0 {
		maxW = DefaultMaxWidth
	}
	b := img.Bounds()
	w, h := VideoSize(b, maxW)
	if b.Dx() > maxW {
		if s.scaled == nil || s.scaled.Bounds().Dx() != w || s.scaled.Bounds().Dy() != h {
			s.scaled = image.NewRGBA(image.Rect(0, 0, w, h))
		}
		// BiLinear keeps code legible; it only runs above 1920 px.
		draw.BiLinear.Scale(s.scaled, s.scaled.Bounds(), img, b, draw.Src, nil)
		return s.scaled
	}
	if w != b.Dx() || h != b.Dy() {
		return img.SubImage(image.Rect(b.Min.X, b.Min.Y, b.Min.X+w, b.Min.Y+h)).(*image.RGBA)
	}
	return img
}

func (s *Source) closeEncoder() {
	if s.encoder != nil {
		if err := s.encoder.Close(); err != nil {
			s.log().Debug("broadcast encoder close", "err", err)
		}
		s.encoder = nil
	}
}

func (s *Source) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.DiscardHandler)
}
