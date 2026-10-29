package platform

import (
	"context"
	"errors"
	"testing"
)

func TestHotkeyString(t *testing.T) {
	if got := DefaultCaptureHotkey.String(); got != "Ctrl+Alt+Shift+C" {
		t.Fatalf("String = %q", got)
	}
	if got := (Hotkey{Mods: ModWin | ModCtrl, Key: '7'}).String(); got != "Ctrl+Win+7" {
		t.Fatalf("String = %q", got)
	}
}

func TestHotkeyValidation(t *testing.T) {
	for _, h := range []Hotkey{{Key: 'C'}, {Mods: ModCtrl, Key: 'c'}, {Mods: ModCtrl, Key: '!'}} {
		if err := SystemHotkeys().Register(context.Background(), h, func() {}); err == nil || errors.Is(err, ErrNotSupported) {
			t.Errorf("Register(%v) = %v, want a validation error", h, err)
		}
	}
}
