package platform

import "image/color"

// PauseText is what the pause window says (docs/privacidad.md rule 9). The
// agent builds it; the window only lays it out, with the clock at the foot.
type PauseText struct {
	Title   string // fixed: "Tu profesor pausó esta computadora"
	Message string // the teacher's note; may be empty
	Detail  string // who paused it and that the work is still open
	Note    string // that the pause goes away by itself if the class is lost
}

// PauseColors is one theme of the pause window.
type PauseColors struct {
	Background, Text color.RGBA
}

// PauseLook is the pause window in the light and the dark theme of the
// system, taken from the design system (internal/ui) by the caller.
type PauseLook struct {
	Light, Dark PauseColors
}

// PauseOverlay is the student's pause window (docs/protocol.md §5.9; D-013):
// one borderless window per monitor, above every other one, that takes the
// clicks. It never captures the keyboard.
type PauseOverlay interface {
	// Show covers every monitor with text and takes the focus once. If the
	// windows are already up, it only replaces the text, without closing
	// them.
	Show(text PauseText) error
	// Yield with on keeps the windows right below this process's main
	// window instead of above everything, so what the student must see
	// (the broadcast, the power countdown, the remote control notice) stays
	// in front. With on false they go back above everything.
	Yield(on bool) error
	// Hide removes the windows. Idempotent.
	Hide() error
}

// NewPauseOverlay returns the OS pause window: Windows today; elsewhere it
// fails with ErrNotSupported, and the agent does not declare screen.freeze.
func NewPauseOverlay(look PauseLook) (PauseOverlay, error) { return newPauseOverlay(look) }
