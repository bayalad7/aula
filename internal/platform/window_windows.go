//go:build windows

package platform

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Native window of the "screen viewed" indicator (docs/privacidad.md rule 6).
// It owns an OS thread with its own message loop.

var (
	gdi32                          = windows.NewLazySystemDLL("gdi32.dll")
	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procFillRect                   = user32.NewProc("FillRect")
	procDrawTextW                  = user32.NewProc("DrawTextW")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procSetTimer                   = user32.NewProc("SetTimer")
	procKillTimer                  = user32.NewProc("KillTimer")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procLoadCursorW                = user32.NewProc("LoadCursorW")
	procGetDpiForSystem            = user32.NewProc("GetDpiForSystem")
	procCreateSolidBrush           = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW                = gdi32.NewProc("CreateFontW")
	procSelectObject               = gdi32.NewProc("SelectObject")
	procDeleteObject               = gdi32.NewProc("DeleteObject")
	procSetTextColor               = gdi32.NewProc("SetTextColor")
	procSetBkMode                  = gdi32.NewProc("SetBkMode")
)

const (
	smCxScreen = 0
	smCyScreen = 1

	wsPopup          = 0x80000000
	wsExTopmost      = 0x00000008
	wsExTransparent  = 0x00000020
	wsExToolWindow   = 0x00000080
	wsExLayered      = 0x00080000
	wsExNoActivate   = 0x08000000
	swShowNoActivate = 4
	swpNoSize        = 0x0001
	swpNoMove        = 0x0002
	swpNoActivate    = 0x0010
	swpShowWindow    = 0x0040
	hwndTopmost      = ^uintptr(0) // (HWND)-1
	lwaAlpha         = 0x2
	csHRedraw        = 0x2
	csVRedraw        = 0x1
	idcArrow         = 32512
	maNoActivate     = 3

	wmDestroy       = 0x0002
	wmPaint         = 0x000F
	wmEraseBkgnd    = 0x0014
	wmMouseActivate = 0x0021
	wmTimer         = 0x0113
	wmAppUpdate     = 0x8001 // WM_APP+1: the text changed
	wmAppClose      = 0x8002 // WM_APP+2: close, posted from another goroutine

	dtCenter      = 0x0001
	dtVCenter     = 0x0004
	dtSingleLine  = 0x0020
	dtNoPrefix    = 0x0800
	dtEndEllipsis = 0x8000
	transparentBk = 1
	fwSemiBold    = 600
	cleartype     = 5

	errClassAlreadyExists = windows.Errno(1410)
)

const windowClass = "AulaScreenWindow"

type window struct {
	hwnd uintptr
	done chan struct{}

	mu   sync.Mutex
	text string
}

type rect struct{ left, top, right, bottom int32 }

type paintStruct struct {
	hdc         uintptr
	erase       int32
	paint       rect
	restore     int32
	incUpdate   int32
	rgbReserved [32]byte
}

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

var (
	classOnce   sync.Once
	classErr    error
	instance    uintptr
	openWindows sync.Map // hwnd → *window
)

func registerClass() error {
	classOnce.Do(func() {
		var h windows.Handle
		if err := windows.GetModuleHandleEx(0, nil, &h); err != nil {
			classErr = fmt.Errorf("GetModuleHandleEx: %w", err)
			return
		}
		instance = uintptr(h)
		cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
		name, _ := windows.UTF16PtrFromString(windowClass)
		wc := wndClassEx{
			style:     csHRedraw | csVRedraw,
			wndProc:   syscall.NewCallback(wndProc),
			instance:  instance,
			cursor:    cursor,
			className: name,
		}
		wc.size = uint32(unsafe.Sizeof(wc))
		if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 && err != errClassAlreadyExists {
			classErr = fmt.Errorf("RegisterClassEx: %w", err)
		}
	})
	return classErr
}

// openWindow creates the window on its own locked OS thread and returns once
// it is on screen.
func openWindow(text string) (*window, error) {
	if err := registerClass(); err != nil {
		return nil, err
	}
	w := &window{text: text, done: make(chan struct{})}
	errc := make(chan error, 1)
	go w.run(errc)
	if err := <-errc; err != nil {
		return nil, err
	}
	return w, nil
}

func (w *window) run(errc chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(w.done)

	x, y, cx, cy := w.geometry()
	// Topmost, no taskbar button, never takes clicks or focus.
	ex := uintptr(wsExTopmost | wsExToolWindow | wsExNoActivate | wsExLayered | wsExTransparent)
	class, _ := windows.UTF16PtrFromString(windowClass)
	title, _ := windows.UTF16PtrFromString("Aula")
	hwnd, _, err := procCreateWindowExW.Call(ex, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		wsPopup, uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), 0, 0, instance, 0)
	if hwnd == 0 {
		errc <- fmt.Errorf("CreateWindowEx: %w", err)
		return
	}
	w.hwnd = hwnd
	openWindows.Store(hwnd, w)
	defer openWindows.Delete(hwnd)

	procSetLayeredWindowAttributes.Call(hwnd, 0, 235, lwaAlpha)
	procShowWindow.Call(hwnd, swShowNoActivate)
	w.keepOnTop()
	procSetTimer.Call(hwnd, 1, 2000, 0)
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

func (w *window) setText(t string) {
	w.mu.Lock()
	w.text = t
	w.mu.Unlock()
	procPostMessageW.Call(w.hwnd, wmAppUpdate, 0, 0)
}

// close destroys the window and waits for its thread to finish.
func (w *window) close() {
	procPostMessageW.Call(w.hwnd, wmAppClose, 0, 0)
	<-w.done
}

// geometry is the top centre of the primary display, which Windows always
// places at (0, 0) of the virtual screen.
func (w *window) geometry() (x, y, cx, cy int) {
	s := dpiScale()
	cx, cy = int(520*s), int(36*s)
	return (systemMetric(smCxScreen) - cx) / 2, 0, cx, cy
}

// keepOnTop puts the indicator back above other topmost windows.
func (w *window) keepOnTop() {
	procSetWindowPos.Call(w.hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	v, ok := openWindows.Load(hwnd)
	if !ok {
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	}
	w := v.(*window)
	switch msg {
	case wmPaint:
		w.paint()
		return 0
	case wmEraseBkgnd:
		return 1 // paint fills the whole window
	case wmTimer:
		w.keepOnTop()
		return 0
	case wmMouseActivate:
		return maNoActivate
	case wmAppUpdate:
		procInvalidateRect.Call(hwnd, 0, 1)
		return 0
	case wmAppClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procKillTimer.Call(hwnd, 1)
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func (w *window) paint() {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	var rc rect
	procGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&rc)))
	w.mu.Lock()
	t := w.text
	w.mu.Unlock()
	s := dpiScale()
	procSetBkMode.Call(hdc, transparentBk)
	fill(hdc, rc, rgb(0xb9, 0x1c, 0x1c))
	drawText(hdc, "● "+t, rc, 15*s, fwSemiBold, rgb(0xff, 0xff, 0xff),
		dtCenter|dtVCenter|dtSingleLine|dtEndEllipsis)
}

func fill(hdc uintptr, rc rect, color uintptr) {
	brush, _, _ := procCreateSolidBrush.Call(color)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), brush)
	procDeleteObject.Call(brush)
}

func drawText(hdc uintptr, text string, rc rect, pt float64, weight int, color uintptr, format uintptr) {
	face, _ := windows.UTF16PtrFromString("Segoe UI")
	height := -int32(pt * 96 / 72) // pt is already DPI-scaled
	font, _, _ := procCreateFontW.Call(uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0,
		1 /* DEFAULT_CHARSET */, 0, 0, cleartype, 0, uintptr(unsafe.Pointer(face)))
	old, _, _ := procSelectObject.Call(hdc, font)
	procSetTextColor.Call(hdc, color)
	s, _ := windows.UTF16FromString(text)
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&s[0])), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), format|dtNoPrefix)
	procSelectObject.Call(hdc, old)
	procDeleteObject.Call(font)
}

func rgb(r, g, b byte) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }

func systemMetric(i int) int {
	r, _, _ := procGetSystemMetrics.Call(uintptr(i))
	return int(int32(r))
}

// dpiScale is the system DPI relative to 96 (100 %).
func dpiScale() float64 {
	if procGetDpiForSystem.Find() != nil {
		return 1
	}
	if r, _, _ := procGetDpiForSystem.Call(); r > 0 {
		return float64(r) / 96
	}
	return 1
}
