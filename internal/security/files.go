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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := platform.RestrictToOwner(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if err := platform.RestrictToOwner(tmpName); err != nil {
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
