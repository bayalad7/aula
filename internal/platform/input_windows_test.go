//go:build windows

package platform

import (
	"testing"
	"unsafe"
)

// The tests never call SendInput: CI must not type or click on the desktop.

func TestRawInputLayout(t *testing.T) {
	// INPUT is 40 bytes on 64-bit Windows and 28 on 32-bit.
	want := uintptr(40)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		want = 28
	}
	if got := unsafe.Sizeof(rawInput{}); got != want {
		t.Fatalf("sizeof(INPUT) = %d, want %d", got, want)
	}
	if unsafe.Sizeof(keybdInput{}) > unsafe.Sizeof(mouseInput{}) {
		t.Fatal("KEYBDINPUT must fit in the union")
	}
}

func TestToRaw(t *testing.T) {
	w := &winInput{}
	r, err := w.toRaw(InputEvent{Kind: InputButton, Button: ButtonRight, Down: true, X: 1, Y: 0})
	if err != nil {
		t.Fatal(err)
	}
	if r.typ != inputMouse || r.mi.dx != 65535 || r.mi.dy != 0 || r.mi.flags != mouseMove|mouseAbsolute|mouseRightDown || r.mi.extra != aulaInputTag {
		t.Fatalf("button: %+v", r)
	}
	r, err = w.toRaw(InputEvent{Kind: InputKey, Code: "ArrowUp"})
	if err != nil {
		t.Fatal(err)
	}
	kb := *(*keybdInput)(unsafe.Pointer(&r.mi))
	if r.typ != inputKeyboard || kb.scan != 0x48 || kb.flags != keyScancode|keyExtended|keyUp || kb.vk != 0 {
		t.Fatalf("key: %+v", kb)
	}
	r, _ = w.toRaw(InputEvent{Kind: InputKey, Code: "Pause", Down: true})
	if kb := *(*keybdInput)(unsafe.Pointer(&r.mi)); kb.vk != 0x13 || kb.flags != 0 {
		t.Fatalf("pause: %+v", kb)
	}
	r, _ = w.toRaw(InputEvent{Kind: InputWheel, DY: -120})
	if r.mi.flags != mouseWheel || int32(r.mi.mouseData) != -120 {
		t.Fatalf("wheel: %+v", r.mi)
	}
	if _, err := w.toRaw(InputEvent{Kind: InputKey, Code: "MetaLeft"}); err == nil {
		t.Fatal("unknown key must fail")
	}
}
