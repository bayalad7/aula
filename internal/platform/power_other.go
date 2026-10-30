//go:build !windows

package platform

// Power is only implemented on Windows for now.

func newPower() (Power, error)        { return nil, ErrNotSupported }
func currentSession() (uint32, error) { return 0, ErrNotSupported }
