//go:build !windows

package platform

// Launcher is only implemented on Windows for now.

func newLauncher() (Launcher, error) { return nil, ErrNotSupported }
