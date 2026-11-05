package platform

// Tray is the app's icon in the notification area (Windows: "Mostrar iconos
// ocultos"), so the app can keep running with its window closed
// (docs/privacidad.md rule 2) and still be found and opened. Windows today;
// elsewhere NewTray returns ErrNotSupported.
type Tray interface {
	// SetTooltip changes the text shown when the pointer rests on the icon.
	SetTooltip(text string) error
	// SetMarked switches between the normal icon and the marked one
	// (something arrived while the window was closed).
	SetMarked(marked bool) error
	// Close removes the icon. Idempotent.
	Close() error
}

// TrayOptions describe the icon.
type TrayOptions struct {
	Icon, MarkedIcon []byte // PNG images, square (32×32 or larger)
	Tooltip          string
	OpenLabel        string // the menu entry that opens the window
	OnOpen           func() // a click on the icon or on OpenLabel; runs on its own goroutine
}

// NewTray adds the icon to the notification area, or returns
// ErrNotSupported.
func NewTray(o TrayOptions) (Tray, error) { return newTray(o) }
