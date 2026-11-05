package platform

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procFlashWindowEx       = user32.NewProc("FlashWindowEx")
	procGetWindowLongPtrW   = user32.NewProc("GetWindowLongPtrW")
	procGetWindowPlacement  = user32.NewProc("GetWindowPlacement")
	procSetWindowPlacement  = user32.NewProc("SetWindowPlacement")
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	procGetMonitorInfoWApp  = user32.NewProc("GetMonitorInfoW")
	procGetWindow           = user32.NewProc("GetWindow")
	procEnumWindowsApp      = user32.NewProc("EnumWindows")
	procGetWindowThreadPIDA = user32.NewProc("GetWindowThreadProcessId")
)

const (
	gwlExStyle             = ^uintptr(19) // GWL_EXSTYLE (-20)
	gwOwner                = 4
	flashwTray             = 0x00000002
	flashwTimerNoFG        = 0x0000000C
	monitorDefaultToNearst = 2
	swpNoZOrder            = 0x0004
	swShowNormal           = 1
	swShowMinimized        = 2
	swShowMaximized        = 3
)

// flashWInfo is FLASHWINFO.
type flashWInfo struct {
	size    uint32
	hwnd    uintptr
	flags   uint32
	count   uint32
	timeout uint32
}

// windowPlacement is WINDOWPLACEMENT.
type windowPlacement struct {
	length         uint32
	flags          uint32
	showCmd        uint32
	minPosition    struct{ x, y int32 }
	maxPosition    struct{ x, y int32 }
	normalPosition rect
}

// monitorInfoApp is MONITORINFO.
type monitorInfoApp struct {
	size    uint32
	monitor rect
	work    rect
	flags   uint32
}

type winAppWindow struct {
	mu    sync.Mutex
	saved *windowPlacement // placement before Compact(true)
}

func newAppWindow() (AppWindow, error) {
	if _, err := mainWindow(); err != nil {
		return nil, err
	}
	return &winAppWindow{}, nil
}

// mainWindow is the visible top-level window of this process that has a
// taskbar button: not a tool window (the "screen viewed" indicator) and not
// owned by another window.
func mainWindow() (uintptr, error) {
	pid := windows.GetCurrentProcessId()
	var found uintptr
	cb := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		var owner uint32
		procGetWindowThreadPIDA.Call(hwnd, uintptr(unsafe.Pointer(&owner))) //nolint:errcheck // returns a thread ID
		if owner != pid {
			return 1
		}
		if ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle); ex&wsExToolWindow != 0 {
			return 1
		}
		if o, _, _ := procGetWindow.Call(hwnd, gwOwner); o != 0 {
			return 1
		}
		found = hwnd
		return 0 // stop: EnumWindows goes from the top of the z-order down
	})
	procEnumWindowsApp.Call(cb, 0) //nolint:errcheck // stopping early reports an error
	if found == 0 {
		return 0, errors.New("app window: no main window in this process")
	}
	return found, nil
}

func (w *winAppWindow) Flash() error {
	hwnd, err := mainWindow()
	if err != nil {
		return err
	}
	// The taskbar button only, until the window comes to the foreground.
	fi := flashWInfo{hwnd: hwnd, flags: flashwTray | flashwTimerNoFG}
	fi.size = uint32(unsafe.Sizeof(fi))
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&fi))) //nolint:errcheck // returns the previous state
	return nil
}

func (w *winAppWindow) Compact(on bool, width, height int) error {
	hwnd, err := mainWindow()
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !on {
		if w.saved == nil {
			return nil
		}
		p := *w.saved
		w.saved = nil
		if p.showCmd == swShowMinimized {
			p.showCmd = swShowNormal
		}
		if r, _, err := procSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&p))); r == 0 {
			return fmt.Errorf("compact off: %w", err)
		}
		return nil
	}
	if w.saved == nil {
		p := windowPlacement{}
		p.length = uint32(unsafe.Sizeof(p))
		if r, _, err := procGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&p))); r == 0 {
			return fmt.Errorf("compact: %w", err)
		}
		w.saved = &p
		if p.showCmd == swShowMaximized {
			procShowWindow.Call(hwnd, swShowNoActivate) //nolint:errcheck // returns the previous visibility
		}
	}
	mon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearst)
	mi := monitorInfoApp{}
	mi.size = uint32(unsafe.Sizeof(mi))
	if r, _, err := procGetMonitorInfoWApp.Call(mon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return fmt.Errorf("compact: monitor: %w", err)
	}
	scale := 1.0
	if dpi, _, _ := procGetDpiForWindow.Call(hwnd); dpi != 0 {
		scale = float64(dpi) / 96
	}
	cx, cy := int32(float64(width)*scale), int32(float64(height)*scale)
	margin := int32(12 * scale)
	x, y := mi.work.right-cx-margin, mi.work.bottom-cy-margin
	// Not on top and not activated: it is a small window like any other.
	if r, _, err := procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(cx), uintptr(cy),
		swpNoZOrder|swpNoActivate); r == 0 {
		return fmt.Errorf("compact: %w", err)
	}
	return nil
}
