//go:build !windows

package platform

// Interfaces is only implemented on Windows for now (prompt 10.19).

func interfaces() ([]NIC, error) { return nil, ErrNotSupported }
