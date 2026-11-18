//go:build linux

package platform

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// Linux graphical sessions. Aula watches the screen, draws its notice and
// grabs the console's hotkey only in an X11 session (docs/privacidad.md
// rules 96-97): in Wayland another app's windows cannot be captured without
// asking each time, and nothing stays reliably on top.

func displayServer() string {
	return sessionKind(os.Getenv("XDG_SESSION_TYPE"), os.Getenv("WAYLAND_DISPLAY"), os.Getenv("DISPLAY"))
}

// sessionKind decides the session from its environment: x11, wayland or ""
// (no graphical session, such as a service or SSH).
func sessionKind(sessionType, waylandDisplay, display string) string {
	switch strings.ToLower(sessionType) {
	case "wayland":
		return DisplayWayland
	case "x11":
		if display != "" {
			return DisplayX11
		}
		return ""
	}
	// Without XDG_SESSION_TYPE (started by hand), the sockets decide.
	if waylandDisplay != "" {
		return DisplayWayland
	}
	if display != "" {
		return DisplayX11
	}
	return ""
}

// needX11 fails with ErrNotSupported, saying why, outside an X11 session.
func needX11() error {
	switch displayServer() {
	case DisplayX11:
		return nil
	case DisplayWayland:
		return fmt.Errorf("wayland session: %w", ErrNotSupported)
	}
	return fmt.Errorf("no graphical session: %w", ErrNotSupported)
}

func init() {
	// xgb prints to stderr (for example "no .Xauthority, trying without"):
	// the agent's log already says what failed through the returned errors.
	xgb.Logger = log.New(io.Discard, "", 0)
}

// x11Connect opens a connection to the session's X server.
func x11Connect() (*xgb.Conn, *xproto.ScreenInfo, error) {
	if err := needX11(); err != nil {
		return nil, nil, err
	}
	c, err := xgb.NewConn()
	if err != nil {
		return nil, nil, fmt.Errorf("X11: %w", err)
	}
	setup := xproto.Setup(c)
	if setup == nil || len(setup.Roots) == 0 {
		c.Close()
		return nil, nil, fmt.Errorf("X11: no screen")
	}
	return c, setup.DefaultScreen(c), nil
}

// x11Scale is the desktop's text scale: Xft.dpi of the root window's
// resources relative to 96, or 1 when it is not set.
func x11Scale(c *xgb.Conn, root xproto.Window) float64 {
	atom, err := xproto.InternAtom(c, true, uint16(len("RESOURCE_MANAGER")), "RESOURCE_MANAGER").Reply()
	if err != nil || atom.Atom == xproto.AtomNone {
		return 1
	}
	prop, err := xproto.GetProperty(c, false, root, atom.Atom, xproto.AtomString, 0, 1<<16).Reply()
	if err != nil {
		return 1
	}
	return xftScale(string(prop.Value))
}

// xftScale reads Xft.dpi from X resources ("Xft.dpi:\t144" gives 1.5),
// limited to 1-3.
func xftScale(resources string) float64 {
	for _, line := range strings.Split(resources, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(k) != "Xft.dpi" {
			continue
		}
		dpi, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil || dpi <= 0 {
			return 1
		}
		return min(max(dpi/96, 1), 3)
	}
	return 1
}
