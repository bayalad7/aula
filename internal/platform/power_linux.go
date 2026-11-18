//go:build linux

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

// Power on Linux through systemd (docs/privacidad.md rule 102). The
// countdown and the notice live in internal/power, as on Windows.
//
// Without force, systemd honours the "block" inhibitors programs hold (for
// example an editor saving); with force they are ignored (-i). Logoff ends
// the logind session; without force only the caller's own (the service asks
// its helper, which runs as the student).

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
	}
	_, err := runCommand(ctx, "loginctl", "terminate-session", strconv.FormatUint(uint64(session), 10))
	return err
}
