package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"golang.org/x/image/draw"
)

// appIcon is the app's icon (the one Wails puts on the window and the
// installer); the notification area icon is made from it.
//
//go:embed build/appicon.png
var appIcon []byte

// trayIconSize is the size of the PNG handed to the system, which scales it
// to its small-icon size.
const trayIconSize = 32

// markColor is the dot of the marked icon: something arrived while the
// window was closed. The accent blue of internal/ui/tokens.css (--c-acento),
// with a white ring so it shows on dark and light taskbars.
var markColor = color.RGBA{0x1d, 0x4e, 0xd8, 0xff}

// trayIcons returns the normal icon and the marked one, as PNG.
func trayIcons() (normal, marked []byte, err error) {
	src, err := png.Decode(bytes.NewReader(appIcon))
	if err != nil {
		return nil, nil, fmt.Errorf("tray icon: %w", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, trayIconSize, trayIconSize))
	draw.CatmullRom.Scale(img, img.Bounds(), src, src.Bounds(), draw.Over, nil)
	if normal, err = encodePNG(img); err != nil {
		return nil, nil, err
	}
	// The dot: bottom right, a third of the icon, with a white ring.
	m := image.NewRGBA(img.Bounds())
	draw.Copy(m, image.Point{}, img, img.Bounds(), draw.Src, nil)
	disc(m, 24, 24, 8, color.RGBA{0xff, 0xff, 0xff, 0xff})
	disc(m, 24, 24, 6, markColor)
	if marked, err = encodePNG(m); err != nil {
		return nil, nil, err
	}
	return normal, marked, nil
}

func disc(m *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if dx, dy := x-cx, y-cy; dx*dx+dy*dy <= r*r && image.Pt(x, y).In(m.Bounds()) {
				m.SetRGBA(x, y, c)
			}
		}
	}
}

func encodePNG(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, fmt.Errorf("tray icon: %w", err)
	}
	return b.Bytes(), nil
}
