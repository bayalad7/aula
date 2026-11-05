//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

func documentsDir() (string, error) {
	dir, err := windows.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", fmt.Errorf("documents folder: %w", err)
	}
	return dir, nil
}

func openFolder(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("open folder: %s is not a folder", path)
	}
	// explorer.exe exits with status 1 even when it opens the folder: only
	// a failure to start it is an error.
	if err := exec.Command("explorer.exe", path).Start(); err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	return nil
}
