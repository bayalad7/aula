//go:build !windows

package platform

func newNetworkInspector() (NetworkInspector, error) { return nil, ErrNotSupported }
