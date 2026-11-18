//go:build linux

package platform

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// documentsDir is the user's Documents folder as xdg-user-dir DOCUMENTS
// gives it (~/Documentos on a Spanish desktop), read from user-dirs.dirs
// without running anything; ~/Documents when it is not set.
func documentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("documents folder: %w", err)
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" || !filepath.IsAbs(cfg) {
		cfg = filepath.Join(home, ".config")
	}
	if f, err := os.Open(filepath.Join(cfg, "user-dirs.dirs")); err == nil {
		defer f.Close()
		if dir, ok := xdgUserDir(f, "DOCUMENTS", home); ok {
			return dir, nil
		}
	}
	return filepath.Join(home, "Documents"), nil
}

// xdgUserDir reads XDG_<name>_DIR from a user-dirs.dirs file: lines like
// XDG_DOCUMENTS_DIR="$HOME/Documentos". Like xdg-user-dir, only "$HOME/…"
// and absolute paths count, and a value equal to $HOME means "not set".
func xdgUserDir(r io.Reader, name, home string) (string, bool) {
	key := "XDG_" + name + "_DIR="
	found := ""
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if !strings.HasPrefix(line, key) {
			continue
		}
		v, err := strconv.Unquote(strings.TrimPrefix(line, key))
		if err != nil {
			continue
		}
		found = v // the last one wins, as in the shell
	}
	switch {
	case found == "":
		return "", false
	case found == "$HOME" || found == "$HOME/":
		return "", false
	case strings.HasPrefix(found, "$HOME/"):
		found = filepath.Join(home, strings.TrimPrefix(found, "$HOME/"))
	case !filepath.IsAbs(found):
		return "", false
	}
	return filepath.Clean(found), true
}

func openFolder(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("open folder: %s is not a folder", path)
	}
	// xdg-open hands the folder to the desktop's file manager. It has no
	// "--": an absolute path never starts with "-", so it is never an option.
	if !filepath.IsAbs(path) {
		return fmt.Errorf("open folder: %s is not absolute", path)
	}
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open folder: %w: %w", ErrNotSupported, err)
	}
	go func() { _ = cmd.Wait() }() // reap it; the file manager stays open
	return nil
}
