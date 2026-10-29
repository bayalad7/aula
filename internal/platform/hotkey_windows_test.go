//go:build windows

package platform

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSendInput = user32.NewProc("SendInput")

type keyInput struct {
	typ   uint32
	_     uint32 // padding to the union on 64-bit
	vk    uint16
	scan  uint16
	flags uint32
	time  uint32
	extra uintptr
	_     [8]byte // KEYBDINPUT is smaller than MOUSEINPUT in the union
}

func press(t *testing.T, vks ...uint16) {
	t.Helper()
	var in []keyInput
	for _, vk := range vks {
		in = append(in, keyInput{typ: 1, vk: vk})
	}
	for i := len(vks) - 1; i >= 0; i-- {
		in = append(in, keyInput{typ: 1, vk: vks[i], flags: 2}) // KEYEVENTF_KEYUP
	}
	if r, _, err := procSendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0])); int(r) != len(in) {
		t.Skipf("SendInput not available here: %v", err)
	}
}

// Registers a real system hotkey: a duplicate is refused and the combination
// is released on cancel. It sends no input, so it always runs.
func TestHotkeyRegisterConflictAndRelease(t *testing.T) {
	h := Hotkey{Mods: ModCtrl | ModAlt | ModShift, Key: '9'}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := SystemHotkeys().Register(ctx, h, func() {}); err != nil {
		t.Fatal(err)
	}
	if err := SystemHotkeys().Register(ctx, h, func() {}); !errors.Is(err, ErrHotkeyInUse) {
		t.Fatalf("second Register = %v, want ErrHotkeyInUse", err)
	}
	cancel()
	time.Sleep(200 * time.Millisecond)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if err := SystemHotkeys().Register(ctx2, h, func() {}); err != nil {
		t.Fatalf("hotkey not released after cancel: %v", err)
	}
}

// Presses the combination for real with SendInput and waits for the callback.
// It types on the developer's desktop (the modifier presses reach the window
// in front), so it only runs on request: AULA_HOTKEY_TEST=1.
func TestHotkeyFiresOnRealKeyPress(t *testing.T) {
	if os.Getenv("AULA_HOTKEY_TEST") != "1" {
		t.Skip("simulates key presses on the desktop; set AULA_HOTKEY_TEST=1 to run it")
	}
	h := Hotkey{Mods: ModCtrl | ModAlt | ModShift, Key: '9'}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fired := make(chan struct{}, 4)
	if err := SystemHotkeys().Register(ctx, h, func() { fired <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	press(t, windows.VK_CONTROL, windows.VK_MENU, windows.VK_SHIFT, '9')
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("hotkey callback not called")
	}
}
