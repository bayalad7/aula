package platform

// Power turns the computer off, reboots it or ends a session at once
// (docs/protocol.md §5.15, D-018). The countdown, the notice and the
// postponing live in internal/power; nothing here waits. Windows today;
// elsewhere NewPower returns ErrNotSupported.
type Power interface {
	// Shutdown turns the computer off. With force, applications close even
	// with unsaved changes; without it, Windows lets them ask to save.
	Shutdown(force bool) error
	// Reboot restarts the computer, with force as in Shutdown.
	Reboot(force bool) error
	// Logoff ends the session with the given ID. Without force it must be the
	// caller's own session (the service asks its helper to do it).
	Logoff(session uint32, force bool) error
}

// NewPower returns the OS power control, or ErrNotSupported.
func NewPower() (Power, error) { return newPower() }

// CurrentSession returns the session of this process.
func CurrentSession() (uint32, error) { return currentSession() }
