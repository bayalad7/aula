//go:build windows

package platform

import (
	"context"
	"fmt"
	"image"
	"sync"

	"github.com/kbinani/screenshot"
)

func newScreenCapture() (ScreenCapture, error) { return winCapture{}, nil }

type winCapture struct{}

// Capture grabs the primary display. Windows always places it at (0, 0) of the
// virtual screen; the agent is per-monitor DPI aware, so sizes are physical.
func (winCapture) Capture(ctx context.Context, maxWidth, quality int) (Frame, error) {
	if err := ctx.Err(); err != nil {
		return Frame{}, err
	}
	w, h := systemMetric(smCxScreen), systemMetric(smCyScreen)
	if w <= 0 || h <= 0 {
		return Frame{}, fmt.Errorf("capture: primary display has no size")
	}
	img, err := screenshot.CaptureRect(image.Rect(0, 0, w, h))
	if err != nil {
		// Typical when the session is locked or on the secure desktop.
		return Frame{}, fmt.Errorf("capture: %w", err)
	}
	return EncodeFrame(img, maxWidth, quality)
}

func newScreenIndicator() (ScreenIndicator, error) { return &winIndicator{}, nil }

// winIndicator is a small click-through topmost banner at the top centre of
// the primary display.
type winIndicator struct {
	mu  sync.Mutex
	win *window
}

func (i *winIndicator) Show(text string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.win != nil {
		i.win.setText(text)
		return nil
	}
	w, err := openWindow(text)
	if err != nil {
		return fmt.Errorf("screen indicator: %w", err)
	}
	i.win = w
	return nil
}

func (i *winIndicator) Hide() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.win != nil {
		i.win.close()
		i.win = nil
	}
	return nil
}
