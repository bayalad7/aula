//go:build linux && cgo

package platform

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"
)

func TestNV12ToI420(t *testing.T) {
	// 2x2: four Y, then U0 V0 interleaved.
	nv12 := []byte{1, 2, 3, 4, 10, 20}
	i420 := make([]byte, 6)
	nv12ToI420(nv12, i420, 2, 2)
	if !bytes.Equal(i420, []byte{1, 2, 3, 4, 10, 20}) {
		t.Fatalf("i420 = %v", i420)
	}
	nv12 = make([]byte, 4*4*3/2)
	for i := range 4 {
		nv12[16+2*i], nv12[16+2*i+1] = byte(100+i), byte(200+i)
	}
	i420 = make([]byte, len(nv12))
	nv12ToI420(nv12, i420, 4, 4)
	if !bytes.Equal(i420[16:20], []byte{100, 101, 102, 103}) || !bytes.Equal(i420[20:24], []byte{200, 201, 202, 203}) {
		t.Fatalf("planes = %v", i420[16:])
	}
}

// nalTypes lists the NAL unit types of an Annex-B stream.
func nalTypes(b []byte) []int {
	var out []int
	for i := 0; i+3 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			out = append(out, int(b[i+3]&0x1f))
			i += 3
		}
	}
	return out
}

func has(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// openh264 for real (the CI and the container install libopenh264): the
// first frame is an IDR with SPS and PPS, Constrained Baseline; a forced
// keyframe brings them again.
func TestOpenH264Encodes(t *testing.T) {
	enc, err := NewH264Encoder(VideoEncoderConfig{Width: 320, Height: 240, FPS: 10, BitrateKbps: 800, KeyframeEvery: 100})
	if errors.Is(err, ErrNotSupported) {
		t.Skip("no libopenh264.so.7 here")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	frame := func(shade uint8) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 320, 240))
		for y := range 240 {
			for x := range 320 {
				img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), shade, 0xff})
			}
		}
		return img
	}
	first, err := enc.Encode(frame(0), false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(first, []byte{0, 0, 0, 1}) {
		t.Fatalf("not Annex-B: % x", first[:min(8, len(first))])
	}
	types := nalTypes(first)
	if !has(types, 7) || !has(types, 8) || !has(types, 5) {
		t.Fatalf("first frame NALs %v, want SPS (7), PPS (8) and IDR (5)", types)
	}
	sps := bytes.Index(first, []byte{0, 0, 1, 0x67})
	if sps < 0 || first[sps+4] != 66 {
		t.Fatalf("profile_idc = %d, want 66 (Baseline)", first[sps+4])
	}
	if first[sps+5]&0x40 == 0 {
		t.Errorf("constraint_set1_flag not set: not Constrained Baseline")
	}
	for i := range 5 {
		out, err := enc.Encode(frame(uint8(10*i)), false)
		if err != nil {
			t.Fatal(err)
		}
		if has(nalTypes(out), 5) {
			t.Errorf("frame %d is an IDR without being asked", i+2)
		}
	}
	key, err := enc.Encode(frame(99), true)
	if err != nil {
		t.Fatal(err)
	}
	if kt := nalTypes(key); !has(kt, 5) || !has(kt, 7) {
		t.Fatalf("forced keyframe NALs %v, want IDR with SPS", kt)
	}
	if _, err := enc.Encode(image.NewRGBA(image.Rect(0, 0, 16, 16)), false); err == nil {
		t.Error("a frame of the wrong size was encoded")
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Encode(frame(1), false); err == nil {
		t.Error("encoded after Close")
	}
}
