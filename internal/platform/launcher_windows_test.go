//go:build windows

package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The launcher is tested without opening anything on the desktop: no browser
// and no window. StartApp runs hostname.exe, a console tool with no window.

func TestLauncherFolders(t *testing.T) {
	l, err := NewLauncher()
	if err != nil {
		t.Fatal(err)
	}
	f, err := l.Folders()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"ProgramFiles", "SystemRoot"} {
		if f.Vars[v] == "" || !filepath.IsAbs(f.Vars[v]) {
			t.Errorf("%%%s%% = %q", v, f.Vars[v])
		}
	}
	if len(f.Roots) < 2 {
		t.Errorf("roots %q", f.Roots)
	}
	docs, _ := DocumentsDir()
	found := false
	for _, d := range f.Forbidden {
		found = found || strings.EqualFold(d, filepath.Clean(docs))
	}
	if !found {
		t.Errorf("Documents %q is not among the forbidden folders %q", docs, f.Forbidden)
	}
}

func TestLauncherResolve(t *testing.T) {
	l, _ := NewLauncher()
	f, _ := l.Folders()
	notepad := filepath.Join(f.Vars["SystemRoot"], "System32", "notepad.exe")
	got, err := l.Resolve(notepad)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(got, notepad) {
		t.Errorf("Resolve(%q) = %q", notepad, got)
	}
	if _, err := l.Resolve(filepath.Join(t.TempDir(), "no-existe.exe")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: %v, want fs.ErrNotExist", err)
	}
	if _, err := l.Resolve(t.TempDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a folder: %v, want fs.ErrNotExist", err)
	}
	// A symbolic link resolves to its target (needs the developer mode or
	// administrator to create one; skipped otherwise).
	dir := t.TempDir()
	link := filepath.Join(dir, "enlace.exe")
	if err := os.Symlink(notepad, link); err == nil {
		got, err := l.Resolve(link)
		if err != nil || !strings.EqualFold(got, notepad) {
			t.Errorf("Resolve(link) = %q, %v; want %q", got, err, notepad)
		}
	}
}

func TestLauncherStartApp(t *testing.T) {
	l, _ := NewLauncher()
	f, _ := l.Folders()
	if err := l.StartApp(filepath.Join(f.Vars["SystemRoot"], "System32", "hostname.exe")); err != nil {
		t.Fatal(err)
	}
	if err := l.StartApp(filepath.Join(t.TempDir(), "no-existe.exe")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing app: %v, want fs.ErrNotExist", err)
	}
}
