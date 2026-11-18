//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// Global hotkeys on Linux: XGrabKey on the root window of an X11 session.
// In Wayland there is no way to grab a key for every app, so Register fails
// with ErrNotSupported and the console keeps its on-screen button.

// Modifier masks that do not change the combination: Caps Lock (Lock) and
// Num Lock (usually Mod2). The grab is repeated with each of them.
var ignoredMods = []uint16{0, xproto.ModMaskLock, xproto.ModMask2, xproto.ModMaskLock | xproto.ModMask2}

func registerHotkey(ctx context.Context, h Hotkey, fn func()) error {
	c, screen, err := x11Connect()
	if err != nil {
		return err
	}
	code, err := keycodeOf(c, h.Key)
	if err != nil {
		c.Close()
		return err
	}
	mods := x11Mods(h.Mods)
	root := screen.Root
	for _, extra := range ignoredMods {
		if err := xproto.GrabKeyChecked(c, true, root, mods|extra, code,
			xproto.GrabModeAsync, xproto.GrabModeAsync).Check(); err != nil {
			ungrab(c, root, mods, code)
			c.Close()
			var access xproto.AccessError
			if errors.As(err, &access) {
				return ErrHotkeyInUse
			}
			return fmt.Errorf("X11: grab key: %w", err)
		}
	}
	stop := context.AfterFunc(ctx, func() {
		ungrab(c, root, mods, code)
		c.Close() // ends WaitForEvent below
	})
	go func() {
		defer stop()
		for {
			ev, xerr := c.WaitForEvent()
			if ev == nil && xerr == nil {
				return // connection closed
			}
			if kp, ok := ev.(xproto.KeyPressEvent); ok && kp.Detail == code &&
				kp.State&^(xproto.ModMaskLock|xproto.ModMask2) == mods {
				go fn()
			}
		}
	}()
	return nil
}

func ungrab(c *xgb.Conn, root xproto.Window, mods uint16, code xproto.Keycode) {
	for _, extra := range ignoredMods {
		xproto.UngrabKey(c, code, root, mods|extra)
	}
	_, _ = xproto.GetInputFocus(c).Reply() // flushes the ungrabs
}

// x11Mods maps Aula's modifiers to X11's: Alt is Mod1 and the Windows
// (Super) key Mod4 on every common keyboard map.
func x11Mods(m Modifier) uint16 {
	var x uint16
	if m&ModCtrl != 0 {
		x |= xproto.ModMaskControl
	}
	if m&ModAlt != 0 {
		x |= xproto.ModMask1
	}
	if m&ModShift != 0 {
		x |= xproto.ModMaskShift
	}
	if m&ModWin != 0 {
		x |= xproto.ModMask4
	}
	return x
}

// keycodeOf finds the key that types r ('A'-'Z' or '0'-'9') in the current
// keyboard map. Letters are looked up by their lower-case keysym, which is
// the key's first symbol on Latin layouts.
func keycodeOf(c *xgb.Conn, r rune) (xproto.Keycode, error) {
	sym := xproto.Keysym(r)
	if r >= 'A' && r <= 'Z' {
		sym = xproto.Keysym(r - 'A' + 'a')
	}
	setup := xproto.Setup(c)
	first, last := setup.MinKeycode, setup.MaxKeycode
	m, err := xproto.GetKeyboardMapping(c, first, byte(last-first+1)).Reply()
	if err != nil {
		return 0, fmt.Errorf("X11: keyboard map: %w", err)
	}
	return findKeycode(m.Keysyms, int(m.KeysymsPerKeycode), first, sym, xproto.Keysym(r))
}

// findKeycode searches a keyboard map (per keycode, perCode keysyms) for the
// first key whose symbols include sym or alt.
func findKeycode(syms []xproto.Keysym, perCode int, first xproto.Keycode, sym, alt xproto.Keysym) (xproto.Keycode, error) {
	if perCode < 1 {
		return 0, fmt.Errorf("X11: empty keyboard map")
	}
	for i := 0; i+perCode <= len(syms); i += perCode {
		for _, s := range syms[i : i+perCode] {
			if s == sym || s == alt {
				return first + xproto.Keycode(i/perCode), nil
			}
		}
	}
	return 0, fmt.Errorf("X11: no key types %q: %w", rune(alt), ErrNotSupported)
}
