//go:build linux

package platform

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xinerama"
	"github.com/jezek/xgb/xproto"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The pause window on Linux (docs/privacidad.md rules 9-11 and 101): one
// override-redirect window per monitor in an X11 session, put back above the
// others every half second, that takes the clicks and the focus once. In
// Wayland it fails with ErrNotSupported and the agent does not declare
// screen.freeze. It never grabs the keyboard (D-013).

func newPauseOverlay(look PauseLook) (PauseOverlay, error) {
	if err := needX11(); err != nil {
		return nil, err
	}
	return &x11Pause{look: look}, nil
}

type x11Pause struct {
	look PauseLook

	op sync.Mutex // one Show, Yield or Hide at a time
	mu sync.Mutex
	c  *xgb.Conn
	sc *xproto.ScreenInfo
	ws []*pauseWin

	text   PauseText
	yield  bool
	stop   chan struct{}
	done   chan struct{}
	minute int
}

type pauseWin struct {
	win  xproto.Window
	pix  xproto.Pixmap
	gc   xproto.Gcontext
	area image.Rectangle
}

func (p *x11Pause) Show(t PauseText) error {
	p.op.Lock()
	defer p.op.Unlock()
	p.mu.Lock()
	p.text = t
	up := p.c != nil
	p.mu.Unlock()
	if up {
		return p.redraw() // only the text changes; the windows stay
	}
	c, sc, err := x11Connect()
	if err != nil {
		return fmt.Errorf("pause: %w", err)
	}
	if sc.RootDepth != 24 && sc.RootDepth != 32 {
		c.Close()
		return fmt.Errorf("pause: root depth %d: %w", sc.RootDepth, ErrNotSupported)
	}
	p.mu.Lock()
	p.c, p.sc = c, sc
	p.stop, p.done = make(chan struct{}), make(chan struct{})
	p.mu.Unlock()
	for _, area := range monitors(c, sc) {
		w, err := p.open(area)
		if err != nil {
			p.closeAll()
			return fmt.Errorf("pause: %w", err)
		}
		p.mu.Lock()
		p.ws = append(p.ws, w)
		p.mu.Unlock()
	}
	if err := p.redraw(); err != nil {
		p.closeAll()
		return err
	}
	for _, w := range p.ws {
		xproto.MapWindow(c, w.win)
	}
	p.raise()
	p.focus()
	go p.loop()
	return nil
}

// monitors are the screens of Xinerama, or the root window without it.
func monitors(c *xgb.Conn, sc *xproto.ScreenInfo) []image.Rectangle {
	if xinerama.Init(c) == nil {
		if r, err := xinerama.QueryScreens(c).Reply(); err == nil && len(r.ScreenInfo) > 0 {
			var out []image.Rectangle
			for _, s := range r.ScreenInfo {
				out = append(out, image.Rect(int(s.XOrg), int(s.YOrg), int(s.XOrg)+int(s.Width), int(s.YOrg)+int(s.Height)))
			}
			return out
		}
	}
	return []image.Rectangle{image.Rect(0, 0, int(sc.WidthInPixels), int(sc.HeightInPixels))}
}

func (p *x11Pause) open(area image.Rectangle) (*pauseWin, error) {
	c, sc := p.c, p.sc
	w := &pauseWin{area: area}
	var err error
	if w.win, err = xproto.NewWindowId(c); err != nil {
		return nil, err
	}
	// Override-redirect: no frame, no taskbar entry, not moved by the window
	// manager. It takes the clicks (ButtonPress gives it the focus back).
	if err := xproto.CreateWindowChecked(c, sc.RootDepth, w.win, sc.Root, int16(area.Min.X), int16(area.Min.Y),
		uint16(area.Dx()), uint16(area.Dy()), 0, xproto.WindowClassInputOutput, sc.RootVisual,
		xproto.CwBackPixel|xproto.CwOverrideRedirect|xproto.CwEventMask,
		[]uint32{0, 1, xproto.EventMaskButtonPress}).Check(); err != nil {
		return nil, fmt.Errorf("create window: %w", err)
	}
	const title, class = "Aula", "aula-cliente\x00Aula\x00"
	xproto.ChangeProperty(c, xproto.PropModeReplace, w.win, xproto.AtomWmName, xproto.AtomString, 8, uint32(len(title)), []byte(title))
	xproto.ChangeProperty(c, xproto.PropModeReplace, w.win, xproto.AtomWmClass, xproto.AtomString, 8, uint32(len(class)), []byte(class))
	if w.pix, err = xproto.NewPixmapId(c); err != nil {
		return nil, err
	}
	if err := xproto.CreatePixmapChecked(c, sc.RootDepth, w.pix, xproto.Drawable(w.win), uint16(area.Dx()), uint16(area.Dy())).Check(); err != nil {
		return nil, fmt.Errorf("create pixmap: %w", err)
	}
	if w.gc, err = xproto.NewGcontextId(c); err != nil {
		return nil, err
	}
	if err := xproto.CreateGCChecked(c, w.gc, xproto.Drawable(w.pix), 0, nil).Check(); err != nil {
		return nil, fmt.Errorf("create GC: %w", err)
	}
	return w, nil
}

// redraw paints the text on every monitor, in the theme of the desktop.
func (p *x11Pause) redraw() error {
	p.mu.Lock()
	c, sc, ws, t := p.c, p.sc, p.ws, p.text
	p.minute = time.Now().Minute()
	p.mu.Unlock()
	if c == nil {
		return nil
	}
	colors := p.look.Light
	if desktopDark() {
		colors = p.look.Dark
	}
	scale := x11Scale(c, sc.Root)
	for _, w := range ws {
		img := pauseImage(t, colors, w.area.Dx(), w.area.Dy(), scale, time.Now())
		if err := putImage(c, xproto.Drawable(w.pix), w.gc, sc.RootDepth, img); err != nil {
			return fmt.Errorf("pause: %w", err)
		}
		if err := xproto.ChangeWindowAttributesChecked(c, w.win, xproto.CwBackPixmap, []uint32{uint32(w.pix)}).Check(); err != nil {
			return fmt.Errorf("pause: %w", err)
		}
		xproto.ClearArea(c, false, w.win, 0, 0, 0, 0)
	}
	_, err := xproto.GetInputFocus(c).Reply()
	return err
}

// desktopDark reads the GNOME colour scheme (also used by most GTK desktops).
func desktopDark() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := runCommand(ctx, "gsettings", "get", "org.gnome.desktop.interface", "color-scheme")
	return err == nil && strings.Contains(out, "dark")
}

// raise puts the windows above everything, or right below Aula's own window
// while it yields.
func (p *x11Pause) raise() {
	p.mu.Lock()
	c, sc, ws, yield := p.c, p.sc, p.ws, p.yield
	p.mu.Unlock()
	if c == nil {
		return
	}
	var below xproto.Window
	if yield {
		below = ownFrame(c, sc.Root)
	}
	for _, w := range ws {
		switch {
		case !yield:
			xproto.ConfigureWindow(c, w.win, xproto.ConfigWindowStackMode, []uint32{xproto.StackModeAbove})
		case below != 0:
			xproto.ConfigureWindow(c, w.win, xproto.ConfigWindowSibling|xproto.ConfigWindowStackMode,
				[]uint32{uint32(below), xproto.StackModeBelow})
		default:
			// Aula's window was not found: out of the way, so what the
			// student must see is not covered.
			xproto.ConfigureWindow(c, w.win, xproto.ConfigWindowStackMode, []uint32{xproto.StackModeBelow})
		}
	}
}

// focus gives the focus to the first window, once (it is not kept: the
// keyboard is never captured).
func (p *x11Pause) focus() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.c != nil && len(p.ws) > 0 && !p.yield {
		xproto.SetInputFocus(p.c, xproto.InputFocusParent, p.ws[0].win, xproto.TimeCurrentTime)
	}
}

// ownFrame is the top-level window (the frame the window manager put around
// it) of this process's main window, found by _NET_WM_PID; 0 if none.
func ownFrame(c *xgb.Conn, root xproto.Window) xproto.Window {
	list := atom(c, "_NET_CLIENT_LIST")
	pidAtom := atom(c, "_NET_WM_PID")
	if list == 0 || pidAtom == 0 {
		return 0
	}
	r, err := xproto.GetProperty(c, false, root, list, xproto.AtomWindow, 0, 4096).Reply()
	if err != nil {
		return 0
	}
	pid := uint32(os.Getpid())
	for i := 0; i+4 <= len(r.Value); i += 4 {
		w := xproto.Window(xgb.Get32(r.Value[i:]))
		pr, err := xproto.GetProperty(c, false, w, pidAtom, xproto.AtomCardinal, 0, 1).Reply()
		if err != nil || len(pr.Value) < 4 || xgb.Get32(pr.Value) != pid {
			continue
		}
		for { // up to the child of the root
			t, err := xproto.QueryTree(c, w).Reply()
			if err != nil {
				return 0
			}
			if t.Parent == root || t.Parent == 0 {
				return w
			}
			w = t.Parent
		}
	}
	return 0
}

func atom(c *xgb.Conn, name string) xproto.Atom {
	r, err := xproto.InternAtom(c, true, uint16(len(name)), name).Reply()
	if err != nil {
		return 0
	}
	return r.Atom
}

// loop keeps the windows on top every 0.5 s, gives back the focus when the
// student clicks them, and redraws the clock each minute.
func (p *x11Pause) loop() {
	p.mu.Lock()
	c, stop, done := p.c, p.stop, p.done
	p.mu.Unlock()
	defer close(done)
	clicks := make(chan struct{}, 1)
	go func() {
		for {
			ev, err := c.WaitForEvent()
			if ev == nil && err == nil {
				return // closed
			}
			if _, ok := ev.(xproto.ButtonPressEvent); ok {
				select {
				case clicks <- struct{}{}:
				default:
				}
			}
		}
	}()
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-clicks:
			p.focus()
		case <-t.C:
			p.raise()
			p.mu.Lock()
			minute := p.minute
			p.mu.Unlock()
			// TryLock: Hide holds op while it waits for this loop to end.
			if time.Now().Minute() != minute && p.op.TryLock() {
				_ = p.redraw()
				p.op.Unlock()
			}
		}
	}
}

func (p *x11Pause) Yield(on bool) error {
	p.op.Lock()
	defer p.op.Unlock()
	p.mu.Lock()
	p.yield = on
	p.mu.Unlock()
	p.raise()
	if !on {
		p.focus()
	}
	return nil
}

func (p *x11Pause) Hide() error {
	p.op.Lock()
	defer p.op.Unlock()
	p.closeAll()
	return nil
}

func (p *x11Pause) closeAll() {
	p.mu.Lock()
	c, ws, stop, done := p.c, p.ws, p.stop, p.done
	p.c, p.ws, p.yield = nil, nil, false
	p.mu.Unlock()
	if c == nil {
		return
	}
	if stop != nil {
		close(stop)
	}
	for _, w := range ws {
		xproto.DestroyWindow(c, w.win)
		xproto.FreePixmap(c, w.pix)
		xproto.FreeGC(c, w.gc)
	}
	_, _ = xproto.GetInputFocus(c).Reply()
	c.Close() // ends the event reader
	if done != nil {
		<-done
	}
}

var (
	pauseFontsOnce          sync.Once
	pauseRegular, pauseBold *opentype.Font
)

// pauseImage lays the text out centred, like Windows: the pause mark, the
// title, the teacher's message in a frame, who paused it, the note and the
// clock (HH:MM). Text wraps at 720 px (scaled) and is centred line by line.
func pauseImage(t PauseText, colors PauseColors, w, h int, scale float64, now time.Time) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(colors.Background), image.Point{}, draw.Src)
	pauseFontsOnce.Do(func() {
		pauseRegular, _ = opentype.Parse(goregular.TTF)
		pauseBold, _ = opentype.Parse(gobold.TTF)
	})
	if pauseRegular == nil || pauseBold == nil {
		return img
	}
	px := func(v float64) int { return int(v * scale) }
	width := min(w-2*px(48), px(720))
	type block struct {
		text   string
		pt     float64
		bold   bool
		gap    int
		framed bool
	}
	blocks := []block{{t.Title, 28, true, px(24), false}}
	if t.Message != "" {
		blocks = append(blocks, block{"«" + t.Message + "»", 18, false, px(20), true})
	}
	blocks = append(blocks,
		block{t.Detail, 16, false, px(20), false},
		block{t.Note, 14, false, px(12), false},
		block{now.Format("15:04"), 14, true, px(24), false})

	frame := px(14)
	type laid struct {
		face  font.Face
		lines []string
		lineH int
		h     int
	}
	out := make([]laid, len(blocks))
	mark := px(56)
	total := mark
	for i, b := range blocks {
		f := pauseRegular
		if b.bold {
			f = pauseBold
		}
		face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: b.pt * scale, DPI: 96, Hinting: font.HintingFull})
		if err != nil {
			return img
		}
		defer face.Close()
		inner := width
		if b.framed {
			inner -= 2 * frame
		}
		lines := wrapText(face, b.text, inner)
		lineH := face.Metrics().Height.Ceil()
		hh := lineH * len(lines)
		if b.framed {
			hh += 2 * frame
		}
		out[i] = laid{face, lines, lineH, hh}
		total += b.gap + hh
	}
	y := (h - total) / 2
	cx := w / 2
	fg := image.NewUniform(colors.Text)
	fillRect := func(r image.Rectangle) { draw.Draw(img, r, fg, image.Point{}, draw.Src) }

	// The pause mark: two bars.
	bar, gap := px(16), px(14)
	fillRect(image.Rect(cx-gap/2-bar, y, cx-gap/2, y+mark))
	fillRect(image.Rect(cx+gap/2, y, cx+gap/2+bar, y+mark))
	y += mark

	left := cx - width/2
	for i, b := range blocks {
		y += b.gap
		l := out[i]
		top := y
		if b.framed {
			line := max(px(2), 1)
			fillRect(image.Rect(left, y, left+width, y+line))
			fillRect(image.Rect(left, y+l.h-line, left+width, y+l.h))
			fillRect(image.Rect(left, y, left+line, y+l.h))
			fillRect(image.Rect(left+width-line, y, left+width, y+l.h))
			top += frame
		}
		ascent := l.face.Metrics().Ascent.Ceil()
		for j, line := range l.lines {
			lw := font.MeasureString(l.face, line).Ceil()
			d := font.Drawer{Dst: img, Src: fg, Face: l.face,
				Dot: fixed.P(cx-lw/2, top+j*l.lineH+ascent)}
			d.DrawString(line)
		}
		y += l.h
	}
	return img
}

// wrapText breaks s into lines at most width pixels wide, at spaces; a word
// longer than the line is cut with "…".
func wrapText(face font.Face, s string, width int) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		cur := ""
		for _, word := range strings.Fields(para) {
			if font.MeasureString(face, word).Ceil() > width {
				word = fitText(face, word, width)
			}
			try := word
			if cur != "" {
				try = cur + " " + word
			}
			if font.MeasureString(face, try).Ceil() <= width {
				cur = try
				continue
			}
			if cur != "" {
				lines = append(lines, cur)
			}
			cur = word
		}
		lines = append(lines, cur)
	}
	return lines
}
