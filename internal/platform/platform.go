// Package platform hides every operating-system API (screen capture, clipboard,
// overlay, service, hotkey) behind Go interfaces. Implementations live in
// *_windows.go, *_linux.go and *_darwin.go files selected by build constraints;
// the default implementation returns ErrNotSupported.
package platform

import "errors"

// ErrNotSupported is returned by features not implemented on the current OS.
var ErrNotSupported = errors.New("platform: not supported on this operating system")
