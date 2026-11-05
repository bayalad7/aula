//go:build !windows

package platform

func newAppWindow() (AppWindow, error) { return nil, ErrNotSupported }
