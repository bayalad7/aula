//go:build !windows

package platform

// The class restrictions are only implemented on Windows for now.

func newProcessControl() (ProcessControl, error) { return nil, ErrNotSupported }
func newDNSRedirect() (DNSRedirect, error)       { return nil, ErrNotSupported }
func newBrowserDoH() (BrowserDoH, error)         { return nil, ErrNotSupported }
