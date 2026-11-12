package platform

// AppWindow is this process's main window, for what Wails does not offer
// (prompt 19): telling the user something arrived without taking the keyboard
// focus, and a compact size in a corner of the screen. Windows today;
// elsewhere NewAppWindow returns ErrNotSupported and the app falls back to
// Wails' own window calls.
type AppWindow interface {
	// Flash makes the window's taskbar button flash until the user brings
	// the window forward. It never activates the window nor changes its
	// place among the others.
	Flash() error
	// Compact with on remembers the window's placement and shrinks it to
	// width×height (in device-independent pixels) at the bottom right of
	// the work area of its monitor, without activating it nor keeping it on
	// top. With on false it puts the remembered placement back.
	Compact(on bool, width, height int) error
	// Hold keeps the window up while the teacher broadcasts to it
	// (docs/privacidad.md rule 12). HoldWindow keeps it above the other
	// windows, movable and resizable, without minimize nor close;
	// HoldFullscreen makes it cover its monitor, borderless, above
	// everything, without minimize nor close. A minimized window is restored
	// right away. The first Hold remembers the placement, styles and z-order;
	// a later Hold changes the mode from there.
	Hold(mode HoldMode) error
	// Release puts back what the first Hold remembered, exactly. Idempotent.
	Release() error
}

// HoldMode is how AppWindow.Hold keeps the window up.
type HoldMode int

// Hold modes.
const (
	HoldWindow HoldMode = iota + 1
	HoldFullscreen
)

// NewAppWindow returns the main window of this process, or ErrNotSupported.
// Call it once the window exists.
func NewAppWindow() (AppWindow, error) { return newAppWindow() }
