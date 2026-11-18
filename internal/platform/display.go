package platform

// Kinds of graphical session that DisplayServer reports.
const (
	DisplayX11     = "x11"
	DisplayWayland = "wayland"
	// DisplayNative is Windows or macOS: one display system, nothing to choose.
	DisplayNative = "native"
)

// DisplayServer says which graphical session the program runs in: on Linux
// DisplayX11, DisplayWayland or "" (none, as a service or over SSH); on
// Windows and macOS DisplayNative. The diagnosis shows it, because on Linux
// the screen thumbnails and the console's hotkey need X11
// (docs/compatibilidad.md).
func DisplayServer() string { return displayServer() }
