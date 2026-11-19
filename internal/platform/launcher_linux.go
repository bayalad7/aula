//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Opening on Linux (docs/protocol.md §5.22): pages in the default browser
// with xdg-open. Apps are not started yet: their fixed rules (which .desktop
// or binaries qualify) are a design of their own (docs/compatibilidad.md),
// so Folders and StartApp say ErrNotSupported and the agent declares only
// open.url.

func newLauncher() (Launcher, error) {
	if _, err := exec.LookPath("xdg-open"); err != nil {
		return nil, fmt.Errorf("launcher: no xdg-open: %w", ErrNotSupported)
	}
	return xdgLauncher{}, nil
}

type xdgLauncher struct{}

func (xdgLauncher) Folders() (AppFolders, error) {
	return AppFolders{}, fmt.Errorf("app folders: %w", ErrNotSupported)
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

// xdgOpenWait is how long OpenURL waits for xdg-open to hand the page over;
// some browsers keep it waiting while they start, which is not a failure.
var xdgOpenWait = 10 * time.Second

// OpenURL hands an http/https URL (checked by internal/openapp) to the
// default browser. The URL starts with "http", so xdg-open never reads it as
// an option.
func (xdgLauncher) OpenURL(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("open url: not http or https")
	}
	ctx, cancel := context.WithTimeout(context.Background(), xdgOpenWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, "xdg-open", url)
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil // still opening: the browser took it
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 3 {
		return fmt.Errorf("open url: %w", ErrNoDefaultBrowser) // xdg-open: no program for it
	}
	if err != nil {
		return fmt.Errorf("open url: %w", err)
	}
	return nil
}

func (xdgLauncher) StartApp(string) error {
	return fmt.Errorf("start app: %w", ErrNotSupported)
}
