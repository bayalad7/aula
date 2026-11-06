//go:build !windows

package platform

func newWorkFolder() (WorkFolder, error) { return nil, ErrNotSupported }
