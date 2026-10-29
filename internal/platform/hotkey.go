package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Modifier keys of a Hotkey.
type Modifier uint

// Modifiers; combine them with |.
const (
	ModCtrl Modifier = 1 << iota
	ModAlt
	ModShift
	ModWin
)

// Hotkey is a system-wide key combination: modifiers plus a letter or digit.
type Hotkey struct {
	Mods Modifier
	Key  rune // 'A'-'Z' or '0'-'9'
}

// DefaultCaptureHotkey captures the clipboard into the teacher's console.
// Ctrl+Alt alone is avoided: on Spanish and other layouts it is AltGr, which
// types characters such as @, # or [.
var DefaultCaptureHotkey = Hotkey{Mods: ModCtrl | ModAlt | ModShift, Key: 'C'}

// ErrHotkeyInUse means another program already registered the combination.
var ErrHotkeyInUse = errors.New("hotkey already in use by another program")

// String renders the combination as shown to the user, e.g. "Ctrl+Alt+Shift+C".
func (h Hotkey) String() string {
	var parts []string
	for _, m := range []struct {
		mod  Modifier
		name string
	}{{ModCtrl, "Ctrl"}, {ModAlt, "Alt"}, {ModShift, "Shift"}, {ModWin, "Win"}} {
		if h.Mods&m.mod != 0 {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, string(h.Key)), "+")
}

func (h Hotkey) validate() error {
	if h.Mods == 0 {
		return fmt.Errorf("hotkey %s: at least one modifier is required", h)
	}
	if !(h.Key >= 'A' && h.Key <= 'Z') && !(h.Key >= '0' && h.Key <= '9') {
		return fmt.Errorf("hotkey %s: key must be A-Z or 0-9", h)
	}
	return nil
}

// Hotkeys registers system-wide hotkeys.
type Hotkeys interface {
	// Register calls fn (on its own goroutine) every time h is pressed, in any
	// application, until ctx is canceled. It fails with ErrHotkeyInUse if
	// another program owns the combination, or ErrNotSupported where global
	// hotkeys are not implemented.
	Register(ctx context.Context, h Hotkey, fn func()) error
}

// SystemHotkeys returns the OS implementation: Windows today; elsewhere
// Register returns ErrNotSupported (the console keeps its on-screen button).
func SystemHotkeys() Hotkeys { return systemHotkeys{} }

type systemHotkeys struct{}

func (systemHotkeys) Register(ctx context.Context, h Hotkey, fn func()) error {
	if err := h.validate(); err != nil {
		return err
	}
	if err := registerHotkey(ctx, h, fn); err != nil {
		return fmt.Errorf("register hotkey %s: %w", h, err)
	}
	return nil
}
