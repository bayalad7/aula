package platform

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"

	"golang.org/x/image/draw"
)

// Capture sizes used by the console (docs/protocol.md §5.6).
const (
	ThumbnailWidth = 320  // the grid of thumbnails
	DetailWidth    = 1280 // one computer seen in detail
)

// Frame is one capture of the screen, encoded as JPEG.
type Frame struct {
	JPEG          []byte
	Width, Height int
}

// ScreenCapture grabs the student's primary display. Captures live only in
// memory: no implementation writes them to disk (docs/privacidad.md).
type ScreenCapture interface {
	// Capture returns the primary display scaled down to at most maxWidth
	// pixels wide (never enlarged), encoded as JPEG with quality 1-100. Use
	// ThumbnailWidth for the grid and DetailWidth for the detailed view.
	Capture(ctx context.Context, maxWidth, quality int) (Frame, error)
}

// ScreenIndicator is the always-visible notice "your screen is being viewed"
// (docs/privacidad.md rule 6): a small window on top of every other one, on
// the primary display, that never takes clicks or focus.
type ScreenIndicator interface {
	// Show displays the notice with the given text; if already shown, it
	// only replaces the text.
	Show(text string) error
	// Hide removes it. Idempotent.
	Hide() error
}

// NewScreenCapture returns the OS screen capture: Windows today; elsewhere it
// fails with ErrNotSupported, and the agent does not declare screen.thumb.
func NewScreenCapture() (ScreenCapture, error) { return newScreenCapture() }

// NewScreenIndicator returns the OS "screen viewed" notice, or ErrNotSupported.
func NewScreenIndicator() (ScreenIndicator, error) { return newScreenIndicator() }

// EncodeFrame scales img down to at most maxWidth pixels wide, keeping its
// proportions, and encodes it as JPEG. It is the OS-independent half of
// Capture.
func EncodeFrame(img image.Image, maxWidth, quality int) (Frame, error) {
	b := img.Bounds()
	if b.Empty() {
		return Frame{}, fmt.Errorf("encode frame: empty image")
	}
	if maxWidth < 1 {
		return Frame{}, fmt.Errorf("encode frame: max width %d", maxWidth)
	}
	quality = min(max(quality, 1), 100)
	w, h := b.Dx(), b.Dy()
	if w > maxWidth {
		h = max(1, h*maxWidth/w)
		w = maxWidth
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		// BiLinear averages the source pixels under each output pixel, which
		// keeps text legible at a fraction of the cost of CatmullRom.
		draw.BiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return Frame{}, fmt.Errorf("encode frame: %w", err)
	}
	return Frame{JPEG: buf.Bytes(), Width: w, Height: h}, nil
}
