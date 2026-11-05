//go:build !windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

func documentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("documents folder: %w", err)
	}
	return filepath.Join(home, "Documents"), nil
}

func openFolder(string) error { return ErrNotSupported }
