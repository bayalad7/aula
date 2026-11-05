package platform

import "time"

// InputKind is the kind of an injected event.
type InputKind int

// Kinds of InputEvent.
const (
	InputMove   InputKind = iota + 1 // move the pointer to X, Y
	InputButton                      // move to X, Y and press or release Button
	InputWheel                       // scroll DX, DY wheel units (120 = one notch)
	InputKey                         // press or release the physical key Code
)

// MouseButton is a mouse button of InputButton.
type MouseButton int

// Mouse buttons.
const (
	ButtonLeft MouseButton = iota + 1
	ButtonRight
	ButtonMiddle
)

// InputEvent is one keyboard or mouse event to inject in the student's
// session (docs/protocol.md §5.16). X and Y are 0-1 of the primary display;
// Code is a KeyboardEvent.code of KnownKey.
type InputEvent struct {
	Kind   InputKind
	X, Y   float64
	Button MouseButton
	Down   bool
	DX, DY int
	Code   string
}

// InputInjector types and clicks for the teacher during a remote control
// (docs/privacidad.md rules 36–43). It must run in the student's session: as
// a service, only the helper holds the real one (internal/agentsvc). It never
// records what the student types: LocalInput only says *when* there was input
// that Aula did not inject.
type InputInjector interface {
	// Inject sends the events to the OS in order.
	Inject(events []InputEvent) error
	// LocalInput returns the time of the last keyboard or mouse input that
	// did not come from Inject (the zero time if none was seen since the
	// injector was created).
	LocalInput() (time.Time, error)
}

// NewInputInjector returns the OS input injection: Windows today; elsewhere
// it fails with ErrNotSupported and the agent does not declare remote.
func NewInputInjector() (InputInjector, error) { return newInputInjector() }

// keyCode says how a physical key is injected: by its PC (set 1) scancode,
// with the E0 prefix when ext, or by virtual key when vk is set, for the
// keys whose scancode the OS does not take (Pause, NumLock, PrintScreen).
type keyCode struct {
	scan uint16
	ext  bool
	vk   uint16
}

// keyCodes maps KeyboardEvent.code to its key. Scancodes are those of a PC
// keyboard and do not depend on the layout: the student's layout decides
// the character, as with a physical keyboard.
var keyCodes = map[string]keyCode{
	"Escape": {scan: 0x01}, "Digit1": {scan: 0x02}, "Digit2": {scan: 0x03}, "Digit3": {scan: 0x04},
	"Digit4": {scan: 0x05}, "Digit5": {scan: 0x06}, "Digit6": {scan: 0x07}, "Digit7": {scan: 0x08},
	"Digit8": {scan: 0x09}, "Digit9": {scan: 0x0A}, "Digit0": {scan: 0x0B}, "Minus": {scan: 0x0C},
	"Equal": {scan: 0x0D}, "Backspace": {scan: 0x0E}, "Tab": {scan: 0x0F},
	"KeyQ": {scan: 0x10}, "KeyW": {scan: 0x11}, "KeyE": {scan: 0x12}, "KeyR": {scan: 0x13},
	"KeyT": {scan: 0x14}, "KeyY": {scan: 0x15}, "KeyU": {scan: 0x16}, "KeyI": {scan: 0x17},
	"KeyO": {scan: 0x18}, "KeyP": {scan: 0x19}, "BracketLeft": {scan: 0x1A}, "BracketRight": {scan: 0x1B},
	"Enter": {scan: 0x1C}, "ControlLeft": {scan: 0x1D},
	"KeyA": {scan: 0x1E}, "KeyS": {scan: 0x1F}, "KeyD": {scan: 0x20}, "KeyF": {scan: 0x21},
	"KeyG": {scan: 0x22}, "KeyH": {scan: 0x23}, "KeyJ": {scan: 0x24}, "KeyK": {scan: 0x25},
	"KeyL": {scan: 0x26}, "Semicolon": {scan: 0x27}, "Quote": {scan: 0x28}, "Backquote": {scan: 0x29},
	"ShiftLeft": {scan: 0x2A}, "Backslash": {scan: 0x2B},
	"KeyZ": {scan: 0x2C}, "KeyX": {scan: 0x2D}, "KeyC": {scan: 0x2E}, "KeyV": {scan: 0x2F},
	"KeyB": {scan: 0x30}, "KeyN": {scan: 0x31}, "KeyM": {scan: 0x32}, "Comma": {scan: 0x33},
	"Period": {scan: 0x34}, "Slash": {scan: 0x35}, "ShiftRight": {scan: 0x36},
	"NumpadMultiply": {scan: 0x37}, "AltLeft": {scan: 0x38}, "Space": {scan: 0x39}, "CapsLock": {scan: 0x3A},
	"F1": {scan: 0x3B}, "F2": {scan: 0x3C}, "F3": {scan: 0x3D}, "F4": {scan: 0x3E}, "F5": {scan: 0x3F},
	"F6": {scan: 0x40}, "F7": {scan: 0x41}, "F8": {scan: 0x42}, "F9": {scan: 0x43}, "F10": {scan: 0x44},
	"NumLock": {vk: 0x90}, "ScrollLock": {scan: 0x46},
	"Numpad7": {scan: 0x47}, "Numpad8": {scan: 0x48}, "Numpad9": {scan: 0x49}, "NumpadSubtract": {scan: 0x4A},
	"Numpad4": {scan: 0x4B}, "Numpad5": {scan: 0x4C}, "Numpad6": {scan: 0x4D}, "NumpadAdd": {scan: 0x4E},
	"Numpad1": {scan: 0x4F}, "Numpad2": {scan: 0x50}, "Numpad3": {scan: 0x51}, "Numpad0": {scan: 0x52},
	"NumpadDecimal": {scan: 0x53}, "IntlBackslash": {scan: 0x56}, "F11": {scan: 0x57}, "F12": {scan: 0x58},

	"NumpadEnter": {scan: 0x1C, ext: true}, "ControlRight": {scan: 0x1D, ext: true},
	"NumpadDivide": {scan: 0x35, ext: true}, "AltRight": {scan: 0x38, ext: true},
	"Home": {scan: 0x47, ext: true}, "ArrowUp": {scan: 0x48, ext: true}, "PageUp": {scan: 0x49, ext: true},
	"ArrowLeft": {scan: 0x4B, ext: true}, "ArrowRight": {scan: 0x4D, ext: true}, "End": {scan: 0x4F, ext: true},
	"ArrowDown": {scan: 0x50, ext: true}, "PageDown": {scan: 0x51, ext: true}, "Insert": {scan: 0x52, ext: true},
	"Delete": {scan: 0x53, ext: true}, "ContextMenu": {scan: 0x5D, ext: true},
	"PrintScreen": {vk: 0x2C}, "Pause": {vk: 0x13},
}

// KnownKey reports whether code can be injected.
func KnownKey(code string) bool {
	_, ok := keyCodes[code]
	return ok
}
