package platform

import (
	"fmt"
	"time"
	"unsafe"
)

// Hold and Release of the main window (AppWindow), for the teacher's
// broadcast: on top, without minimize nor close, and, in full screen,
// borderless over its monitor. Release puts back what Hold found.

var (
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procGetSystemMenu     = user32.NewProc("GetSystemMenu")
	procEnableMenuItem    = user32.NewProc("EnableMenuItem")
	procIsIconic          = user32.NewProc("IsIconic")
)

const (
	gwlStyle        = ^uintptr(15) // GWL_STYLE (-16)
	wsCaption       = 0x00C00000
	wsSysMenu       = 0x00080000
	wsThickFrame    = 0x00040000
	wsMinimizeBox   = 0x00020000
	wsMaximizeBox   = 0x00010000
	swpFrameChanged = 0x0020
	swpNoOwnerZOrd  = 0x0200
	hwndNoTopmost   = ^uintptr(1) // (HWND)-2
	scClose         = 0xF060
	mfByCommand     = 0x0000
	mfEnabled       = 0x0000
	mfGrayed        = 0x0001
	swRestore       = 9

	// holdCheck is how often a held window that was minimized anyway (Show
	// desktop) is brought back.
	holdCheck = 500 * time.Millisecond
)

// held is what the first Hold found.
type held struct {
	placement windowPlacement
	style     uintptr
	topmost   bool
	stop      chan struct{} // ends the watcher
}

func (w *winAppWindow) Hold(mode HoldMode) error {
	if mode != HoldWindow && mode != HoldFullscreen {
		return fmt.Errorf("hold: mode %d", mode)
	}
	hwnd, err := mainWindow()
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.held == nil {
		h := &held{stop: make(chan struct{})}
		h.placement.length = uint32(unsafe.Sizeof(h.placement))
		if r, _, err := procGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&h.placement))); r == 0 {
			return fmt.Errorf("hold: placement: %w", err)
		}
		h.style, _, _ = procGetWindowLongPtrW.Call(hwnd, gwlStyle)
		ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
		h.topmost = ex&wsExTopmost != 0
		w.held = h
		go w.watch(hwnd, h.stop)
	} else {
		// Another mode: start again from what the window was.
		w.restore(hwnd, w.held, false)
	}
	base := w.held.style
	if mode == HoldWindow {
		procSetWindowLongPtrW.Call(hwnd, gwlStyle, base&^wsMinimizeBox) //nolint:errcheck // returns the previous style
		if p := w.held.placement; p.showCmd == swShowMinimized {
			procShowWindow.Call(hwnd, swRestore) //nolint:errcheck // returns the previous visibility
		}
		if r, _, err := procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0,
			swpNoMove|swpNoSize|swpFrameChanged|swpShowWindow); r == 0 {
			return fmt.Errorf("hold: on top: %w", err)
		}
		enableClose(hwnd, false)
		return nil
	}
	// Full screen: the window's monitor, borderless. A maximized window is
	// restored first so that its own maximized size does not come back.
	if p := w.held.placement; p.showCmd == swShowMaximized || p.showCmd == swShowMinimized {
		procShowWindow.Call(hwnd, swRestore) //nolint:errcheck // returns the previous visibility
	}
	style := base&^(wsCaption|wsThickFrame|wsMinimizeBox|wsMaximizeBox|wsSysMenu) | wsPopup
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, style) //nolint:errcheck // returns the previous style
	mon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearst)
	mi := monitorInfoApp{}
	mi.size = uint32(unsafe.Sizeof(mi))
	if r, _, err := procGetMonitorInfoWApp.Call(mon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return fmt.Errorf("hold: monitor: %w", err)
	}
	m := mi.monitor
	if r, _, err := procSetWindowPos.Call(hwnd, hwndTopmost, uintptr(m.left), uintptr(m.top),
		uintptr(m.right-m.left), uintptr(m.bottom-m.top), swpFrameChanged|swpShowWindow); r == 0 {
		return fmt.Errorf("hold: full screen: %w", err)
	}
	enableClose(hwnd, false)
	return nil
}

func (w *winAppWindow) Release() error {
	hwnd, err := mainWindow()
	w.mu.Lock()
	defer w.mu.Unlock()
	h := w.held
	if h == nil {
		return nil
	}
	w.held = nil
	close(h.stop)
	if err != nil {
		return err
	}
	return w.restore(hwnd, h, true)
}

// restore puts back the style, the close command, the placement and, with
// zorder, the z-order that Hold found. w.mu is held.
func (w *winAppWindow) restore(hwnd uintptr, h *held, zorder bool) error {
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, h.style) //nolint:errcheck // returns the previous style
	enableClose(hwnd, true)
	p := h.placement
	if r, _, err := procSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&p))); r == 0 {
		return fmt.Errorf("release: placement: %w", err)
	}
	after := hwndNoTopmost
	if h.topmost || !zorder {
		after = hwndTopmost
	}
	if r, _, err := procSetWindowPos.Call(hwnd, after, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate|swpFrameChanged|swpNoOwnerZOrd); r == 0 {
		return fmt.Errorf("release: z-order: %w", err)
	}
	return nil
}

// watch brings a held window back if something minimized it anyway (Show
// desktop, Windows+D), until stop.
func (w *winAppWindow) watch(hwnd uintptr, stop chan struct{}) {
	t := time.NewTicker(holdCheck)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
				procShowWindow.Call(hwnd, swRestore)                 //nolint:errcheck // returns the previous visibility
				procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, //nolint:errcheck // best effort
					swpNoMove|swpNoSize|swpShowWindow)
			}
		}
	}
}

// enableClose grays out, or enables, the window's close button and its
// system menu entry. The app also refuses the close itself.
func enableClose(hwnd uintptr, on bool) {
	menu, _, _ := procGetSystemMenu.Call(hwnd, 0)
	if menu == 0 {
		return
	}
	flag := uintptr(mfByCommand | mfGrayed)
	if on {
		flag = mfByCommand | mfEnabled
	}
	procEnableMenuItem.Call(menu, scClose, flag) //nolint:errcheck // returns the previous state
}
