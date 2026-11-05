package platform

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// needTaskbar skips when there is no notification area to put an icon in:
// CI runners and services have no Explorer.
func needTaskbar(t *testing.T) {
	t.Helper()
	name, _ := windows.UTF16PtrFromString("Shell_TrayWnd")
	if h, _, _ := user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(name)), 0); h == 0 {
		t.Skip("no taskbar in this session")
	}
}

func testPNG(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// The icon appears in the notification area for a moment, changes and goes
// away; a second one in the same process is refused.
func TestTrayLifecycle(t *testing.T) {
	needTaskbar(t)
	tr, err := NewTray(TrayOptions{Icon: testPNG(t, color.RGBA{0x1d, 0x4e, 0xd8, 0xff}),
		MarkedIcon: testPNG(t, color.RGBA{0xc8, 0x32, 0x2a, 0xff}), Tooltip: "Aula — prueba", OpenLabel: "Abrir Aula"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTray(TrayOptions{Icon: []byte{1}, MarkedIcon: []byte{1}}); err == nil {
		t.Fatal("a second icon was accepted")
	}
	if err := tr.SetTooltip("Aula — conectado (canal 1 - Prueba)"); err != nil {
		t.Fatal(err)
	}
	if err := tr.SetMarked(true); err != nil {
		t.Fatal(err)
	}
	if err := tr.SetMarked(false); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if err := tr.SetMarked(true); err != nil {
		t.Fatalf("SetMarked after Close: %v", err)
	}
	// After Close, a new icon is allowed again.
	tr2, err := NewTray(TrayOptions{Icon: testPNG(t, color.White), MarkedIcon: testPNG(t, color.Black), OpenLabel: "Abrir"})
	if err != nil {
		t.Fatal(err)
	}
	_ = tr2.Close()
}

func TestTrayRejectsBadIcons(t *testing.T) {
	needTaskbar(t)
	if _, err := NewTray(TrayOptions{}); err == nil {
		t.Fatal("no icons accepted")
	}
	if _, err := NewTray(TrayOptions{Icon: []byte("no es png"), MarkedIcon: []byte("tampoco")}); err == nil {
		t.Fatal("bad icon data accepted")
	}
	// The failed attempt must not keep the slot.
	tr, err := NewTray(TrayOptions{Icon: testPNG(t, color.White), MarkedIcon: testPNG(t, color.Black), OpenLabel: "Abrir"})
	if err != nil {
		t.Fatal(err)
	}
	_ = tr.Close()
}

// Without a window of its own, a process has no main window to flash.
func TestAppWindowNeedsAWindow(t *testing.T) {
	if _, err := NewAppWindow(); err == nil {
		t.Fatal("NewAppWindow found a main window in the test process")
	}
}
