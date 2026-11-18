//go:build linux

package platform

import (
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/screensaver"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

// Remote control on Linux: XTEST in an X11 session (docs/privacidad.md rule
// 101). In Wayland nothing can inject input without a portal: ErrNotSupported,
// and the agent does not declare remote.
//
// Keys go by physical key: an X keycode is the Linux evdev code plus 8, and
// for the keys without the E0 prefix evdev numbers them like the PC set 1
// scancodes of keyCodes. The student's layout decides the character.

// localSlack is how close to an injection a "last input" may be and still be
// Aula's own (the X server stamps it a little later).
const x11LocalSlack = 150 * time.Millisecond

func newInputInjector() (InputInjector, error) {
	c, screen, err := x11Connect()
	if err != nil {
		return nil, err
	}
	if err := xtest.Init(c); err != nil {
		c.Close()
		return nil, fmt.Errorf("X11: no XTEST: %w", ErrNotSupported)
	}
	in := &x11Input{c: c, root: screen.Root, area: primaryBounds(c, screen), created: time.Now()}
	in.idle = screensaver.Init(c) == nil
	return in, nil
}

type x11Input struct {
	mu         sync.Mutex
	c          *xgb.Conn
	root       xproto.Window
	area       image.Rectangle
	idle       bool // the ScreenSaver extension tells the last input
	created    time.Time
	lastInject time.Time
	local      time.Time
}

// extendedEvdev are the evdev codes of the keys whose scancode has the E0
// prefix, and of those Windows injects by virtual key.
var extendedEvdev = map[string]byte{
	"NumpadEnter": 96, "ControlRight": 97, "NumpadDivide": 98, "AltRight": 100,
	"Home": 102, "ArrowUp": 103, "PageUp": 104, "ArrowLeft": 105, "ArrowRight": 106,
	"End": 107, "ArrowDown": 108, "PageDown": 109, "Insert": 110, "Delete": 111,
	"ContextMenu": 127, "PrintScreen": 99, "Pause": 119, "NumLock": 69,
}

// x11Keycode is the X keycode of a KeyboardEvent.code, or 0.
func x11Keycode(code string) byte {
	if e, ok := extendedEvdev[code]; ok {
		return e + 8
	}
	k, ok := keyCodes[code]
	if !ok || k.ext || k.vk != 0 || k.scan == 0 || k.scan > 0x58 {
		return 0
	}
	return byte(k.scan) + 8
}

// x11Button maps a mouse button to X's: 1 left, 2 middle, 3 right.
func x11Button(b MouseButton) byte {
	switch b {
	case ButtonLeft:
		return 1
	case ButtonMiddle:
		return 2
	case ButtonRight:
		return 3
	}
	return 0
}

// wheelClicks turns wheel units (120 a notch, positive up or right) into X
// button clicks: 4 up, 5 down, 6 left, 7 right.
func wheelClicks(dx, dy int) []byte {
	var out []byte
	add := func(units int, pos, neg byte) {
		n := (abs(units) + 60) / 120 // round to whole notches
		if units != 0 && n == 0 {
			n = 1
		}
		b := pos
		if units < 0 {
			b = neg
		}
		for range n {
			out = append(out, b)
		}
	}
	add(dy, 4, 5)
	add(dx, 7, 6)
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (in *x11Input) point(e InputEvent) (int16, int16) {
	x := in.area.Min.X + int(min(max(e.X, 0), 1)*float64(in.area.Dx()-1)+0.5)
	y := in.area.Min.Y + int(min(max(e.Y, 0), 1)*float64(in.area.Dy()-1)+0.5)
	return int16(x), int16(y)
}

func (in *x11Input) Inject(events []InputEvent) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.checkLocal() // the student may have used the computer since the last batch
	fake := func(typ, detail byte, x, y int16) {
		xtest.FakeInput(in.c, typ, detail, 0, in.root, x, y, 0)
	}
	for _, e := range events {
		switch e.Kind {
		case InputMove:
			x, y := in.point(e)
			fake(xproto.MotionNotify, 0, x, y)
		case InputButton:
			b := x11Button(e.Button)
			if b == 0 {
				return fmt.Errorf("remote input: button %d", e.Button)
			}
			x, y := in.point(e)
			fake(xproto.MotionNotify, 0, x, y)
			if e.Down {
				fake(xproto.ButtonPress, b, 0, 0)
			} else {
				fake(xproto.ButtonRelease, b, 0, 0)
			}
		case InputWheel:
			for _, b := range wheelClicks(e.DX, e.DY) {
				fake(xproto.ButtonPress, b, 0, 0)
				fake(xproto.ButtonRelease, b, 0, 0)
			}
		case InputKey:
			k := x11Keycode(e.Code)
			if k == 0 {
				continue // a key this keyboard has no code for: skipped, as on Windows
			}
			if e.Down {
				fake(xproto.KeyPress, k, 0, 0)
			} else {
				fake(xproto.KeyRelease, k, 0, 0)
			}
		default:
			return fmt.Errorf("remote input: kind %d", e.Kind)
		}
	}
	// A round trip: the events reached the server before Inject returns,
	// and a dead connection shows as an error.
	if _, err := xproto.GetInputFocus(in.c).Reply(); err != nil {
		return fmt.Errorf("remote input: %w", err)
	}
	in.lastInject = time.Now()
	return nil
}

func (in *x11Input) LocalInput() (time.Time, error) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if err := in.checkLocal(); err != nil {
		return in.local, err
	}
	return in.local, nil
}

// checkLocal records input Aula did not inject: the ScreenSaver extension
// says how long ago the last keyboard or mouse input was (any device); if
// that was neither before the injector existed nor right after an
// injection, it was the student's. It never learns which key.
func (in *x11Input) checkLocal() error {
	if !in.idle {
		return fmt.Errorf("last input: no ScreenSaver extension: %w", ErrNotSupported)
	}
	r, err := screensaver.QueryInfo(in.c, xproto.Drawable(in.root)).Reply()
	if err != nil {
		return fmt.Errorf("last input: %w", err)
	}
	at := time.Now().Add(-time.Duration(r.MsSinceUserInput) * time.Millisecond)
	if at.After(in.created) && at.After(in.lastInject.Add(x11LocalSlack)) && at.After(in.local) {
		in.local = at
	}
	return nil
}
