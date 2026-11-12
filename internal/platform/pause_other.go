//go:build !windows

package platform

// The pause window is only implemented on Windows for now (priority
// platform); Linux and macOS do not declare screen.freeze.

func newPauseOverlay(PauseLook) (PauseOverlay, error) { return nil, ErrNotSupported }
