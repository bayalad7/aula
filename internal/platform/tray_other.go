//go:build !windows && !linux

package platform

func newTray(TrayOptions) (Tray, error) { return nil, ErrNotSupported }
