//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Power on Linux through systemd (docs/privacidad.md rule 102). The
// countdown and the notice live in internal/power, as on Windows.
//
// Without force, systemd honours the "block" inhibitors programs hold (for
// example an editor saving); with force they are ignored (-i). Logoff
// without force asks the desktop's session manager from the student's own
// session (the service asks its helper), so programs may ask to save; with
// force it ends the logind session.

func newPower() (Power, error) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil, fmt.Errorf("power: no systemctl: %w", ErrNotSupported)
	}
	return systemdPower{}, nil
}

type systemdPower struct{}

func powerCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func (systemdPower) Shutdown(force bool) error {
	ctx, cancel := powerCtx()
	defer cancel()
	_, err := runCommand(ctx, "systemctl", powerArgs("poweroff", force)...)
	return err
}

func (systemdPower) Reboot(force bool) error {
	ctx, cancel := powerCtx()
	defer cancel()
	_, err := runCommand(ctx, "systemctl", powerArgs("reboot", force)...)
	return err
}

// powerArgs never asks for a password on screen (--no-ask-password): the
// service is root, and a student has nothing to type.
func powerArgs(verb string, force bool) []string {
	args := []string{verb, "--no-ask-password", "--no-wall"}
	if force {
		args = append(args, "-i") // ignore inhibitors
	}
	return args
}

func (systemdPower) Logoff(session uint32, force bool) error {
	ctx, cancel := powerCtx()
	defer cancel()
	if !force {
		own, err := currentSession()
		if err != nil || own != session {
			return fmt.Errorf("logoff: session %d is not this process's: %w", session, ErrNotSupported)
		}
		// Ask the desktop's session manager, which lets programs with unsaved
		// work ask first, as Windows does without force (rule 102).
		if name, args := sessionLogout(os.Getenv("XDG_CURRENT_DESKTOP")); name != "" {
			if _, err := exec.LookPath(name); err == nil {
				if err := runHandOver(name, args...); err != nil {
					return fmt.Errorf("logoff: %w", err)
				}
				return nil
			}
		}
	}
	_, err := runCommand(ctx, "loginctl", "terminate-session", strconv.FormatUint(uint64(session), 10))
	return err
}

// sessionLogout is the command that asks the desktop's session manager to
// log out without its confirmation dialog, while still letting programs
// with unsaved work stop it: GNOME and the desktops that share its session
// manager (Cinnamon, MATE). "" for any other desktop, which falls back to
// loginctl.
func sessionLogout(desktops string) (string, []string) {
	for _, d := range strings.Split(desktops, ":") {
		switch strings.ToUpper(strings.TrimSpace(d)) {
		case "GNOME", "GNOME-CLASSIC", "UNITY", "BUDGIE":
			return "gnome-session-quit", []string{"--logout", "--no-prompt"}
		case "X-CINNAMON", "CINNAMON":
			return "cinnamon-session-quit", []string{"--logout", "--no-prompt"}
		case "MATE":
			return "mate-session-save", []string{"--logout"}
		}
	}
	return "", nil
}
