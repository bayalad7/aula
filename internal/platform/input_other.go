//go:build !windows

package platform

// Input injection is only implemented on Windows for now. Linux (XTest on
// X11; Wayland needs the RemoteDesktop portal) and macOS (Accessibility)
// are documented in docs/protocol.md §5.16.1.

func newInputInjector() (InputInjector, error) { return nil, ErrNotSupported }
