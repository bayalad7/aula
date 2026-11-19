//go:build !windows && !linux

package platform

func newNetworkInspector() (NetworkInspector, error) { return nil, ErrNotSupported }
