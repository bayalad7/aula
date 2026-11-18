//go:build linux

package platform

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

func TestX11Keycodes(t *testing.T) {
	for code, want := range map[string]byte{
		"Escape": 9, "KeyA": 38, "KeyQ": 24, "Space": 65, "F12": 96,
		"ArrowUp": 111, "Delete": 119, "ControlRight": 105, "NumpadEnter": 104, "NumLock": 77,
		"Nope": 0,
	} {
		if got := x11Keycode(code); got != want {
			t.Errorf("x11Keycode(%q) = %d, want %d", code, got, want)
		}
	}
	// Every key Aula forwards has a code here (none silently lost).
	for code := range keyCodes {
		if x11Keycode(code) == 0 {
			t.Errorf("%s has no X keycode", code)
		}
	}
}

func TestWheelClicks(t *testing.T) {
	for _, tc := range []struct {
		dx, dy int
		want   string
	}{
		{0, 120, "\x04"}, {0, -240, "\x05\x05"}, {120, 0, "\x07"}, {-120, 0, "\x06"},
		{0, 30, "\x04"}, // a touchpad's small step still scrolls
		{0, 0, ""},
	} {
		if got := string(wheelClicks(tc.dx, tc.dy)); got != tc.want {
			t.Errorf("wheelClicks(%d, %d) = %q, want %q", tc.dx, tc.dy, got, tc.want)
		}
	}
}

// Injected input moves the pointer and is never taken for the student's;
// input from another client (the student, here a second XTEST client on
// Xvfb) is.
func TestX11InputInjectAndLocal(t *testing.T) {
	needDisplay(t)
	inj, err := NewInputInjector()
	if err != nil {
		t.Fatal(err)
	}
	in := inj.(*x11Input)
	if err := inj.Inject([]InputEvent{{Kind: InputMove, X: 0.5, Y: 0.25}}); err != nil {
		t.Fatal(err)
	}
	c, err := xgb.NewConn()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	root := xproto.Setup(c).DefaultScreen(c).Root
	p, err := xproto.QueryPointer(c, root).Reply()
	if err != nil {
		t.Fatal(err)
	}
	wantX := in.area.Min.X + (in.area.Dx()-1)/2
	if d := int(p.RootX) - wantX; d < -1 || d > 1 {
		t.Errorf("pointer x = %d, want about %d", p.RootX, wantX)
	}
	if err := inj.Inject([]InputEvent{{Kind: InputKey, Code: "ShiftLeft", Down: true}, {Kind: InputKey, Code: "ShiftLeft"}}); err != nil {
		t.Fatal(err)
	}
	if at, err := inj.LocalInput(); err != nil || !at.IsZero() {
		t.Fatalf("LocalInput after injecting = %v, %v; want none", at, err)
	}
	time.Sleep(400 * time.Millisecond) // past the slack
	if err := xtest.Init(c); err != nil {
		t.Skip("no XTEST")
	}
	xtest.FakeInput(c, xproto.MotionNotify, 0, 0, root, 10, 10, 0)
	_, _ = xproto.GetInputFocus(c).Reply()
	if at, err := inj.LocalInput(); err != nil || at.IsZero() {
		t.Fatalf("LocalInput after the student's move = %v, %v; want a time", at, err)
	}
}

var testLook = PauseLook{
	Light: PauseColors{Background: color.RGBA{0xf4, 0xf6, 0xfa, 0xff}, Text: color.RGBA{0x11, 0x18, 0x27, 0xff}},
	Dark:  PauseColors{Background: color.RGBA{0x0b, 0x0f, 0x19, 0xff}, Text: color.RGBA{0xf9, 0xfa, 0xfb, 0xff}},
}

func TestPauseImage(t *testing.T) {
	txt := PauseText{Title: "Tu profesor pausó esta computadora", Message: "Miren al frente",
		Detail: "Profe Ana · canal 1. Tu trabajo sigue abierto y nada se perdió.",
		Note:   "Si se pierde la conexión con la clase, la pausa se quita sola."}
	img := pauseImage(txt, testLook.Light, 1280, 720, 1, time.Date(2026, 11, 18, 9, 5, 0, 0, time.UTC))
	if img.RGBAAt(5, 5) != testLook.Light.Background {
		t.Errorf("corner = %v", img.RGBAAt(5, 5))
	}
	ink := 0
	for y := 0; y < 720; y++ {
		for x := 0; x < 1280; x++ {
			if img.RGBAAt(x, y) == testLook.Light.Text {
				ink++
			}
		}
	}
	if ink < 3000 {
		t.Errorf("only %d text pixels: the text was not drawn", ink)
	}
	// Long words never leave the line.
	long := pauseImage(PauseText{Title: strings.Repeat("Pausa", 80)}, testLook.Dark, 900, 600, 1, time.Now())
	for y := 0; y < 600; y++ {
		if long.RGBAAt(899, y) != testLook.Dark.Background || long.RGBAAt(0, y) != testLook.Dark.Background {
			t.Fatalf("text reaches the edge at y=%d", y)
		}
	}
}

func TestX11Pause(t *testing.T) {
	needDisplay(t)
	o, err := NewPauseOverlay(testLook)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Hide()
	if err := o.Show(PauseText{Title: "Tu profesor pausó esta computadora", Detail: "x", Note: "y"}); err != nil {
		t.Fatal(err)
	}
	if err := o.Show(PauseText{Title: "Tu profesor pausó esta computadora", Message: "Otro mensaje"}); err != nil {
		t.Fatal(err) // replaces the text, same windows
	}
	c, err := xgb.NewConn()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	win := findWindow(t, c, "Aula")
	attrs, err := xproto.GetWindowAttributes(c, win).Reply()
	if err != nil {
		t.Fatal(err)
	}
	if !attrs.OverrideRedirect || attrs.MapState != xproto.MapStateViewable {
		t.Errorf("pause window: override-redirect %v, map state %d", attrs.OverrideRedirect, attrs.MapState)
	}
	geo, err := xproto.GetGeometry(c, xproto.Drawable(win)).Reply()
	if err != nil {
		t.Fatal(err)
	}
	sc := xproto.Setup(c).DefaultScreen(c)
	if geo.Width != sc.WidthInPixels || geo.Height != sc.HeightInPixels {
		t.Errorf("pause covers %dx%d, the screen is %dx%d", geo.Width, geo.Height, sc.WidthInPixels, sc.HeightInPixels)
	}
	if f, err := xproto.GetInputFocus(c).Reply(); err != nil || f.Focus != win {
		t.Errorf("focus = %v, %v; want the pause window once", f, err)
	}
	if err := o.Yield(true); err != nil {
		t.Fatal(err)
	}
	if err := o.Yield(false); err != nil {
		t.Fatal(err)
	}
	if err := o.Hide(); err != nil {
		t.Fatal(err)
	}
	if hasWindow(c, "Aula") {
		t.Error("the pause is still there after Hide")
	}
	if err := o.Hide(); err != nil {
		t.Errorf("second Hide = %v", err)
	}
}
