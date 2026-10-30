package simclass

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/bayalad7/aula/internal/platform"
)

// Size of the drawn screen before scaling: small enough to draw quickly,
// scaled up so the detail view is not blurry.
const (
	canvasW = 640
	canvasH = 360
	scale   = 2
)

// drawnScreen is the simulated student's "screen": its Aula window with the
// code it received, under the notice that the teacher is viewing it (the
// notice is on whenever a stream runs, as on a real computer).
type drawnScreen struct{ s *student }

// Capture implements platform.ScreenCapture.
func (d drawnScreen) Capture(_ context.Context, maxWidth, quality int) (platform.Frame, error) {
	return platform.EncodeFrame(d.draw(), maxWidth, quality)
}

// viewedFlag is the simulated "screen viewed" indicator.
type viewedFlag struct{ s *student }

// Show implements platform.ScreenIndicator.
func (v viewedFlag) Show(string) error { v.s.setViewed(true); return nil }

// Hide implements platform.ScreenIndicator.
func (v viewedFlag) Hide() error { v.s.setViewed(false); return nil }

var (
	colText    = color.RGBA{0xe7, 0xea, 0xf0, 0xff}
	colMuted   = color.RGBA{0xa7, 0xb0, 0xbf, 0xff}
	colWindow  = color.RGBA{0x1a, 0x1f, 0x29, 0xff}
	colTitle   = color.RGBA{0x23, 0x2a, 0x37, 0xff}
	colCode    = color.RGBA{0x9c, 0xdc, 0xfe, 0xff}
	colViewed  = color.RGBA{0xc8, 0x32, 0x2a, 0xff}
	colNotice  = color.RGBA{0xf3, 0xbc, 0x6a, 0xff}
	colSimBand = color.RGBA{0x7c, 0x4a, 0xe0, 0xff}
)

// desktops are the background colors, one per student, so thumbnails differ.
var desktops = []color.RGBA{
	{0x2d, 0x4a, 0x6b, 0xff}, {0x4b, 0x2f, 0x6b, 0xff}, {0x2f, 0x5b, 0x45, 0xff},
	{0x6b, 0x4a, 0x2d, 0xff}, {0x2d, 0x5f, 0x66, 0xff}, {0x5b, 0x2f, 0x45, 0xff},
}

func (d drawnScreen) draw() image.Image {
	s := d.s
	snap := s.snapshot()
	img := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	fill(img, img.Bounds(), desktops[(s.index-1)%len(desktops)])

	// The notice every viewed computer shows (docs/privacidad.md rule 6).
	fill(img, image.Rect(0, 0, canvasW, 18), colViewed)
	text(img, 8, 13, "Tu pantalla esta siendo vista por el profesor", colText)

	// The Aula window.
	win := image.Rect(24, 32, canvasW-24, canvasH-24)
	fill(img, win, colWindow)
	fill(img, image.Rect(win.Min.X, win.Min.Y, win.Max.X, win.Min.Y+18), colTitle)
	text(img, win.Min.X+8, win.Min.Y+13, "Aula - "+s.name+" (alumno simulado)", colText)
	fill(img, image.Rect(win.Min.X, win.Min.Y+18, win.Max.X, win.Min.Y+22), colSimBand)

	y := win.Min.Y + 40
	if len(snap.Snippets) == 0 {
		text(img, win.Min.X+12, y, "Esperando codigo del profesor...", colMuted)
	}
	for _, sn := range s.board.Snippets() {
		head := "Ejercicio " + sn.Exercise
		if sn.Title != "" {
			head += " - " + sn.Title
		}
		text(img, win.Min.X+12, y, head, colText)
		y += 16
		for _, line := range strings.Split(strings.TrimRight(sn.Content, "\n"), "\n") {
			if y > win.Max.Y-36 {
				break
			}
			text(img, win.Min.X+20, y, strings.ReplaceAll(line, "\t", "    "), colCode)
			y += 14
		}
		y += 8
		if y > win.Max.Y-36 {
			break
		}
	}
	if snap.LastNotice != "" {
		box := image.Rect(win.Min.X+8, win.Max.Y-28, win.Max.X-8, win.Max.Y-8)
		fill(img, box, colTitle)
		text(img, box.Min.X+8, box.Min.Y+14, "Aviso: "+snap.LastNotice, colNotice)
	}

	big := image.NewRGBA(image.Rect(0, 0, canvasW*scale, canvasH*scale))
	for py := 0; py < canvasH*scale; py++ {
		for px := 0; px < canvasW*scale; px++ {
			big.SetRGBA(px, py, img.RGBAAt(px/scale, py/scale))
		}
	}
	return big
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// plain drops the accents the 7x13 bitmap font cannot draw.
var plain = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ñ", "N", "¿", "", "¡", "", "«", "\"", "»", "\"", "—", "-", "…", "...")

func text(img *image.RGBA, x, y int, s string, c color.RGBA) {
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	d.DrawString(plain.Replace(s))
}

// Compile-time checks.
var (
	_ platform.ScreenCapture   = drawnScreen{}
	_ platform.ScreenIndicator = viewedFlag{}
)
