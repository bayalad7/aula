package openapp

import (
	"errors"
	"fmt"
	"io/fs"

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
	StartApp(path string) error
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

// StartApp implements Session: it expands the path in this session, applies
// CheckPath, resolves links and checks the final path again before starting
// it with no arguments.
func (l Local) StartApp(p string) error {
	f, err := l.Launcher.Folders()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLaunch, err)
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
