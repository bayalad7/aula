//go:build !windows

package platform

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

func restrictToOwner(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if info.IsDir() {
		mode = 0o700
	}
	return os.Chmod(path, mode)
}

func attachParentConsole() bool { return true }

// nativeNewlines keeps \n, the line ending on Linux and macOS.
func nativeNewlines(s string) string { return s }

// MachineSecretGroup is the system group whose members (the teachers) may
// read the site key on Linux and macOS. The console package creates it; an
// administrator adds each teacher with: usermod -aG aula <usuario>.
const MachineSecretGroup = "aula"

func restrictMachineSecret(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o640)
	if info.IsDir() {
		mode = 0o750
	}
	// Owner root, group aula: the teachers read it, nobody else. Without the
	// group (installed by hand), only root and its group can.
	if g, err := user.LookupGroup(MachineSecretGroup); err == nil && os.Geteuid() == 0 {
		gid, err := strconv.Atoi(g.Gid)
		if err != nil {
			return fmt.Errorf("group %s: %w", MachineSecretGroup, err)
		}
		if err := os.Chown(path, 0, gid); err != nil {
			return err
		}
	}
	return os.Chmod(path, mode)
}
