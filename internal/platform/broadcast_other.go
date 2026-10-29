//go:build !windows

package platform

// The broadcast of the teacher's screen is only implemented on Windows for
// now (priority platform).

func newDisplayCapture() (DisplayCapture, error) { return nil, ErrNotSupported }

func excludeFromCapture(bool) error { return ErrNotSupported }
