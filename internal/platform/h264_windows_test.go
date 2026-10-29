//go:build windows && (amd64 || arm64)

package platform

import (
	"errors"
	"image"
	"image/color"
	"os"
	"slices"
	"testing"
	"time"
)

// TestH264EncoderProducesDecodableStream encodes a few synthetic frames with
// the real Media Foundation encoder: the first access unit and every forced
// keyframe are IDRs with SPS and PPS, and the rest are P frames.
func TestH264EncoderProducesDecodableStream(t *testing.T) {
	enc, err := NewH264Encoder(VideoEncoderConfig{Width: 1280, Height: 720, FPS: 15, BitrateKbps: 2500, KeyframeEvery: 75})
	if errors.Is(err, ErrNotSupported) && os.Getenv("AULA_REQUIRE_H264") == "" {
		t.Skipf("no H.264 encoder: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()

	img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	var units [][]byte
	start := time.Now()
	for i := range 10 {
		// A moving bar, so P frames carry something.
		for y := range 720 {
			for x := range 1280 {
				c := color.RGBA{30, 30, 30, 255}
				if x/40 == i {
					c = color.RGBA{220, 200, 40, 255}
				}
				img.SetRGBA(x, y, c)
			}
		}
		au, err := enc.Encode(img, i == 6)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		units = append(units, au)
	}
	t.Logf("10 frames of 1280x720 in %v", time.Since(start))
	var idrs []int
	for i, au := range units {
		types := nalTypes(au)
		if slices.Contains(types, 5) {
			idrs = append(idrs, i)
			if !slices.Contains(types, 7) || !slices.Contains(types, 8) {
				t.Errorf("IDR at frame %d without SPS/PPS: NAL types %v", i, types)
			}
		}
	}
	if len(units[0]) == 0 || !slices.Contains(idrs, 0) {
		t.Fatalf("first access unit is not an IDR (low latency should emit it at once): IDRs at %v, sizes %d", idrs, len(units[0]))
	}
	if !slices.Contains(idrs, 6) {
		t.Errorf("forced keyframe missing: IDRs at %v", idrs)
	}
	if len(idrs) != 2 {
		t.Errorf("IDRs at %v, want only 0 and 6", idrs)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Encode(img, false); err == nil {
		t.Error("Encode after Close succeeded")
	}
}

func TestDisplaysHasPrimary(t *testing.T) {
	c, err := NewDisplayCapture()
	if err != nil {
		t.Fatal(err)
	}
	ds, err := c.Displays()
	if err != nil {
		t.Skipf("no monitor: %v", err)
	}
	primaries := 0
	for _, d := range ds {
		if d.Primary {
			primaries++
		}
	}
	if primaries != 1 {
		t.Fatalf("displays %+v: want exactly one primary", ds)
	}
	if _, err := c.Grab(t.Context(), len(ds)); err == nil {
		t.Error("Grab of a monitor that does not exist succeeded")
	}
}
