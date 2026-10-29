//go:build windows

package platform

import (
	"context"
	"fmt"
	"image"
	"unsafe"

	"github.com/kbinani/screenshot"
	"golang.org/x/sys/windows"
)

var (
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")
	procEnumDisplayMonitors      = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")
)

const monitorInfoPrimary = 0x1 // MONITORINFOF_PRIMARY

type monitorInfo struct {
	size    uint32
	monitor windows.Rect
	work    windows.Rect
	flags   uint32
}

const (
	wdaNone               = 0x00
	wdaMonitor            = 0x01 // painted black in captures (Windows 7+)
	wdaExcludeFromCapture = 0x11 // removed from captures (Windows 10 2004+)
)

func newDisplayCapture() (DisplayCapture, error) { return winDisplays{}, nil }

type winDisplays struct{}

// Displays lists the active monitors in the order Windows enumerates them.
// The console is per-monitor DPI aware, so sizes are physical. (Our own
// enumeration: kbinani/screenshot's fails Go's checkptr under -race.)
func (winDisplays) Displays() ([]Display, error) {
	var handles []uintptr
	cb := windows.NewCallback(func(hmon, _, _, _ uintptr) uintptr {
		handles = append(handles, hmon)
		return 1 // continue
	})
	if r, _, err := procEnumDisplayMonitors.Call(0, 0, cb, 0); r == 0 {
		return nil, fmt.Errorf("displays: %w", err)
	}
	out := make([]Display, 0, len(handles))
	for _, h := range handles {
		mi := monitorInfo{size: uint32(unsafe.Sizeof(monitorInfo{}))}
		if r, _, _ := procGetMonitorInfoW.Call(h, uintptr(unsafe.Pointer(&mi))); r == 0 {
			continue
		}
		b := image.Rect(int(mi.monitor.Left), int(mi.monitor.Top), int(mi.monitor.Right), int(mi.monitor.Bottom))
		out = append(out, Display{Index: len(out), Bounds: b, Primary: mi.flags&monitorInfoPrimary != 0})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("displays: no active monitor")
	}
	return out, nil
}

func (d winDisplays) Grab(ctx context.Context, index int) (*image.RGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ds, err := d.Displays()
	if err != nil {
		return nil, fmt.Errorf("grab: %w", err)
	}
	if index < 0 || index >= len(ds) {
		return nil, fmt.Errorf("grab: monitor %d does not exist", index)
	}
	img, err := screenshot.CaptureRect(ds[index].Bounds)
	if err != nil {
		// Typical when the session is locked or on the secure desktop.
		return nil, fmt.Errorf("grab: %w", err)
	}
	return img, nil
}

func excludeFromCapture(exclude bool) error {
	pid := windows.GetCurrentProcessId()
	var hwnds []uintptr
	cb := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		var owner uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&owner))) //nolint:errcheck // returns a thread ID
		if owner == pid {
			hwnds = append(hwnds, hwnd)
		}
		return 1 // continue
	})
	procEnumWindows.Call(cb, 0) //nolint:errcheck // the callback never stops it
	var firstErr error
	for _, hwnd := range hwnds {
		if err := setAffinity(hwnd, exclude); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func setAffinity(hwnd uintptr, exclude bool) error {
	if !exclude {
		if r, _, err := procSetWindowDisplayAffinity.Call(hwnd, wdaNone); r == 0 {
			return fmt.Errorf("display affinity: %w", err)
		}
		return nil
	}
	if r, _, _ := procSetWindowDisplayAffinity.Call(hwnd, wdaExcludeFromCapture); r != 0 {
		return nil
	}
	if r, _, err := procSetWindowDisplayAffinity.Call(hwnd, wdaMonitor); r == 0 {
		return fmt.Errorf("display affinity: %w", err)
	}
	return nil
}
