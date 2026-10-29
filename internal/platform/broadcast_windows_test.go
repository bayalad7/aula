//go:build windows

package platform

import (
	"testing"
	"unsafe"
)

var procGetWindowDisplayAffinity = user32.NewProc("GetWindowDisplayAffinity")

func affinity(t *testing.T, hwnd uintptr) uint32 {
	t.Helper()
	var a uint32
	if r, _, err := procGetWindowDisplayAffinity.Call(hwnd, uintptr(unsafe.Pointer(&a))); r == 0 {
		t.Fatalf("GetWindowDisplayAffinity: %v", err)
	}
	return a
}

// TestExcludeFromCapture marks a window of this process, created on another
// thread (like the Wails window), as excluded from captures and back. It
// reads the affinity instead of capturing the screen.
func TestExcludeFromCapture(t *testing.T) {
	w, err := openWindow("prueba de exclusión")
	if err != nil {
		t.Skipf("no desktop: %v", err)
	}
	defer w.close()
	if err := ExcludeFromCapture(true); err != nil {
		t.Fatalf("ExcludeFromCapture(true): %v", err)
	}
	if a := affinity(t, w.hwnd); a != wdaExcludeFromCapture && a != wdaMonitor {
		t.Fatalf("affinity = %#x after excluding", a)
	}
	if err := ExcludeFromCapture(false); err != nil {
		t.Fatalf("ExcludeFromCapture(false): %v", err)
	}
	if a := affinity(t, w.hwnd); a != wdaNone {
		t.Fatalf("affinity = %#x after including", a)
	}
}
