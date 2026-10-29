package broadcast

import (
	"context"
	"image"
	"image/color"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
)

// stillScreen returns the same image until change is called.
type stillScreen struct{ img *image.RGBA }

func (s *stillScreen) Displays() ([]platform.Display, error) {
	return []platform.Display{{Bounds: s.img.Bounds(), Primary: true}}, nil
}
func (s *stillScreen) Grab(context.Context, int) (*image.RGBA, error) { return s.img, nil }
func (s *stillScreen) change(x int)                                   { s.img.SetRGBA(x, 0, color.RGBA{255, 0, 0, 255}) }

type countEncoder struct{ frames, keys atomic.Int64 }

func (e *countEncoder) Encode(_ *image.RGBA, key bool) ([]byte, error) {
	e.frames.Add(1)
	if key {
		e.keys.Add(1)
	}
	return []byte{0, 0, 0, 1, 0x41}, nil
}
func (e *countEncoder) Close() error { return nil }

// A still screen is encoded once and then only once per second; a change
// or a keyframe request is encoded right away.
func TestSourceSkipsIdenticalFrames(t *testing.T) {
	screen := &stillScreen{img: image.NewRGBA(image.Rect(0, 0, 64, 36))}
	enc := &countEncoder{}
	var written atomic.Int64
	s := &Source{Capture: screen, NewEncoder: func(platform.VideoEncoderConfig) (platform.VideoEncoder, error) { return enc, nil },
		Write: func([]byte, time.Duration) error { written.Add(1); return nil }}
	ctx := t.Context()
	tick := func() {
		t.Helper()
		if err := s.tick(ctx, 15, time.Second/15); err != nil {
			t.Fatal(err)
		}
	}
	for range 10 {
		tick()
	}
	if n := enc.frames.Load(); n != 1 {
		t.Fatalf("still screen encoded %d times in under a second, want 1", n)
	}
	screen.change(3)
	tick()
	tick()
	if n := enc.frames.Load(); n != 2 {
		t.Fatalf("encoded %d frames after one change, want 2", n)
	}
	s.RequestKeyframe()
	tick()
	if enc.frames.Load() != 3 || enc.keys.Load() != 1 {
		t.Fatalf("keyframe request: %d frames, %d keys", enc.frames.Load(), enc.keys.Load())
	}
	s.lastSent = time.Now().Add(-staticResend) // a second later
	tick()
	if n := enc.frames.Load(); n != 4 {
		t.Fatalf("still screen not resent after %v: %d frames", staticResend, n)
	}
	if written.Load() != enc.frames.Load() {
		t.Fatalf("wrote %d access units for %d frames", written.Load(), enc.frames.Load())
	}
}
