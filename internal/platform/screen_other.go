//go:build !windows

package platform

// Screen monitoring is only implemented on Windows for now (priority
// platform). Linux and macOS will add their own files with their notices
// documented first in docs/privacidad.md.

func newScreenCapture() (ScreenCapture, error)     { return nil, ErrNotSupported }
func newScreenIndicator() (ScreenIndicator, error) { return nil, ErrNotSupported }
