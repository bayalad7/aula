//go:build !windows && !linux

package platform

import "context"

// Global hotkeys are implemented on Windows and Linux (X11) only for now.
func registerHotkey(context.Context, Hotkey, func()) error { return ErrNotSupported }
