package security

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bayalad7/aula/internal/platform"
)

// DefaultDir is where the console keeps its identity and certificate:
// %AppData%\Aula\consola on Windows, ~/.config/Aula/consola on Linux,
// ~/Library/Application Support/Aula/consola on macOS.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(base, "Aula", "consola"), nil
}

// writePrivateFile writes secret data so that no other user can read it at
// any moment: the directory and a temporary file are restricted to the owner
// before the data is written, then the file is renamed into place.
func writePrivateFile(path string, data []byte) error {
	return writeSecretFile(path, data, platform.RestrictToOwner)
}

// writeMachineSecret writes the site key: readable by the machine's users by
// design, writable only by administrators. The policy is applied after the
// file is in place, because it would also stop a non-elevated writer; the
// short window exposes nothing users may not read anyway.
func writeMachineSecret(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if err := platform.RestrictMachineSecret(path); err != nil {
		return err
	}
	return platform.RestrictMachineSecret(dir)
}

// writeSecretFile writes data restricting the directory and a temporary file
// with restrict before any byte is written, then renames it into place.
func writeSecretFile(path string, data []byte, restrict func(string) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := restrict(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if err := restrict(tmpName); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
