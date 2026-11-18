//go:build linux

package platform

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/shape"
	"github.com/jezek/xgb/xinerama"
	"github.com/jezek/xgb/xproto"
	"github.com/kbinani/screenshot"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Screen capture and the "screen viewed" notice on Linux: X11 only
// (docs/privacidad.md rules 96-97). In Wayland both fail with
// ErrNotSupported, so the agent declares neither screen.thumb nor
// screen.snapshot and never asks the desktop's permission portal.

func newScreenCapture() (ScreenCapture, error) {
	if err := needX11(); err != nil {
		return nil, err
	}
	return x11Capture{}, nil
}

type x11Capture struct{}

// Capture grabs the primary display: Xinerama's first screen, the one
// kbinani/screenshot measures from and where the notice sits.
func (x11Capture) Capture(ctx context.Context, maxWidth, quality int) (Frame, error) {
	if err := ctx.Err(); err != nil {
		return Frame{}, err
	}
	// The session can change under a running agent (the student logs into
	// Wayland after an X11 one is gone): never fall through to the portal.
	if err := needX11(); err != nil {
		return Frame{}, fmt.Errorf("capture: %w", err)
	}
	r := screenshot.GetDisplayBounds(0)
	if r.Empty() {
		return Frame{}, fmt.Errorf("capture: primary display has no size")
	}
	img, err := screenshot.CaptureRect(r)
	if err != nil {
		return Frame{}, fmt.Errorf("capture: %w", err)
	}
	return EncodeFrame(img, maxWidth, quality)
}

func newScreenIndicator() (ScreenIndicator, error) {
	if err := needX11(); err != nil {
		return nil, err
	}
	return &x11Indicator{}, nil
}

// x11Indicator is a click-through banner at the top centre of the primary
// display, kept above the other windows.
type x11Indicator struct {
	mu  sync.Mutex
	win *x11Banner
}

func (i *x11Indicator) Show(text string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.win != nil {
		if err := i.win.setText(text); err == nil {
			return nil
		}
		i.win.close() // the X server went away: open a new one
		i.win = nil
	}
	w, err := openBanner(text)
	if err != nil {
		return fmt.Errorf("screen indicator: %w", err)
	}
	i.win = w
	return nil
}

func (i *x11Indicator) Hide() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.win != nil {
		i.win.close()
		i.win = nil
	}
	return nil
}

// Size and colours of the banner, the same as on Windows (520×36 at 100 %,
// white text on red, 15 pt semibold).
const (
	bannerWidth  = 520
	bannerHeight = 36
	bannerPoints = 15
)

var (
	bannerBack = color.RGBA{0xb9, 0x1c, 0x1c, 0xff}
	bannerText = color.RGBA{0xff, 0xff, 0xff, 0xff}
)

// x11Banner owns its own connection: drawing it never waits on anything
// else Aula does with X.
type x11Banner struct {
	c      *xgb.Conn
	screen *xproto.ScreenInfo
	win    xproto.Window
	pix    xproto.Pixmap
	gc     xproto.Gcontext
	w, h   int
	scale  float64
	stop   chan struct{}
	done   chan struct{}
	once   sync.Once
}

func openBanner(text string) (*x11Banner, error) {
	c, screen, err := x11Connect()
	if err != nil {
		return nil, err
	}
	b := &x11Banner{c: c, screen: screen, stop: make(chan struct{}), done: make(chan struct{})}
	if err := b.create(text); err != nil {
		c.Close()
		return nil, err
	}
	go b.keepOnTop()
	return b, nil
}

// primaryBounds is the primary display in root coordinates: Xinerama's
// first screen, or the whole root window without Xinerama.
func primaryBounds(c *xgb.Conn, screen *xproto.ScreenInfo) image.Rectangle {
	if xinerama.Init(c) == nil {
		if r, err := xinerama.QueryScreens(c).Reply(); err == nil && len(r.ScreenInfo) > 0 {
			s := r.ScreenInfo[0]
			return image.Rect(int(s.XOrg), int(s.YOrg), int(s.XOrg)+int(s.Width), int(s.YOrg)+int(s.Height))
		}
	}
	return image.Rect(0, 0, int(screen.WidthInPixels), int(screen.HeightInPixels))
}

func (b *x11Banner) create(text string) error {
	c, s := b.c, b.screen
	if s.RootDepth != 24 && s.RootDepth != 32 {
		return fmt.Errorf("X11: root depth %d: %w", s.RootDepth, ErrNotSupported)
	}
	b.scale = x11Scale(c, s.Root)
	area := primaryBounds(c, s)
	b.w = min(int(bannerWidth*b.scale), area.Dx())
	b.h = int(bannerHeight * b.scale)
	x := area.Min.X + (area.Dx()-b.w)/2
	y := area.Min.Y

	win, err := xproto.NewWindowId(c)
	if err != nil {
		return fmt.Errorf("X11: %w", err)
	}
	b.win = win
	// Override-redirect: the window manager neither decorates, moves nor
	// focuses it, and it has no taskbar entry.
	if err := xproto.CreateWindowChecked(c, s.RootDepth, win, s.Root, int16(x), int16(y),
		uint16(b.w), uint16(b.h), 0, xproto.WindowClassInputOutput, s.RootVisual,
		xproto.CwBackPixel|xproto.CwOverrideRedirect,
		[]uint32{0xb91c1c, 1}).Check(); err != nil {
		return fmt.Errorf("X11: create window: %w", err)
	}
	b.setNames()
	// An empty input shape: clicks go to the window below.
	if shape.Init(c) == nil {
		_ = shape.RectanglesChecked(c, shape.SoSet, shape.SkInput, xproto.ClipOrderingUnsorted,
			win, 0, 0, nil).Check()
	}
	if b.pix, err = xproto.NewPixmapId(c); err != nil {
		return fmt.Errorf("X11: %w", err)
	}
	if err := xproto.CreatePixmapChecked(c, s.RootDepth, b.pix, xproto.Drawable(win), uint16(b.w), uint16(b.h)).Check(); err != nil {
		return fmt.Errorf("X11: create pixmap: %w", err)
	}
	if b.gc, err = xproto.NewGcontextId(c); err != nil {
		return fmt.Errorf("X11: %w", err)
	}
	if err := xproto.CreateGCChecked(c, b.gc, xproto.Drawable(b.pix), 0, nil).Check(); err != nil {
		return fmt.Errorf("X11: create GC: %w", err)
	}
	if err := b.draw(text); err != nil {
		return err
	}
	if err := xproto.MapWindowChecked(c, win).Check(); err != nil {
		return fmt.Errorf("X11: map window: %w", err)
	}
	b.raise()
	return nil
}

// setNames gives the window a title and class, so it reads as Aula in any
// tool that lists windows: it never hides what it is.
func (b *x11Banner) setNames() {
	const title, class = "Aula", "aula-cliente\x00Aula\x00"
	xproto.ChangeProperty(b.c, xproto.PropModeReplace, b.win, xproto.AtomWmName, xproto.AtomString,
		8, uint32(len(title)), []byte(title))
	xproto.ChangeProperty(b.c, xproto.PropModeReplace, b.win, xproto.AtomWmClass, xproto.AtomString,
		8, uint32(len(class)), []byte(class))
}

// draw paints the text into the pixmap and makes it the window background:
// the X server repaints it on every expose without asking Aula.
func (b *x11Banner) draw(text string) error {
	img := bannerImage(text, b.w, b.h, b.scale)
	if err := putImage(b.c, xproto.Drawable(b.pix), b.gc, b.screen.RootDepth, img); err != nil {
		return err
	}
	if err := xproto.ChangeWindowAttributesChecked(b.c, b.win, xproto.CwBackPixmap,
		[]uint32{uint32(b.pix)}).Check(); err != nil {
		return fmt.Errorf("X11: set background: %w", err)
	}
	return xproto.ClearAreaChecked(b.c, false, b.win, 0, 0, 0, 0).Check()
}

func (b *x11Banner) setText(text string) error { return b.draw(text) }

func (b *x11Banner) raise() {
	xproto.ConfigureWindow(b.c, b.win, xproto.ConfigWindowStackMode, []uint32{xproto.StackModeAbove})
}

// keepOnTop puts the banner back above every other window every 2 s, as on
// Windows, until close.
func (b *x11Banner) keepOnTop() {
	defer close(b.done)
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-t.C:
			b.raise()
			_, _ = xproto.GetInputFocus(b.c).Reply() // flushes; fails if X is gone
		}
	}
}

func (b *x11Banner) close() {
	b.once.Do(func() {
		close(b.stop)
		<-b.done
		xproto.DestroyWindow(b.c, b.win)
		xproto.FreePixmap(b.c, b.pix)
		xproto.FreeGC(b.c, b.gc)
		_, _ = xproto.GetInputFocus(b.c).Reply() // makes sure the destroy left
		b.c.Close()
	})
}

// putImage uploads img (ZPixmap, 32 bits per pixel, BGRX as the X server
// stores 24- and 32-bit visuals on little-endian machines) in strips that
// fit one request.
func putImage(c *xgb.Conn, d xproto.Drawable, gc xproto.Gcontext, depth byte, img *image.RGBA) error {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	rowBytes := w * 4
	// Requests are at most 256 KiB without BIG-REQUESTS; leave room for the header.
	rows := max(1, (256*1024-64)/rowBytes)
	buf := make([]byte, 0, rows*rowBytes)
	for y0 := 0; y0 < h; y0 += rows {
		n := min(rows, h-y0)
		buf = buf[:0]
		for y := y0; y < y0+n; y++ {
			line := img.Pix[y*img.Stride : y*img.Stride+rowBytes]
			for x := 0; x < rowBytes; x += 4 {
				buf = append(buf, line[x+2], line[x+1], line[x], 0xff)
			}
		}
		if err := xproto.PutImageChecked(c, xproto.ImageFormatZPixmap, d, gc, uint16(w), uint16(n),
			0, int16(y0), 0, depth, buf).Check(); err != nil {
			return fmt.Errorf("X11: put image: %w", err)
		}
	}
	return nil
}

var (
	bannerFontOnce sync.Once
	bannerFont     *opentype.Font
)

// bannerImage draws the notice: a dot and the text, centred, white on red,
// cut with "…" if it does not fit. The Go font ships inside the binary, so
// the notice looks the same on any distribution.
func bannerImage(text string, w, h int, scale float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(bannerBack), image.Point{}, draw.Src)
	bannerFontOnce.Do(func() { bannerFont, _ = opentype.Parse(gobold.TTF) })
	if bannerFont == nil {
		return img
	}
	face, err := opentype.NewFace(bannerFont, &opentype.FaceOptions{Size: bannerPoints * scale, DPI: 96, Hinting: font.HintingFull})
	if err != nil {
		return img
	}
	defer face.Close()
	m := face.Metrics()
	dot := (m.Ascent - m.Descent).Ceil() * 2 / 3
	gap := int(8 * scale)
	pad := int(12 * scale)
	text = fitText(face, text, w-2*pad-dot-gap)
	tw := font.MeasureString(face, text).Ceil()
	x := (w - (dot + gap + tw)) / 2
	baseline := (h+(m.Ascent-m.Descent).Ceil())/2 - 1
	fillCircle(img, x+dot/2, h/2, dot/2, bannerText)
	d := font.Drawer{Dst: img, Src: image.NewUniform(bannerText), Face: face,
		Dot: fixed.P(x+dot+gap, baseline)}
	d.DrawString(text)
	return img
}

// fitText cuts s with "…" until it is at most width pixels wide.
func fitText(face font.Face, s string, width int) string {
	if font.MeasureString(face, s).Ceil() <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		if t := string(r) + "…"; font.MeasureString(face, t).Ceil() <= width {
			return t
		}
	}
	return ""
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				img.SetRGBA(cx+x, cy+y, c)
			}
		}
	}
}
