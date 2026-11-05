package platform

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The notification area icon (prompt 19): a hidden window of its own, on a
// locked OS thread with its own message loop, receives the icon's clicks.
// It is a hidden top-level window and not a message-only one: those do not
// get the "TaskbarCreated" broadcast (Explorer restarted) and cannot take the
// foreground for the menu.

var (
	shell32                      = windows.NewLazySystemDLL("shell32.dll")
	procShellNotifyIconW         = shell32.NewProc("Shell_NotifyIconW")
	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon              = user32.NewProc("DestroyIcon")
	procCreatePopupMenu          = user32.NewProc("CreatePopupMenu")
	procAppendMenuW              = user32.NewProc("AppendMenuW")
	procTrackPopupMenu           = user32.NewProc("TrackPopupMenu")
	procDestroyMenu              = user32.NewProc("DestroyMenu")
	procSetForegroundWindowTray  = user32.NewProc("SetForegroundWindow")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procRegisterWindowMessageW   = user32.NewProc("RegisterWindowMessageW")
)

const (
	nimAdd             = 0
	nimModify          = 1
	nimDelete          = 2
	nimSetVersion      = 4
	nifMessage         = 0x01
	nifIcon            = 0x02
	nifTip             = 0x04
	nifShowTip         = 0x80
	notifyIconVersion4 = 4

	wmTrayCallback = 0x8010 // WM_APP+0x10: the icon's events
	wmTrayClose    = 0x8011 // WM_APP+0x11: remove the icon, posted from another goroutine
	wmContextMenu  = 0x007B
	wmNull         = 0x0000
	ninSelect      = 0x0400
	ninKeySelect   = 0x0401

	tpmReturnCmd   = 0x0100
	tpmRightButton = 0x0002
	tpmBottomAlign = 0x0020
	mfString       = 0x0000
	menuOpen       = 1

	smCxSmIcon     = 49
	lrDefaultColor = 0
	iconVersion    = 0x00030000
)

// notifyIconData is NOTIFYICONDATAW (Windows Vista and later).
type notifyIconData struct {
	size            uint32
	hwnd            uintptr
	id              uint32
	flags           uint32
	callbackMessage uint32
	icon            uintptr
	tip             [128]uint16
	state           uint32
	stateMask       uint32
	info            [256]uint16
	version         uint32 // union with uTimeout
	infoTitle       [64]uint16
	infoFlags       uint32
	guidItem        windows.GUID
	balloonIcon     uintptr
}

type point struct{ x, y int32 }

const trayClass = "AulaTrayWindow"

var (
	trayClassOnce sync.Once
	trayClassErr  error
	currentTray   atomic.Pointer[winTray] // one icon per process
	taskbarMsg    uintptr                 // "TaskbarCreated"
)

type winTray struct {
	o    TrayOptions
	done chan struct{}

	mu             sync.Mutex
	hwnd           uintptr
	nid            notifyIconData
	normal, marked uintptr // HICON
	closed         bool
}

func registerTrayClass() error {
	trayClassOnce.Do(func() {
		if err := registerClass(); err != nil { // sets instance
			trayClassErr = err
			return
		}
		name, _ := windows.UTF16PtrFromString(trayClass)
		wc := wndClassEx{wndProc: syscall.NewCallback(trayWndProc), instance: instance, className: name}
		wc.size = uint32(unsafe.Sizeof(wc))
		if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 && err != errClassAlreadyExists {
			trayClassErr = fmt.Errorf("RegisterClassEx: %w", err)
		}
		msg, _ := windows.UTF16PtrFromString("TaskbarCreated")
		taskbarMsg, _, _ = procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(msg)))
	})
	return trayClassErr
}

func newTray(o TrayOptions) (Tray, error) {
	if len(o.Icon) == 0 || len(o.MarkedIcon) == 0 {
		return nil, errors.New("tray: the icons are required")
	}
	if err := registerTrayClass(); err != nil {
		return nil, err
	}
	t := &winTray{o: o, done: make(chan struct{})}
	if !currentTray.CompareAndSwap(nil, t) {
		return nil, errors.New("tray: this process already has an icon")
	}
	errc := make(chan error, 1)
	go t.run(errc)
	if err := <-errc; err != nil {
		currentTray.CompareAndSwap(t, nil)
		<-t.done
		return nil, err
	}
	return t, nil
}

// icon makes an HICON of the system's small-icon size from PNG data.
func icon(png []byte) (uintptr, error) {
	size := systemMetric(smCxSmIcon)
	h, _, err := procCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)), 1, iconVersion,
		uintptr(size), uintptr(size), lrDefaultColor)
	if h == 0 {
		return 0, fmt.Errorf("tray icon: %w", err)
	}
	return h, nil
}

func (t *winTray) run(errc chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(t.done)

	fail := func(err error) {
		t.destroyIcons()
		errc <- err
	}
	var err error
	if t.normal, err = icon(t.o.Icon); err != nil {
		fail(err)
		return
	}
	if t.marked, err = icon(t.o.MarkedIcon); err != nil {
		fail(err)
		return
	}
	class, _ := windows.UTF16PtrFromString(trayClass)
	title, _ := windows.UTF16PtrFromString("Aula")
	hwnd, _, cerr := procCreateWindowExW.Call(wsExToolWindow, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		wsPopup, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		fail(fmt.Errorf("tray window: %w", cerr))
		return
	}
	t.mu.Lock()
	t.hwnd = hwnd
	t.nid = notifyIconData{hwnd: hwnd, id: 1, flags: nifMessage | nifIcon | nifTip | nifShowTip,
		callbackMessage: wmTrayCallback, icon: t.normal, version: notifyIconVersion4}
	t.nid.size = uint32(unsafe.Sizeof(t.nid))
	setTip(&t.nid, t.o.Tooltip)
	err = t.addLocked()
	t.mu.Unlock()
	if err != nil {
		procDestroyWindow.Call(hwnd) //nolint:errcheck // already failing
		fail(err)
		return
	}
	errc <- nil

	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// addLocked adds the icon and asks for the version 4 behavior. t.mu held.
func (t *winTray) addLocked() error {
	if r, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&t.nid))); r == 0 {
		return fmt.Errorf("tray: Shell_NotifyIcon: %w", err)
	}
	procShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&t.nid))) //nolint:errcheck // older shells keep version 0
	return nil
}

func setTip(n *notifyIconData, text string) {
	u, _ := windows.UTF16FromString(text)
	if len(u) > len(n.tip) {
		u = u[:len(n.tip)]
		u[len(u)-1] = 0
	}
	n.tip = [128]uint16{}
	copy(n.tip[:], u)
}

func (t *winTray) modify(change func(*notifyIconData)) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	change(&t.nid)
	if r, _, err := procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&t.nid))); r == 0 {
		return fmt.Errorf("tray: Shell_NotifyIcon: %w", err)
	}
	return nil
}

func (t *winTray) SetTooltip(text string) error {
	return t.modify(func(n *notifyIconData) { setTip(n, text) })
}

func (t *winTray) SetMarked(marked bool) error {
	return t.modify(func(n *notifyIconData) {
		n.icon = t.normal
		if marked {
			n.icon = t.marked
		}
	})
}

func (t *winTray) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	hwnd := t.hwnd
	t.mu.Unlock()
	procPostMessageW.Call(hwnd, wmTrayClose, 0, 0) //nolint:errcheck // the window outlives this call
	<-t.done
	t.destroyIcons()
	currentTray.CompareAndSwap(t, nil)
	return nil
}

func (t *winTray) destroyIcons() {
	for _, h := range []uintptr{t.normal, t.marked} {
		if h != 0 {
			procDestroyIcon.Call(h) //nolint:errcheck // nothing to do if it fails
		}
	}
	t.normal, t.marked = 0, 0
}

// menu shows the icon's menu at the pointer and opens the window if chosen.
func (t *winTray) menu(hwnd uintptr) {
	m, _, _ := procCreatePopupMenu.Call()
	if m == 0 {
		return
	}
	defer procDestroyMenu.Call(m) //nolint:errcheck // nothing to do if it fails
	label, _ := windows.UTF16PtrFromString(t.o.OpenLabel)
	procAppendMenuW.Call(m, mfString, menuOpen, uintptr(unsafe.Pointer(label))) //nolint:errcheck // an empty menu just closes
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p))) //nolint:errcheck // (0,0) is still usable
	// Required so the menu closes when the user clicks elsewhere.
	procSetForegroundWindowTray.Call(hwnd) //nolint:errcheck // best effort
	cmd, _, _ := procTrackPopupMenu.Call(m, tpmReturnCmd|tpmRightButton|tpmBottomAlign, uintptr(p.x), uintptr(p.y), 0, hwnd, 0)
	procPostMessageW.Call(hwnd, wmNull, 0, 0) //nolint:errcheck // documented workaround
	if cmd == menuOpen && t.o.OnOpen != nil {
		go t.o.OnOpen()
	}
}

func trayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	t := currentTray.Load()
	if t == nil || t.hwnd != hwnd {
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	}
	switch {
	case msg == wmTrayCallback:
		switch lParam & 0xFFFF { // version 4: the event is in the low word
		case ninSelect, ninKeySelect:
			if t.o.OnOpen != nil {
				go t.o.OnOpen()
			}
		case wmContextMenu:
			t.menu(hwnd)
		}
		return 0
	case taskbarMsg != 0 && msg == taskbarMsg: // Explorer restarted: the icon is gone
		t.mu.Lock()
		if !t.closed {
			_ = t.addLocked()
		}
		t.mu.Unlock()
		return 0
	case msg == wmTrayClose:
		t.mu.Lock()
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&t.nid))) //nolint:errcheck // gone already is fine
		t.mu.Unlock()
		procDestroyWindow.Call(hwnd) //nolint:errcheck // the loop ends either way
		return 0
	case msg == wmDestroy:
		procPostQuitMessage.Call(0) //nolint:errcheck // no result
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}
