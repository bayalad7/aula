//go:build !windows

package platform

import "os"

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
