package openapp

import (
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Errors a Session reports besides ErrNotAllowed and fs.ErrNotExist.
var (
	// ErrBusy: over the agent's pace (open.ack busy).
	ErrBusy = errors.New("openapp: too many in a row")
	// ErrLaunch: the OS could not open it (open.ack launch_failed).
	ErrLaunch = errors.New("openapp: could not open")
)

// Session opens in the student's session. Errors wrap ErrNotAllowed,
// fs.ErrNotExist or ErrLaunch.
type Session interface {
	OpenURL(url string) error
	// StartApp starts the app of open.app: path (the .exe, Windows) or
	// desktop (the .desktop id, Linux), whichever this system uses.
	StartApp(path, desktop string) error
}

// Local opens with a launcher of this session (window mode, or the helper),
// checking the URL and the path with the fixed rules first.
type Local struct{ Launcher platform.Launcher }

// OpenURL implements Session.
func (l Local) OpenURL(raw string) error {
	if err := protocol.CheckOpenURL(raw); err != nil {
		return fmt.Errorf("%w: %v", ErrNotAllowed, err)
	}
	if err := l.Launcher.OpenURL(raw); err != nil {
		return fmt.Errorf("%w: %v", ErrLaunch, err)
	}
	return nil
}

// StartApp implements Session. On Windows it expands the path in this
// session, applies CheckPath, resolves links and checks the final path again
// before starting it with no arguments; on Linux (the launcher has
// DesktopDirs) it goes by the .desktop id (startDesktop).
func (l Local) StartApp(p, desktop string) error {
	f, err := l.Launcher.Folders()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLaunch, err)
	}
	if len(f.DesktopDirs) > 0 {
		return l.startDesktop(desktop, f.DesktopDirs)
	}
	if p == "" {
		return fmt.Errorf("start app: no Windows path: %w", fs.ErrNotExist) // not_installed
	}
	e, err := Expand(p, f.Vars)
	if err != nil {
		return err
	}
	c, err := CheckPath(e, f)
	if err != nil {
		return err
	}
	final, err := l.Launcher.Resolve(c)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("start app: %w", fs.ErrNotExist)
		}
		return fmt.Errorf("%w: %v", ErrLaunch, err)
	}
	if final, err = CheckPath(final, f); err != nil {
		return err // a link out of the program folders
	}
	if err := l.Launcher.StartApp(final); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("start app: %w", fs.ErrNotExist)
		}
		return fmt.Errorf("%w: %v", ErrLaunch, err)
	}
	return nil
}

// startDesktop finds the id in the system's application folders (the
// first that has it), checks its entry and starts it. The file is read and
// checked where it was found; a link may point elsewhere (Flatpak exports
// do), but only a root-owned system folder can hold the link.
func (l Local) startDesktop(id string, dirs []string) error {
	if id == "" {
		return fmt.Errorf("start app: no Linux .desktop: %w", fs.ErrNotExist) // not_installed
	}
	if err := CheckDesktopID(id); err != nil {
		return err
	}
	for _, dir := range dirs {
		p := path.Join(dir, id)
		final, err := l.Launcher.Resolve(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrLaunch, err)
		}
		data, err := l.Launcher.ReadEntry(final)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrLaunch, err)
		}
		if err := CheckDesktopEntry(data); err != nil {
			return err
		}
		if err := l.Launcher.StartApp(p); err != nil {
			return fmt.Errorf("%w: %v", ErrLaunch, err)
		}
		return nil
	}
	return fmt.Errorf("start app: %w", fs.ErrNotExist)
}

// Reason is the open.ack reason of err ("" when nil).
func Reason(err error) protocol.OpenReason {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrNotAllowed):
		return protocol.OpenNotAllowed
	case errors.Is(err, fs.ErrNotExist):
		return protocol.OpenNotInstalled
	case errors.Is(err, ErrBusy):
		return protocol.OpenBusy
	default:
		return protocol.OpenLaunchFailed
	}
}

// ReasonErr is the error of an open.ack reason, for the helper's replies.
func ReasonErr(r protocol.OpenReason) error {
	switch r {
	case "":
		return nil
	case protocol.OpenNotAllowed:
		return ErrNotAllowed
	case protocol.OpenNotInstalled:
		return fs.ErrNotExist
	case protocol.OpenBusy:
		return ErrBusy
	default:
		return ErrLaunch
	}
}
