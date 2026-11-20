//go:build linux

package platform

import (
	"bytes"
	"fmt"

	"image/png"
	"os"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// The notification area icon on Linux (docs/privacidad.md rule 2): a
// StatusNotifierItem on the session bus, with a one-entry menu
// (com.canonical.dbusmenu). GNOME shows it with the AppIndicator extension
// (Ubuntu ships it), KDE, XFCE and Cinnamon natively. Without a host that
// shows these icons NewTray fails with ErrNotSupported, so the window keeps
// minimizing instead of hiding where the student could not find it again.

const (
	sniWatcher     = "org.kde.StatusNotifierWatcher"
	sniWatcherPath = dbus.ObjectPath("/StatusNotifierWatcher")
	sniIface       = "org.kde.StatusNotifierItem"
	sniPath        = dbus.ObjectPath("/StatusNotifierItem")
	menuIface      = "com.canonical.dbusmenu"
	menuPath       = dbus.ObjectPath("/MenuBar")
	menuOpenID     = 1
)

// sessionBus opens a private connection to the session bus; replaced in
// tests.
var sessionBus = func() (*dbus.Conn, error) { return dbus.ConnectSessionBus() }

// sniPixmap is one icon image: width, height and ARGB32 pixels in network
// byte order (a(iiay) on the bus).
type sniPixmap struct {
	W, H int32
	Data []byte
}

// sniToolTip is (sa(iiay)ss): icon name, images, title and text.
type sniToolTip struct {
	Icon  string
	Pix   []sniPixmap
	Title string
	Text  string
}

func newTray(o TrayOptions) (Tray, error) {
	normal, err := pngPixmap(o.Icon)
	if err != nil {
		return nil, fmt.Errorf("tray: %w", err)
	}
	marked, err := pngPixmap(o.MarkedIcon)
	if err != nil {
		return nil, fmt.Errorf("tray: %w", err)
	}
	conn, err := sessionBus()
	if err != nil {
		return nil, fmt.Errorf("tray: no session bus: %w", ErrNotSupported)
	}
	if !hostRegistered(conn) {
		conn.Close()
		return nil, fmt.Errorf("tray: no notification area host: %w", ErrNotSupported)
	}
	t := &sniTray{conn: conn, normal: normal, marked: marked, onOpen: o.OnOpen,
		label: o.OpenLabel, tooltip: o.Tooltip, name: fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())}
	if err := t.export(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("tray: %w", err)
	}
	if err := t.register(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("tray: %w", err)
	}
	go t.followWatcher()
	return t, nil
}

// hostRegistered: a watcher exists and some host (the panel) shows items.
func hostRegistered(conn *dbus.Conn) bool {
	v, err := conn.Object(sniWatcher, sniWatcherPath).GetProperty(sniWatcher + ".IsStatusNotifierHostRegistered")
	if err != nil {
		return false
	}
	on, ok := v.Value().(bool)
	return ok && on
}

// pngPixmap decodes a PNG into the ARGB32 (big-endian) image of the bus.
func pngPixmap(data []byte) ([]sniPixmap, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	}
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA() // premultiplied, 16 bits
			if a > 0 {                         // the bus wants straight alpha
				r, g, bl = r*0xffff/a, g*0xffff/a, bl*0xffff/a
			}
			out = append(out, byte(a>>8), byte(r>>8), byte(g>>8), byte(bl>>8))
		}
	}
	return []sniPixmap{{W: int32(b.Dx()), H: int32(b.Dy()), Data: out}}, nil
}

type sniTray struct {
	conn           *dbus.Conn
	name           string
	normal, marked []sniPixmap
	onOpen         func()
	label          string

	mu      sync.Mutex
	tooltip string
	isMark  bool
	closed  bool
	props   *prop.Properties
}

// sniItem is the exported org.kde.StatusNotifierItem.
type sniItem struct{ t *sniTray }

func (i sniItem) Activate(x, y int32) *dbus.Error          { i.t.open(); return nil }
func (i sniItem) SecondaryActivate(x, y int32) *dbus.Error { i.t.open(); return nil }
func (i sniItem) ContextMenu(x, y int32) *dbus.Error       { return nil }
func (i sniItem) Scroll(delta int32, orientation string) *dbus.Error {
	return nil
}

func (t *sniTray) open() {
	if t.onOpen != nil {
		go t.onOpen()
	}
}

func (t *sniTray) tooltipValue() sniToolTip {
	return sniToolTip{Title: t.tooltip}
}

func (t *sniTray) export() error {
	if reply, err := t.conn.RequestName(t.name, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("bus name %s: %v", t.name, err)
	}
	item := sniItem{t}
	if err := t.conn.Export(item, sniPath, sniIface); err != nil {
		return err
	}
	props, err := prop.Export(t.conn, sniPath, prop.Map{sniIface: {
		"Category":            {Value: "ApplicationStatus", Emit: prop.EmitConst},
		"Id":                  {Value: "aula-cliente", Emit: prop.EmitConst},
		"Title":               {Value: "Aula", Emit: prop.EmitConst},
		"Status":              {Value: "Active", Emit: prop.EmitTrue},
		"IconName":            {Value: "", Emit: prop.EmitConst},
		"IconPixmap":          {Value: t.normal, Emit: prop.EmitTrue},
		"AttentionIconPixmap": {Value: t.marked, Emit: prop.EmitConst},
		"ToolTip":             {Value: t.tooltipValue(), Emit: prop.EmitTrue},
		"ItemIsMenu":          {Value: false, Emit: prop.EmitConst},
		"Menu":                {Value: menuPath, Emit: prop.EmitConst},
	}})
	if err != nil {
		return err
	}
	t.props = props
	menu := &dbusMenu{t: t}
	if err := t.conn.Export(menu, menuPath, menuIface); err != nil {
		return err
	}
	if _, err := prop.Export(t.conn, menuPath, prop.Map{menuIface: {
		"Version":       {Value: uint32(3), Emit: prop.EmitConst},
		"TextDirection": {Value: "ltr", Emit: prop.EmitConst},
		"Status":        {Value: "normal", Emit: prop.EmitConst},
		"IconThemePath": {Value: []string{}, Emit: prop.EmitConst},
	}}); err != nil {
		return err
	}
	node := &introspect.Node{Name: string(sniPath), Interfaces: []introspect.Interface{
		introspect.IntrospectData, prop.IntrospectData,
		{Name: sniIface, Methods: introspect.Methods(item), Properties: props.Introspection(sniIface),
			Signals: []introspect.Signal{{Name: "NewIcon"}, {Name: "NewToolTip"}, {Name: "NewStatus", Args: []introspect.Arg{{Name: "status", Type: "s"}}}}},
	}}
	return t.conn.Export(introspect.NewIntrospectable(node), sniPath, "org.freedesktop.DBus.Introspectable")
}

// register tells the watcher about the item.
func (t *sniTray) register() error {
	return t.conn.Object(sniWatcher, sniWatcherPath).Call(sniWatcher+".RegisterStatusNotifierItem", 0, t.name).Err
}

// followWatcher registers again when the watcher comes back (the panel or
// the shell restarted), until Close.
func (t *sniTray) followWatcher() {
	if err := t.conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, sniWatcher)); err != nil {
		return
	}
	ch := make(chan *dbus.Signal, 8)
	t.conn.Signal(ch)
	for s := range ch {
		if len(s.Body) == 3 {
			if newOwner, _ := s.Body[2].(string); newOwner != "" {
				_ = t.register()
			}
		}
	}
}

func (t *sniTray) SetTooltip(text string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.tooltip = text
	t.props.SetMust(sniIface, "ToolTip", t.tooltipValue())
	return t.conn.Emit(sniPath, sniIface+".NewToolTip")
}

// SetMarked switches the image (and the status, for hosts that show
// attention) and tells the host.
func (t *sniTray) SetMarked(marked bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.isMark == marked {
		return nil
	}
	t.isMark = marked
	icon, status := t.normal, "Active"
	if marked {
		icon, status = t.marked, "NeedsAttention"
	}
	t.props.SetMust(sniIface, "IconPixmap", icon)
	t.props.SetMust(sniIface, "Status", status)
	if err := t.conn.Emit(sniPath, sniIface+".NewIcon"); err != nil {
		return err
	}
	return t.conn.Emit(sniPath, sniIface+".NewStatus", status)
}

func (t *sniTray) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	_, _ = t.conn.ReleaseName(t.name)
	return t.conn.Close() // the host drops an item whose name is gone
}

// dbusMenu is the item's menu: one entry that opens the window.
type dbusMenu struct {
	t        *sniTray
	revision uint32
}

// menuLayout is (ia{sv}av).
type menuLayout struct {
	ID       int32
	Props    map[string]dbus.Variant
	Children []dbus.Variant
}

func (m *dbusMenu) entry() menuLayout {
	return menuLayout{ID: menuOpenID, Props: map[string]dbus.Variant{
		"label": dbus.MakeVariant(m.t.label), "enabled": dbus.MakeVariant(true), "visible": dbus.MakeVariant(true),
	}, Children: []dbus.Variant{}}
}

func (m *dbusMenu) GetLayout(parent, depth int32, names []string) (uint32, menuLayout, *dbus.Error) {
	if parent == menuOpenID {
		return m.revision, m.entry(), nil
	}
	root := menuLayout{ID: 0, Props: map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")},
		Children: []dbus.Variant{dbus.MakeVariant(m.entry())}}
	return m.revision, root, nil
}

type menuProps struct {
	ID    int32
	Props map[string]dbus.Variant
}

func (m *dbusMenu) GetGroupProperties(ids []int32, names []string) ([]menuProps, *dbus.Error) {
	var out []menuProps
	for _, id := range ids {
		if id == menuOpenID {
			out = append(out, menuProps{ID: id, Props: m.entry().Props})
		}
	}
	return out, nil
}

func (m *dbusMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	if v, ok := m.entry().Props[name]; ok && id == menuOpenID {
		return v, nil
	}
	return dbus.MakeVariant(""), nil
}

func (m *dbusMenu) Event(id int32, event string, data dbus.Variant, ts uint32) *dbus.Error {
	if id == menuOpenID && event == "clicked" {
		m.t.open()
	}
	return nil
}

type menuEvent struct {
	ID    int32
	Event string
	Data  dbus.Variant
	TS    uint32
}

func (m *dbusMenu) EventGroup(events []menuEvent) ([]int32, *dbus.Error) {
	for _, e := range events {
		_ = m.Event(e.ID, e.Event, e.Data, e.TS)
	}
	return []int32{}, nil
}

func (m *dbusMenu) AboutToShow(id int32) (bool, *dbus.Error) { return false, nil }

func (m *dbusMenu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	return []int32{}, []int32{}, nil
}
