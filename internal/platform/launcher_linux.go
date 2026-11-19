//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Opening on Linux (docs/protocol.md §5.22, docs/privacidad.md rule 104):
// pages in the default browser with xdg-open; apps by their .desktop id,
// looked up only in the system's application folders and started with
// "gio launch" and no arguments. internal/openapp applies the fixed rules
// (the id, the entry's content); this file only finds, reads and starts.

// desktopDirs are the system folders of .desktop files: root writes them,
// the student cannot. ~/.local/share/applications is never one of them.
var desktopDirs = []string{
	"/usr/share/applications",
	"/usr/local/share/applications",
	"/var/lib/flatpak/exports/share/applications",
	"/var/lib/snapd/desktop/applications",
}

// maxEntry is the largest .desktop file read.
const maxEntry = 64 << 10

func newLauncher() (Launcher, error) {
	if _, err := exec.LookPath("xdg-open"); err != nil {
		return nil, fmt.Errorf("launcher: no xdg-open: %w", ErrNotSupported)
	}
	return xdgLauncher{}, nil
}

type xdgLauncher struct{}

// Folders gives the system's application folders; without gio there is no
// way to start an app from its .desktop, and apps are not offered.
func (xdgLauncher) Folders() (AppFolders, error) {
	if _, err := exec.LookPath("gio"); err != nil {
		return AppFolders{}, fmt.Errorf("app folders: no gio: %w", ErrNotSupported)
	}
	return AppFolders{DesktopDirs: append([]string(nil), desktopDirs...)}, nil
}

func (xdgLauncher) Resolve(path string) (string, error) {
	final, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve: %w", err)
	}
	info, err := os.Stat(final)
	if err != nil {
		return "", fmt.Errorf("resolve: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("resolve: not a file: %w", fs.ErrNotExist)
	}
	return final, nil
}

func (xdgLauncher) ReadEntry(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read entry: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxEntry+1))
	if err != nil {
		return nil, fmt.Errorf("read entry: %w", err)
	}
	if len(data) > maxEntry {
		return nil, fmt.Errorf("read entry: larger than %d bytes", maxEntry)
	}
	return data, nil
}

// xdgOpenWait is how long OpenURL and StartApp wait for the tool to hand
// over; some programs keep it waiting while they start, which is not a
// failure.
var xdgOpenWait = 10 * time.Second

// OpenURL hands an http/https URL (checked by internal/openapp) to the
// default browser. The URL starts with "http", so xdg-open never reads it as
// an option.
func (xdgLauncher) OpenURL(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("open url: not http or https")
	}
	err := runHandOver("xdg-open", url)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 3 {
		return fmt.Errorf("open url: %w", ErrNoDefaultBrowser) // xdg-open: no program for it
	}
	if err != nil {
		return fmt.Errorf("open url: %w", err)
	}
	return nil
}

// StartApp starts the app of an absolute .desktop path (already checked by
// internal/openapp) with "gio launch" and no files or arguments.
func (xdgLauncher) StartApp(path string) error {
	if !filepath.IsAbs(path) || !strings.HasSuffix(path, ".desktop") {
		return fmt.Errorf("start app: not an absolute .desktop")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("start app: %w", err)
	}
	if err := runHandOver("gio", "launch", path); err != nil {
		return fmt.Errorf("start app: %w", err)
	}
	return nil
}

// runHandOver runs a tool that hands something to the desktop and waits at
// most xdgOpenWait: still running then means the program took it.
func runHandOver(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), xdgOpenWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil
	}
	return err
}
