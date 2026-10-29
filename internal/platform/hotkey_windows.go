//go:build windows

package platform

import (
	"context"
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
)

const (
	wmQuit   = 0x0012
	wmHotkey = 0x0312

	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000

	errorHotkeyAlreadyRegistered = windows.Errno(1409)
)

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
	private uint32
}

// registerHotkey owns one OS thread with a message loop: RegisterHotKey
// delivers WM_HOTKEY to the thread that registered it.
func registerHotkey(ctx context.Context, h Hotkey, fn func()) error {
	var mods uintptr = modNoRepeat
	if h.Mods&ModCtrl != 0 {
		mods |= modControl
	}
	if h.Mods&ModAlt != 0 {
		mods |= modAlt
	}
	if h.Mods&ModShift != 0 {
		mods |= modShift
	}
	if h.Mods&ModWin != 0 {
		mods |= modWin
	}
	vk := uintptr(h.Key) // virtual-key codes of A-Z and 0-9 are their ASCII codes

	errc := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		const id = 1
		if r, _, err := procRegisterHotKey.Call(0, id, mods, vk); r == 0 {
			if errors.Is(err, errorHotkeyAlreadyRegistered) {
				err = ErrHotkeyInUse
			}
			errc <- err
			return
		}
		defer procUnregisterHotKey.Call(0, id) //nolint:errcheck // best effort on shutdown
		tid := windows.GetCurrentThreadId()
		stop := context.AfterFunc(ctx, func() { procPostThreadMessageW.Call(uintptr(tid), wmQuit, 0, 0) })
		defer stop()
		errc <- nil

		var m winMsg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 { // WM_QUIT or error
				return
			}
			if m.message == wmHotkey {
				go fn()
			}
		}
	}()
	return <-errc
}
