package platform

import (
	"context"
	"fmt"
	"image"
	"runtime"
	"sync"
)

// Display is one monitor of the teacher's machine.
type Display struct {
	Index   int             // 0, 1, 2… as DisplayCapture.Grab expects it
	Bounds  image.Rectangle // in virtual-screen pixels (physical, not scaled)
	Primary bool
}

// DisplayCapture grabs whole monitors continuously, for the broadcast of the
// teacher's screen (docs/protocol.md §5.12). Frames live only in memory.
type DisplayCapture interface {
	// Displays lists the active monitors.
	Displays() ([]Display, error)
	// Grab captures monitor index at its native resolution. It is called up
	// to fps times per second; the image may be reused by the next call.
	Grab(ctx context.Context, index int) (*image.RGBA, error)
}

// VideoEncoderConfig sets up an H.264 encoder.
type VideoEncoderConfig struct {
	Width, Height int // even, at most 4096
	FPS           int
	BitrateKbps   int // peak; the mean is about 60 % of it
	KeyframeEvery int // frames between periodic keyframes (GOP)
}

// VideoEncoder compresses frames to H.264 Constrained Baseline.
type VideoEncoder interface {
	// Encode compresses one Width×Height frame and returns its access unit
	// in Annex-B format (start codes), with SPS and PPS before every IDR. It
	// returns nil while the encoder buffers. forceKey asks for an IDR.
	Encode(img *image.RGBA, forceKey bool) ([]byte, error)
	Close() error
}

// NewDisplayCapture returns the OS monitor capture: Windows today; elsewhere
// it fails with ErrNotSupported and the console cannot broadcast.
func NewDisplayCapture() (DisplayCapture, error) { return newDisplayCapture() }

// NewH264Encoder returns the OS H.264 encoder (Media Foundation on Windows),
// or ErrNotSupported.
func NewH264Encoder(cfg VideoEncoderConfig) (VideoEncoder, error) {
	if cfg.Width < 2 || cfg.Height < 2 || cfg.Width%2 != 0 || cfg.Height%2 != 0 || cfg.Width > 4096 || cfg.Height > 4096 {
		return nil, fmt.Errorf("h264 encoder: frame size %dx%d must be even and at most 4096", cfg.Width, cfg.Height)
	}
	if cfg.FPS < 1 || cfg.BitrateKbps < 1 || cfg.KeyframeEvery < 1 {
		return nil, fmt.Errorf("h264 encoder: fps, bitrate and keyframe interval are required")
	}
	return newH264Encoder(cfg)
}

// ExcludeFromCapture hides (or shows again) every top-level window of this
// process from screen captures, so the console never appears in its own
// broadcast (docs/privacidad.md rule 15). Windows 10 2004+ removes the windows
// from the capture; older versions paint them black.
func ExcludeFromCapture(exclude bool) error { return excludeFromCapture(exclude) }

// RGBAToNV12 converts img to NV12 (BT.601, limited range) into dst, which
// must hold w*h*3/2 bytes for even w and h. It is the OS-independent half of
// the encoders, split by rows across the CPUs.
func RGBAToNV12(img *image.RGBA, dst []byte) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w%2 != 0 || h%2 != 0 || len(dst) < w*h*3/2 {
		return fmt.Errorf("nv12: frame %dx%d does not fit %d bytes or is odd", w, h, len(dst))
	}
	workers := min(runtime.GOMAXPROCS(0), h/2)
	rowsPer := (h/2 + workers - 1) / workers // pairs of rows per worker
	var wg sync.WaitGroup
	for start := 0; start < h/2; start += rowsPer {
		end := min(start+rowsPer, h/2)
		wg.Add(1)
		go func() {
			defer wg.Done()
			nv12Rows(img, dst, w, h, start, end)
		}()
	}
	wg.Wait()
	return nil
}

// nv12Rows converts the row pairs [from, to): luma for both rows and one row
// of interleaved chroma averaged over each 2×2 block. The rows are resliced
// to their exact length so the compiler drops the bounds checks.
func nv12Rows(img *image.RGBA, dst []byte, w, h, from, to int) {
	luma, chroma := dst[:w*h], dst[w*h:]
	for pair := from; pair < to; pair++ {
		y0 := pair * 2
		r0 := img.Pix[y0*img.Stride : y0*img.Stride+w*4]
		r1 := img.Pix[(y0+1)*img.Stride : (y0+1)*img.Stride+w*4]
		l0 := luma[y0*w : y0*w+w]
		l1 := luma[(y0+1)*w : (y0+1)*w+w]
		c := chroma[pair*w : pair*w+w]
		for x := 0; x+1 < len(l0) && x*4+7 < len(r0) && x*4+7 < len(r1) && x+1 < len(l1) && x+1 < len(c); x += 2 {
			i := x * 4
			ra, ga, ba := int32(r0[i]), int32(r0[i+1]), int32(r0[i+2])
			rb, gb, bb := int32(r0[i+4]), int32(r0[i+5]), int32(r0[i+6])
			rc, gc, bc := int32(r1[i]), int32(r1[i+1]), int32(r1[i+2])
			rd, gd, bd := int32(r1[i+4]), int32(r1[i+5]), int32(r1[i+6])
			l0[x] = luma601(ra, ga, ba)
			l0[x+1] = luma601(rb, gb, bb)
			l1[x] = luma601(rc, gc, bc)
			l1[x+1] = luma601(rd, gd, bd)
			r, g, b := (ra+rb+rc+rd)>>2, (ga+gb+gc+gd)>>2, (ba+bb+bc+bd)>>2
			c[x] = uint8((-38*r - 74*g + 112*b + 128 + 128<<8) >> 8)
			c[x+1] = uint8((112*r - 94*g - 18*b + 128 + 128<<8) >> 8)
		}
	}
}

func luma601(r, g, b int32) uint8 {
	return uint8((66*r + 129*g + 25*b + 128 + 16<<8) >> 8)
}
