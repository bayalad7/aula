//go:build linux

package platform

import (
	"context"
	"fmt"
	"image"

	"github.com/kbinani/screenshot"
)

// Broadcast capture on Linux (docs/privacidad.md rule 106): whole monitors
// of an X11 session. In Wayland it fails with ErrNotSupported, so the
// console does not offer to broadcast and the agent does not declare
// screen.present.

func newDisplayCapture() (DisplayCapture, error) {
	if err := needX11(); err != nil {
		return nil, err
	}
	return x11Displays{}, nil
}

type x11Displays struct{}

// Displays lists Xinerama's screens; the first is the primary one, as
// kbinani/screenshot and the "screen viewed" notice count them.
func (x11Displays) Displays() ([]Display, error) {
	c, sc, err := x11Connect()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	var out []Display
	for i, r := range monitors(c, sc) {
		out = append(out, Display{Index: i, Bounds: r, Primary: i == 0})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("displays: none")
	}
	return out, nil
}

func (x11Displays) Grab(ctx context.Context, index int) (*image.RGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := needX11(); err != nil {
		return nil, fmt.Errorf("grab: %w", err)
	}
	if index < 0 || index >= screenshot.NumActiveDisplays() {
		return nil, fmt.Errorf("grab: no monitor %d", index)
	}
	img, err := screenshot.CaptureDisplay(index)
	if err != nil {
		return nil, fmt.Errorf("grab: %w", err)
	}
	return img, nil
}

// excludeFromCapture: X11 has no way to leave a window out of a capture.
// It does nothing and does not fail, so broadcasting and presenting go on;
// the console says that its window is seen (rule 106, CanExcludeFromCapture).
func excludeFromCapture(bool) error { return nil }

func canExcludeFromCapture() bool { return false }
