package platform

import (
	"image"
	"image/color"
	"testing"
)

func TestRGBAToNV12(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	// Left 2×2 block white, right one pure red.
	for y := range 2 {
		img.Set(0, y, color.White)
		img.Set(1, y, color.White)
		img.Set(2, y, color.RGBA{255, 0, 0, 255})
		img.Set(3, y, color.RGBA{255, 0, 0, 255})
	}
	dst := make([]byte, 4*2*3/2)
	if err := RGBAToNV12(img, dst); err != nil {
		t.Fatal(err)
	}
	near := func(got, want byte) bool { return int(got)-int(want) <= 1 && int(want)-int(got) <= 1 }
	// BT.601 limited range: white Y=235 U=V=128; red Y=82 U=90 V=240.
	for i, want := range []byte{235, 235, 82, 82, 235, 235, 82, 82, 128, 128, 90, 240} {
		if !near(dst[i], want) {
			t.Errorf("byte %d = %d, want %d (%v)", i, dst[i], want, dst)
		}
	}
	if err := RGBAToNV12(image.NewRGBA(image.Rect(0, 0, 3, 2)), dst); err == nil {
		t.Error("odd width accepted")
	}
	if err := RGBAToNV12(img, dst[:5]); err == nil {
		t.Error("short buffer accepted")
	}
}

func TestNewH264EncoderValidatesConfig(t *testing.T) {
	for _, cfg := range []VideoEncoderConfig{
		{Width: 1921, Height: 1080, FPS: 15, BitrateKbps: 2500, KeyframeEvery: 75},
		{Width: 1920, Height: 1080, FPS: 0, BitrateKbps: 2500, KeyframeEvery: 75},
		{Width: 8192, Height: 1080, FPS: 15, BitrateKbps: 2500, KeyframeEvery: 75},
	} {
		if _, err := NewH264Encoder(cfg); err == nil {
			t.Errorf("config %+v accepted", cfg)
		}
	}
}
