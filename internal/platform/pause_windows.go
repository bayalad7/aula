//go:build windows

package platform

import (
	"fmt"
	"image/color"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// The pause window (docs/protocol.md §5.9; D-013): one borderless window per
// monitor, topmost, that takes the clicks and is put back on top every half
// second. Every window lives on one locked OS thread, driven through a
// message-only host window; nothing here reads the keyboard.

var (
	procSetForegroundWindowPause = user32.NewProc("SetForegroundWindow")
)

const (
	pauseClass = "AulaPauseWindow"

	hwndMessage = ^uintptr(2) // HWND_MESSAGE (-3)

	wmClose         = 0x0010
	wmSettingChange = 0x001A
	wmDisplayChange = 0x007E
	wmDPIChanged    = 0x02E0

	wmPauseUpdate  = 0x8011 // WM_APP+0x11: the text changed
	wmPauseRestack = 0x8012 // the yield changed
	wmPauseClose   = 0x8013 // close everything
	wmPauseRebuild = 0x8014 // the monitors changed

	dtWordBreak = 0x0010
	dtCalcRect  = 0x0400
	fwNormal    = 400

	// pauseOnTop is how often the windows go back on top (D-013).
	pauseOnTop = 500 * time.Millisecond
)

var (
	pauseClassOnce sync.Once
	pauseClassErr  error
	pauseWindows   sync.Map // hwnd → *winPause (the host and every cover)
)

func registerPauseClass() error {
	pauseClassOnce.Do(func() {
		if err := registerClass(); err != nil { // sets instance
			pauseClassErr = err
			return
		}
		cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
		name, _ := windows.UTF16PtrFromString(pauseClass)
		wc := wndClassEx{
			style:     csHRedraw | csVRedraw,
			wndProc:   syscall.NewCallback(pauseProc),
			instance:  instance,
			cursor:    cursor,
			className: name,
		}
		wc.size = uint32(unsafe.Sizeof(wc))
		if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 && err != errClassAlreadyExists {
			pauseClassErr = fmt.Errorf("RegisterClassEx: %w", err)
		}
	})
	return pauseClassErr
}

func newPauseOverlay(look PauseLook) (PauseOverlay, error) {
	if err := registerPauseClass(); err != nil {
		return nil, fmt.Errorf("pause overlay: %w", err)
	}
	return &winPause{look: look}, nil
}

type winPause struct {
	look PauseLook

	op    sync.Mutex // one Show or Hide at a time
	mu    sync.Mutex
	text  PauseText
	yield bool
	host  uintptr       // message-only window; 0 while hidden
	done  chan struct{} // closed when the thread ends

	// Only on the window thread.
	covers []uintptr
	minute int
}

func (p *winPause) Show(text PauseText) error {
	p.op.Lock()
	defer p.op.Unlock()
	p.mu.Lock()
	p.text = text
	if p.host != 0 {
		host := p.host
		p.mu.Unlock()
		procPostMessageW.Call(host, wmPauseUpdate, 0, 0) //nolint:errcheck // best effort
		return nil
	}
	p.done = make(chan struct{})
	p.mu.Unlock()
	errc := make(chan error, 1)
	go p.run(errc)
	if err := <-errc; err != nil {
		return fmt.Errorf("pause overlay: %w", err)
	}
	return nil
}

func (p *winPause) Yield(on bool) error {
	p.mu.Lock()
	p.yield = on
	host := p.host
	p.mu.Unlock()
	if host != 0 {
		procPostMessageW.Call(host, wmPauseRestack, 0, 0) //nolint:errcheck // best effort
	}
	return nil
}

func (p *winPause) Hide() error {
	p.op.Lock()
	defer p.op.Unlock()
	p.mu.Lock()
	host, done := p.host, p.done
	p.host = 0
	p.mu.Unlock()
	if host == 0 {
		return nil
	}
	procPostMessageW.Call(host, wmPauseClose, 0, 0) //nolint:errcheck // best effort
	<-done
	return nil
}

func (p *winPause) run(errc chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p.mu.Lock()
	done := p.done
	p.mu.Unlock()
	defer close(done)

	class, _ := windows.UTF16PtrFromString(pauseClass)
	host, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, instance, 0)
	if host == 0 {
		errc <- fmt.Errorf("CreateWindowEx (host): %w", err)
		return
	}
	pauseWindows.Store(host, p)
	defer pauseWindows.Delete(host)
	if err := p.build(); err != nil {
		procDestroyWindow.Call(host) //nolint:errcheck // nothing else to do
		errc <- err
		return
	}
	if len(p.covers) > 0 {
		procSetForegroundWindowPause.Call(p.covers[0]) //nolint:errcheck // best effort: Windows may refuse
	}
	p.restack()
	procSetTimer.Call(host, 1, uintptr(pauseOnTop/time.Millisecond), 0) //nolint:errcheck // a failed timer only skips the re-stacking
	p.mu.Lock()
	p.host = host
	p.mu.Unlock()
	errc <- nil

	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m))) //nolint:errcheck // returns whether it translated
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m))) //nolint:errcheck // returns the procedure's result
	}
}

// build opens one cover per active monitor.
func (p *winPause) build() error {
	ds, err := winDisplays{}.Displays()
	if err != nil {
		return err
	}
	class, _ := windows.UTF16PtrFromString(pauseClass)
	title, _ := windows.UTF16PtrFromString("Aula — pantalla en pausa")
	// Topmost and without a taskbar button; it takes clicks and focus.
	ex := uintptr(wsExTopmost | wsExToolWindow)
	for _, d := range ds {
		b := d.Bounds
		hwnd, _, err := procCreateWindowExW.Call(ex, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
			wsPopup, uintptr(b.Min.X), uintptr(b.Min.Y), uintptr(b.Dx()), uintptr(b.Dy()), 0, 0, instance, 0)
		if hwnd == 0 {
			p.destroyCovers()
			return fmt.Errorf("CreateWindowEx (cover): %w", err)
		}
		pauseWindows.Store(hwnd, p)
		p.covers = append(p.covers, hwnd)
		procShowWindow.Call(hwnd, swShowNoActivate) //nolint:errcheck // returns the previous visibility
	}
	p.minute = time.Now().Minute()
	return nil
}

func (p *winPause) destroyCovers() {
	for _, h := range p.covers {
		procDestroyWindow.Call(h) //nolint:errcheck // nothing else to do
		pauseWindows.Delete(h)
	}
	p.covers = nil
}

// restack puts the covers back on top, or right below the main window while
// they yield.
func (p *winPause) restack() {
	p.mu.Lock()
	yield := p.yield
	p.mu.Unlock()
	after := hwndTopmost
	if yield {
		if main, err := mainWindow(); err == nil {
			after = main
		}
	}
	for _, h := range p.covers {
		procSetWindowPos.Call(h, after, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow) //nolint:errcheck // best effort
	}
}

func (p *winPause) invalidate() {
	for _, h := range p.covers {
		procInvalidateRect.Call(h, 0, 1) //nolint:errcheck // best effort
	}
}

func pauseProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	v, ok := pauseWindows.Load(hwnd)
	if !ok {
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	}
	p := v.(*winPause)
	switch msg {
	// Host messages.
	case wmTimer:
		p.restack()
		if m := time.Now().Minute(); m != p.minute { // the clock at the foot
			p.minute = m
			p.invalidate()
		}
		return 0
	case wmPauseUpdate:
		p.invalidate()
		return 0
	case wmPauseRestack:
		p.restack()
		return 0
	case wmPauseRebuild:
		p.destroyCovers()
		if err := p.build(); err == nil {
			p.restack()
		}
		return 0
	case wmPauseClose:
		p.destroyCovers()
		procKillTimer.Call(hwnd, 1)  //nolint:errcheck // the window goes away
		procDestroyWindow.Call(hwnd) //nolint:errcheck // posts WM_DESTROY
		return 0
	case wmDestroy:
		if p.isHost(hwnd) {
			procPostQuitMessage.Call(0) //nolint:errcheck // no result
		}
		return 0

	// Cover messages.
	case wmPaint:
		p.paint(hwnd)
		return 0
	case wmEraseBkgnd:
		return 1 // paint fills the whole window
	case wmClose:
		return 0 // Alt+F4 does not close it; unfreeze or the end of the connection does
	case wmDisplayChange:
		p.postHost(wmPauseRebuild)
		return 0
	case wmDPIChanged, wmSettingChange: // a new scale or the light/dark theme
		procInvalidateRect.Call(hwnd, 0, 1) //nolint:errcheck // best effort
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func (p *winPause) isHost(hwnd uintptr) bool {
	for _, h := range p.covers {
		if h == hwnd {
			return false
		}
	}
	return true
}

func (p *winPause) postHost(msg uintptr) {
	p.mu.Lock()
	host := p.host
	p.mu.Unlock()
	if host != 0 {
		procPostMessageW.Call(host, msg, 0, 0) //nolint:errcheck // best effort
	}
}

// paint lays the text out centred on its monitor: the pause mark, the title,
// the teacher's message in a frame, who paused it, the note and the clock.
func (p *winPause) paint(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps))) //nolint:errcheck // always succeeds
	var rc rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc))) //nolint:errcheck // a zero rect paints nothing
	p.mu.Lock()
	t := p.text
	p.mu.Unlock()
	colors := p.look.Light
	if systemDark() {
		colors = p.look.Dark
	}
	bg, fg := colorRef(colors.Background), colorRef(colors.Text)
	s := windowScale(hwnd)
	px := func(v float64) int32 { return int32(v * s) }

	procSetBkMode.Call(hdc, transparentBk) //nolint:errcheck // returns the previous mode
	fill(hdc, rc, bg)

	width := min(rc.right-rc.left-2*px(48), px(720))
	type block struct {
		text   string
		pt     float64
		weight int
		gap    int32 // space above
		framed bool
	}
	blocks := []block{{t.Title, 28, fwSemiBold, px(24), false}}
	if t.Message != "" {
		blocks = append(blocks, block{"«" + t.Message + "»", 18, fwNormal, px(20), true})
	}
	blocks = append(blocks,
		block{t.Detail, 16, fwNormal, px(20), false},
		block{t.Note, 14, fwNormal, px(12), false},
		block{time.Now().Format("15:04"), 14, fwSemiBold, px(24), false})

	frame := px(14)
	heights := make([]int32, len(blocks))
	mark := px(56)
	total := mark
	for i, b := range blocks {
		w := width
		if b.framed {
			w -= 2 * frame
		}
		heights[i] = measureText(hdc, b.text, b.pt*s, b.weight, w)
		if b.framed {
			heights[i] += 2 * frame
		}
		total += b.gap + heights[i]
	}
	y := rc.top + (rc.bottom-rc.top-total)/2
	cx := (rc.left + rc.right) / 2

	// The pause mark: two bars.
	bar, gap := px(16), px(14)
	fill(hdc, rect{cx - gap/2 - bar, y, cx - gap/2, y + mark}, fg)
	fill(hdc, rect{cx + gap/2, y, cx + gap/2 + bar, y + mark}, fg)
	y += mark

	left := cx - width/2
	for i, b := range blocks {
		y += b.gap
		box := rect{left, y, left + width, y + heights[i]}
		if b.framed {
			line := max(px(2), 1)
			fill(hdc, rect{box.left, box.top, box.right, box.top + line}, fg)
			fill(hdc, rect{box.left, box.bottom - line, box.right, box.bottom}, fg)
			fill(hdc, rect{box.left, box.top, box.left + line, box.bottom}, fg)
			fill(hdc, rect{box.right - line, box.top, box.right, box.bottom}, fg)
			box = rect{box.left + frame, box.top + frame, box.right - frame, box.bottom - frame}
		}
		drawWrapped(hdc, b.text, box, b.pt*s, b.weight, fg)
		y += heights[i]
	}
}

func newFont(pt float64, weight int) uintptr {
	face, _ := windows.UTF16PtrFromString("Segoe UI")
	height := -int32(pt * 96 / 72) // pt is already DPI-scaled
	font, _, _ := procCreateFontW.Call(uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0,
		1 /* DEFAULT_CHARSET */, 0, 0, cleartype, 0, uintptr(unsafe.Pointer(face)))
	return font
}

// measureText is the height of text wrapped to width.
func measureText(hdc uintptr, text string, pt float64, weight int, width int32) int32 {
	if text == "" {
		return 0
	}
	font := newFont(pt, weight)
	old, _, _ := procSelectObject.Call(hdc, font)
	rc := rect{0, 0, width, 0}
	u, _ := windows.UTF16FromString(text)
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), //nolint:errcheck // returns the height, read from rc
		dtCenter|dtWordBreak|dtNoPrefix|dtCalcRect)
	procSelectObject.Call(hdc, old) //nolint:errcheck // returns the font put back
	procDeleteObject.Call(font)     //nolint:errcheck // best effort
	return rc.bottom - rc.top
}

func drawWrapped(hdc uintptr, text string, rc rect, pt float64, weight int, color uintptr) {
	if text == "" {
		return
	}
	font := newFont(pt, weight)
	old, _, _ := procSelectObject.Call(hdc, font)
	procSetTextColor.Call(hdc, color) //nolint:errcheck // returns the previous color
	u, _ := windows.UTF16FromString(text)
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), //nolint:errcheck // returns the height
		dtCenter|dtWordBreak|dtNoPrefix)
	procSelectObject.Call(hdc, old) //nolint:errcheck // returns the font put back
	procDeleteObject.Call(font)     //nolint:errcheck // best effort
}

func colorRef(c color.RGBA) uintptr { return rgb(c.R, c.G, c.B) }

// windowScale is the DPI of the window's monitor relative to 96 (100 %).
func windowScale(hwnd uintptr) float64 {
	if procGetDpiForWindow.Find() == nil {
		if r, _, _ := procGetDpiForWindow.Call(hwnd); r > 0 {
			return float64(r) / 96
		}
	}
	return dpiScale()
}

// systemDark reports whether the user chose the dark theme for apps.
func systemDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}
