//go:build linux

package platform

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

// needBus skips without a session bus: the tests run under dbus-run-session
// (CI, the container), never on the student's desktop session.
func needBus(t *testing.T) {
	t.Helper()
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus (run under dbus-run-session)")
	}
}

func testPNG(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for i := range 32 * 32 {
		img.SetRGBA(i%32, i/32, c)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPNGPixmap(t *testing.T) {
	p, err := pngPixmap(testPNG(t, color.RGBA{0x10, 0x20, 0x30, 0xff}))
	if err != nil {
		t.Fatal(err)
	}
	if p[0].W != 32 || p[0].H != 32 || len(p[0].Data) != 32*32*4 {
		t.Fatalf("pixmap %dx%d, %d bytes", p[0].W, p[0].H, len(p[0].Data))
	}
	if got := p[0].Data[:4]; got[0] != 0xff || got[1] != 0x10 || got[2] != 0x20 || got[3] != 0x30 {
		t.Fatalf("first pixel ARGB = %v", got)
	}
	if _, err := pngPixmap([]byte("no es png")); err == nil {
		t.Fatal("not a PNG accepted")
	}
}

// fakeWatcher is a notification area host on the test bus.
type fakeWatcher struct{ registered chan string }

func (w *fakeWatcher) RegisterStatusNotifierItem(service string) *dbus.Error {
	w.registered <- service
	return nil
}

func startWatcher(t *testing.T, host bool) *fakeWatcher {
	t.Helper()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if r, err := conn.RequestName(sniWatcher, dbus.NameFlagDoNotQueue); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("watcher name: %v %v", r, err)
	}
	w := &fakeWatcher{registered: make(chan string, 4)}
	if err := conn.Export(w, sniWatcherPath, sniWatcher); err != nil {
		t.Fatal(err)
	}
	if _, err := prop.Export(conn, sniWatcherPath, prop.Map{sniWatcher: {
		"IsStatusNotifierHostRegistered": {Value: host, Emit: prop.EmitConst},
	}}); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestTrayWithoutHost(t *testing.T) {
	needBus(t)
	icon := testPNG(t, color.RGBA{0, 0, 0xff, 0xff})
	if _, err := NewTray(TrayOptions{Icon: icon, MarkedIcon: icon}); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("no watcher: %v, want ErrNotSupported (the window keeps minimizing)", err)
	}
	startWatcher(t, false) // a watcher, but no panel showing items
	if _, err := NewTray(TrayOptions{Icon: icon, MarkedIcon: icon}); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("no host: %v, want ErrNotSupported", err)
	}
}

func TestTrayOnTheBus(t *testing.T) {
	needBus(t)
	w := startWatcher(t, true)
	opened := make(chan struct{}, 4)
	tray, err := NewTray(TrayOptions{Icon: testPNG(t, color.RGBA{0, 0, 0xff, 0xff}),
		MarkedIcon: testPNG(t, color.RGBA{0xff, 0, 0, 0xff}), Tooltip: "Aula",
		OpenLabel: "Abrir Aula", OnOpen: func() { opened <- struct{}{} }})
	if err != nil {
		t.Fatal(err)
	}
	defer tray.Close()
	var name string
	select {
	case name = <-w.registered:
	case <-time.After(3 * time.Second):
		t.Fatal("the item did not register with the watcher")
	}

	host, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	item := host.Object(name, sniPath)
	waitOpen := func(what string) {
		t.Helper()
		select {
		case <-opened:
		case <-time.After(3 * time.Second):
			t.Fatalf("%s did not open the window", what)
		}
	}
	if err := item.Call(sniIface+".Activate", 0, int32(0), int32(0)).Err; err != nil {
		t.Fatal(err)
	}
	waitOpen("a click")

	if err := tray.SetMarked(true); err != nil {
		t.Fatal(err)
	}
	if v, err := item.GetProperty(sniIface + ".Status"); err != nil || v.Value() != "NeedsAttention" {
		t.Fatalf("status after SetMarked = %v, %v", v, err)
	}
	var pix []sniPixmap
	if v, err := item.GetProperty(sniIface + ".IconPixmap"); err != nil || v.Store(&pix) != nil || pix[0].Data[1] != 0xff {
		t.Fatalf("marked icon = %v, %v", pix, err)
	}
	if err := tray.SetTooltip("Aula · conectado al canal 1"); err != nil {
		t.Fatal(err)
	}

	// The menu has «Abrir Aula», and choosing it opens the window.
	var rev uint32
	var layout menuLayout
	if err := host.Object(name, menuPath).Call(menuIface+".GetLayout", 0, int32(0), int32(-1), []string{}).Store(&rev, &layout); err != nil {
		t.Fatal(err)
	}
	if len(layout.Children) != 1 {
		t.Fatalf("menu = %+v", layout)
	}
	var entry menuLayout
	if err := dbus.Store([]any{layout.Children[0].Value()}, &entry); err != nil || entry.Props["label"].Value() != "Abrir Aula" {
		t.Fatalf("entry = %+v, %v", entry, err)
	}
	if err := host.Object(name, menuPath).Call(menuIface+".Event", 0, int32(menuOpenID), "clicked", dbus.MakeVariant(""), uint32(0)).Err; err != nil {
		t.Fatal(err)
	}
	waitOpen("«Abrir Aula»")

	if err := tray.Close(); err != nil {
		t.Fatal(err)
	}
	var has bool
	if err := host.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, name).Store(&has); err != nil || has {
		t.Fatalf("the item is still on the bus after Close: %v %v", has, err)
	}
	if err := tray.Close(); err != nil {
		t.Errorf("second Close = %v", err)
	}
}
