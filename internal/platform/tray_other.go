//go:build !windows

package platform

func newTray(TrayOptions) (Tray, error) { return nil, ErrNotSupported }
