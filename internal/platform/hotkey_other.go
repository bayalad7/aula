//go:build !windows

package platform

import "context"

// Global hotkeys are only implemented on Windows for now (priority platform).
func registerHotkey(context.Context, Hotkey, func()) error { return ErrNotSupported }
