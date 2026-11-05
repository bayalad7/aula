//go:build windows

package platform

import (
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SendInput constants (winuser.h).
const (
	inputMouse    = 0
	inputKeyboard = 1

	mouseMove       = 0x0001
	mouseLeftDown   = 0x0002
	mouseLeftUp     = 0x0004
	mouseRightDown  = 0x0008
	mouseRightUp    = 0x0010
	mouseMiddleDown = 0x0020
	mouseMiddleUp   = 0x0040
	mouseWheel      = 0x0800
	mouseHWheel     = 0x1000
	mouseAbsolute   = 0x8000

	keyExtended = 0x0001
	keyUp       = 0x0002
	keyScancode = 0x0008

	smCXScreen = 0
	smCYScreen = 1

	// aulaInputTag marks every event Aula injects (dwExtraInfo), so it can be
	// told apart from the student's own.
	aulaInputTag = 0x41554C41 // "AULA"

	// localSlack is how long after an injection the OS may still report it as
	// the last input: input seen later is the student's.
	localSlack = 150 * time.Millisecond
	// cursorSlack is how far (pixels) the pointer may be from where Aula left
	// it before that counts as the student moving it.
	cursorSlack = 3
)

var (
	procSendInput        = user32.NewProc("SendInput")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
)

type mouseInput struct {
	dx, dy    int32
	mouseData uint32
	flags     uint32
	time      uint32
	extra     uintptr
}

type keybdInput struct {
	vk, scan uint16
	flags    uint32
	time     uint32
	extra    uintptr
}

// rawInput is the INPUT struct. Its union is as large as MOUSEINPUT, the
// biggest member; keyboard events are written over the same bytes.
type rawInput struct {
	typ uint32
	mi  mouseInput
}

type lastInputInfo struct {
	size uint32
	time uint32
}

func newInputInjector() (InputInjector, error) {
	return &winInput{created: time.Now()}, nil
}

type winInput struct {
	mu         sync.Mutex
	created    time.Time
	lastInject time.Time
	cursor     point // where the last injected pointer event left it
	haveCursor bool
	local      time.Time
}

func (w *winInput) Inject(events []InputEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.checkCursor() // the student may have moved the pointer since the last batch
	in := make([]rawInput, 0, len(events))
	for _, e := range events {
		r, err := w.toRaw(e)
		if err != nil {
			return err
		}
		in = append(in, r)
	}
	if len(in) == 0 {
		return nil
	}
	n, _, err := procSendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
	w.lastInject = time.Now()
	if int(n) != len(in) {
		// Blocked: the secure desktop (lock screen, UAC) or, without telling
		// it apart, an elevated window (UIPI).
		return fmt.Errorf("send input: %d of %d events: %w", n, len(in), err)
	}
	return nil
}

func (w *winInput) toRaw(e InputEvent) (rawInput, error) {
	switch e.Kind {
	case InputMove, InputButton:
		var flags uint32 = mouseMove | mouseAbsolute
		if e.Kind == InputButton {
			down, up := uint32(mouseLeftDown), uint32(mouseLeftUp)
			switch e.Button {
			case ButtonRight:
				down, up = mouseRightDown, mouseRightUp
			case ButtonMiddle:
				down, up = mouseMiddleDown, mouseMiddleUp
			}
			if e.Down {
				flags |= down
			} else {
				flags |= up
			}
		}
		w.expectCursor(e.X, e.Y)
		// Absolute coordinates without VIRTUALDESK map 0-65535 to the primary
		// display, the one the console sees.
		return rawInput{typ: inputMouse, mi: mouseInput{
			dx: int32(math.Round(e.X * 65535)), dy: int32(math.Round(e.Y * 65535)),
			flags: flags, extra: aulaInputTag}}, nil
	case InputWheel:
		if e.DX != 0 {
			return rawInput{typ: inputMouse, mi: mouseInput{mouseData: uint32(int32(e.DX)), flags: mouseHWheel, extra: aulaInputTag}}, nil
		}
		// The OS takes positive as away from the user; the browser's deltaY
		// is positive towards the user: the console already flips it.
		return rawInput{typ: inputMouse, mi: mouseInput{mouseData: uint32(int32(e.DY)), flags: mouseWheel, extra: aulaInputTag}}, nil
	case InputKey:
		k, ok := keyCodes[e.Code]
		if !ok {
			return rawInput{}, fmt.Errorf("send input: unknown key %q", e.Code)
		}
		kb := keybdInput{extra: aulaInputTag}
		if k.vk != 0 {
			kb.vk = k.vk
		} else {
			kb.scan = k.scan
			kb.flags = keyScancode
			if k.ext {
				kb.flags |= keyExtended
			}
		}
		if !e.Down {
			kb.flags |= keyUp
		}
		r := rawInput{typ: inputKeyboard}
		*(*keybdInput)(unsafe.Pointer(&r.mi)) = kb
		return r, nil
	}
	return rawInput{}, fmt.Errorf("send input: unknown event kind %d", e.Kind)
}

// expectCursor remembers where an absolute pointer event leaves the pointer,
// in the same pixels GetCursorPos reports.
func (w *winInput) expectCursor(x, y float64) {
	cx, cy := systemMetric(smCXScreen), systemMetric(smCYScreen)
	w.cursor = point{x: int32(math.Round(x * float64(cx-1))), y: int32(math.Round(y * float64(cy-1)))}
	w.haveCursor = true
}

// checkCursor records local input if the pointer is not where Aula left it.
// Called with w.mu held.
func (w *winInput) checkCursor() {
	if !w.haveCursor {
		return
	}
	var p point
	if r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p))); r == 0 {
		return
	}
	if abs32(p.x-w.cursor.x) > cursorSlack || abs32(p.y-w.cursor.y) > cursorSlack {
		w.local = time.Now()
		w.cursor = p
	}
}

func (w *winInput) LocalInput() (time.Time, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.checkCursor()
	info := lastInputInfo{size: uint32(unsafe.Sizeof(lastInputInfo{}))}
	if r, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info))); r == 0 {
		return w.local, fmt.Errorf("last input: %w", err)
	}
	// The 32-bit tick count wraps every 49.7 days; the unsigned difference
	// is still the elapsed time.
	now := uint32(windows.DurationSinceBoot() / time.Millisecond)
	at := time.Now().Add(-time.Duration(now-info.time) * time.Millisecond)
	if at.After(w.created) && at.After(w.lastInject.Add(localSlack)) && at.After(w.local) {
		w.local = at
	}
	return w.local, nil
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
