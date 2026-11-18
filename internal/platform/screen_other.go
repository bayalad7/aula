//go:build !windows && !linux

package platform

// Screen monitoring is implemented on Windows and Linux (X11) only for now (priority
// platform). Linux and macOS will add their own files with their notices
// documented first in docs/privacidad.md.

func newScreenCapture() (ScreenCapture, error)     { return nil, ErrNotSupported }
func newScreenIndicator() (ScreenIndicator, error) { return nil, ErrNotSupported }
