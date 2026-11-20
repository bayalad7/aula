//go:build linux

package platform

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"strings"
	"testing"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/shape"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

func TestSessionKind(t *testing.T) {
	for _, tc := range []struct{ typ, wl, disp, want string }{
		{"x11", "", ":0", DisplayX11},
		{"X11", "", ":0", DisplayX11},
		{"x11", "", "", ""},
		{"wayland", "wayland-0", ":0", DisplayWayland}, // XWayland's DISPLAY does not make it X11
		{"wayland", "", "", DisplayWayland},
		{"tty", "", "", ""},
		{"", "wayland-0", ":0", DisplayWayland},
		{"", "", ":99", DisplayX11},
		{"", "", "", ""},
	} {
		if got := sessionKind(tc.typ, tc.wl, tc.disp); got != tc.want {
			t.Errorf("sessionKind(%q, %q, %q) = %q, want %q", tc.typ, tc.wl, tc.disp, got, tc.want)
		}
	}
}

// In Wayland nothing that observes the screen is offered: the agent then
// declares neither screen.thumb nor screen.snapshot (rule 97).
func TestWaylandOffersNoScreen(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if _, err := NewScreenCapture(); !errors.Is(err, ErrNotSupported) {
		t.Errorf("NewScreenCapture = %v, want ErrNotSupported", err)
	}
	if _, err := NewScreenIndicator(); !errors.Is(err, ErrNotSupported) {
		t.Errorf("NewScreenIndicator = %v, want ErrNotSupported", err)
	}
	if err := SystemHotkeys().Register(context.Background(), DefaultCaptureHotkey, func() {}); !errors.Is(err, ErrNotSupported) {
		t.Errorf("Register = %v, want ErrNotSupported", err)
	}
	if DisplayServer() != DisplayWayland {
		t.Errorf("DisplayServer = %q", DisplayServer())
	}
}

func TestXftScale(t *testing.T) {
	for res, want := range map[string]float64{
		"Xft.dpi:\t144\nXft.antialias:\t1": 1.5,
		"Xft.antialias: 1\nXft.dpi: 96":    1,
		"Xft.dpi: 72":                      1, // never smaller than 100 %
		"Xft.dpi: 960":                     3,
		"Xft.dpi: x":                       1,
		"":                                 1,
	} {
		if got := xftScale(res); got != want {
			t.Errorf("xftScale(%q) = %v, want %v", res, got, want)
		}
	}
}

func TestFindKeycode(t *testing.T) {
	// Keycodes 8..10 with 2 symbols each: (a, A), (1, !), (c, C).
	syms := []xproto.Keysym{'a', 'A', '1', '!', 'c', 'C'}
	if code, err := findKeycode(syms, 2, 8, 'c', 'C'); err != nil || code != 10 {
		t.Fatalf("c = %d, %v", code, err)
	}
	if code, err := findKeycode(syms, 2, 8, '1', '1'); err != nil || code != 9 {
		t.Fatalf("1 = %d, %v", code, err)
	}
	if _, err := findKeycode(syms, 2, 8, 'z', 'Z'); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("z = %v", err)
	}
}

func TestX11Mods(t *testing.T) {
	got := x11Mods(ModCtrl | ModAlt | ModShift | ModWin)
	want := uint16(xproto.ModMaskControl | xproto.ModMask1 | xproto.ModMaskShift | xproto.ModMask4)
	if got != want {
		t.Fatalf("x11Mods = %#x, want %#x", got, want)
	}
}

func TestBannerImage(t *testing.T) {
	img := bannerImage("Tu pantalla está siendo vista por el profesor", 520, 36, 1)
	if img.Bounds() != image.Rect(0, 0, 520, 36) {
		t.Fatalf("bounds = %v", img.Bounds())
	}
	if c := img.RGBAAt(2, 2); c != bannerBack {
		t.Errorf("corner = %v, want the red background", c)
	}
	white := 0
	for y := 0; y < 36; y++ {
		for x := 0; x < 520; x++ {
			if img.RGBAAt(x, y) == bannerText {
				white++
			}
		}
	}
	if white < 300 {
		t.Errorf("only %d white pixels: the text was not drawn", white)
	}
	// A text far too long is cut, never drawn past the edge.
	long := bannerImage(strings.Repeat("Tu pantalla ", 40), 520, 36, 1)
	for y := 0; y < 36; y++ {
		if c := long.RGBAAt(519, y); c != bannerBack {
			t.Fatalf("text reaches the right edge at y=%d", y)
		}
	}
}

// needDisplay skips without an X server: tests with a desktop run in CI and
// in a container (docs/desarrollo/empaquetar-linux.md) under Xvfb, never on a
// person's desktop.
func needDisplay(t *testing.T) {
	t.Helper()
	if displayServer() != DisplayX11 {
		t.Skip("no X11 session (run under Xvfb)")
	}
}

func TestX11IndicatorAndCapture(t *testing.T) {
	needDisplay(t)
	ind, err := NewScreenIndicator()
	if err != nil {
		t.Fatal(err)
	}
	if err := ind.Show("Tu pantalla está siendo vista por el profesor"); err != nil {
		t.Fatal(err)
	}
	defer ind.Hide()
	if err := ind.Show("Tu pantalla la está viendo la clase"); err != nil { // replaces the text
		t.Fatal(err)
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
		t.Errorf("override-redirect %v, map state %d: the notice must be on screen without the window manager", attrs.OverrideRedirect, attrs.MapState)
	}
	if shape.Init(c) == nil {
		r, err := shape.GetRectangles(c, win, shape.SkInput).Reply()
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Rectangles) != 0 {
			t.Errorf("input shape has %d rectangles: the notice must not take clicks", len(r.Rectangles))
		}
	}

	// The thumbnail shows the notice: the teacher sees what the student knows (rule 6).
	capt, err := NewScreenCapture()
	if err != nil {
		t.Fatal(err)
	}
	f, err := capt.Capture(context.Background(), DetailWidth, 90)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(f.JPEG))
	if err != nil {
		t.Fatal(err)
	}
	// 254 px right of the centre is inside the banner's right margin (half
	// of 520, less its 12 px padding): always red, never text.
	r, g, b, _ := img.At(img.Bounds().Dx()/2+int(254*float64(f.Width)/float64(primaryWidth(t, c))), 4).RGBA()
	if r>>8 < 0x90 || g>>8 > 0x50 || b>>8 > 0x50 {
		t.Errorf("top centre of the capture = (%d,%d,%d), want the red notice", r>>8, g>>8, b>>8)
	}

	if err := ind.Hide(); err != nil {
		t.Fatal(err)
	}
	if hasWindow(c, "Aula") {
		t.Error("the notice is still there after Hide")
	}
}

func primaryWidth(t *testing.T, c *xgb.Conn) int {
	t.Helper()
	return primaryBounds(c, xproto.Setup(c).DefaultScreen(c)).Dx()
}

func findWindow(t *testing.T, c *xgb.Conn, name string) xproto.Window {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w, ok := windowNamed(c, name); ok {
			return w
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no window %q", name)
	return 0
}

func hasWindow(c *xgb.Conn, name string) bool {
	_, ok := windowNamed(c, name)
	return ok
}

func windowNamed(c *xgb.Conn, name string) (xproto.Window, bool) {
	root := xproto.Setup(c).DefaultScreen(c).Root
	tree, err := xproto.QueryTree(c, root).Reply()
	if err != nil {
		return 0, false
	}
	for _, w := range tree.Children {
		p, err := xproto.GetProperty(c, false, w, xproto.AtomWmName, xproto.AtomString, 0, 64).Reply()
		if err == nil && string(p.Value) == name {
			return w, true
		}
	}
	return 0, false
}

func TestX11Hotkey(t *testing.T) {
	needDisplay(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pressed := make(chan struct{}, 1)
	h := Hotkey{Mods: ModCtrl | ModAlt | ModShift, Key: 'C'}
	if err := SystemHotkeys().Register(ctx, h, func() { pressed <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	// A second owner of the same combination is refused.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if err := SystemHotkeys().Register(ctx2, h, func() {}); !errors.Is(err, ErrHotkeyInUse) {
		t.Fatalf("second Register = %v, want ErrHotkeyInUse", err)
	}

	// Synthetic keys go to Xvfb only (needDisplay): never to a real desktop.
	c, err := xgb.NewConn()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := xtest.Init(c); err != nil {
		t.Skipf("no XTEST: %v", err)
	}
	press := func() {
		root := xproto.Setup(c).DefaultScreen(c).Root
		var codes []xproto.Keycode
		for _, k := range []rune{'￣', '￩', '￡'} { // Control_L, Alt_L, Shift_L keysyms
			code, err := keycodeOfSym(c, xproto.Keysym(k))
			if err != nil {
				t.Skipf("keyboard map: %v", err)
			}
			codes = append(codes, code)
		}
		key, err := keycodeOf(c, 'C')
		if err != nil {
			t.Fatal(err)
		}
		codes = append(codes, key)
		for _, k := range codes {
			xtest.FakeInput(c, xproto.KeyPress, byte(k), 0, root, 0, 0, 0)
		}
		for i := len(codes) - 1; i >= 0; i-- {
			xtest.FakeInput(c, xproto.KeyRelease, byte(codes[i]), 0, root, 0, 0, 0)
		}
		_, _ = xproto.GetInputFocus(c).Reply()
	}
	press()
	select {
	case <-pressed:
	case <-time.After(3 * time.Second):
		t.Fatal("the hotkey did not fire")
	}

	// After cancel the combination is free again.
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for {
		ctx3, cancel3 := context.WithCancel(context.Background())
		err := SystemHotkeys().Register(ctx3, h, func() {})
		cancel3()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Register after cancel = %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func keycodeOfSym(c *xgb.Conn, sym xproto.Keysym) (xproto.Keycode, error) {
	setup := xproto.Setup(c)
	first, last := setup.MinKeycode, setup.MaxKeycode
	m, err := xproto.GetKeyboardMapping(c, first, byte(last-first+1)).Reply()
	if err != nil {
		return 0, err
	}
	return findKeycode(m.Keysyms, int(m.KeysymsPerKeycode), first, sym, sym)
}

// The monitors of the X11 session, grabbed at their size: what the
// broadcast and the presentation encode (rule 106).
func TestX11DisplaysAndGrab(t *testing.T) {
	needDisplay(t)
	dc, err := NewDisplayCapture()
	if err != nil {
		t.Fatal(err)
	}
	ds, err := dc.Displays()
	if err != nil || len(ds) == 0 || !ds[0].Primary {
		t.Fatalf("Displays = %+v, %v", ds, err)
	}
	img, err := dc.Grab(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != ds[0].Bounds.Dx() || img.Bounds().Dy() != ds[0].Bounds.Dy() {
		t.Fatalf("grab %v, monitor %v", img.Bounds(), ds[0].Bounds)
	}
	if _, err := dc.Grab(context.Background(), 99); err == nil {
		t.Error("a monitor that does not exist was grabbed")
	}
	if CanExcludeFromCapture() {
		t.Error("X11 cannot exclude a window: the console must say it is seen")
	}
	if err := ExcludeFromCapture(true); err != nil {
		t.Errorf("ExcludeFromCapture must not fail on Linux (presenting would refuse): %v", err)
	}
}
