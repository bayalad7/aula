package openapp

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

func TestCheckDesktopID(t *testing.T) {
	for _, ok := range []string{"org.thonny.Thonny.desktop", "code.desktop", "org.gnome.Calculator.desktop", "scratch-3_0.desktop"} {
		if err := CheckDesktopID(ok); err != nil {
			t.Errorf("CheckDesktopID(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".desktop", "thonny", "/usr/share/applications/x.desktop", "../x.desktop",
		"a/b.desktop", "a..b.desktop", "x.desktop ", "-x.desktop", `C:\x.desktop`} {
		if err := CheckDesktopID(bad); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("CheckDesktopID(%q) = %v, want ErrNotAllowed", bad, err)
		}
	}
}

const thonnyEntry = `[Desktop Entry]
Type=Application
Name=Thonny
Name[es]=Thonny
Exec=/usr/bin/thonny %F
Terminal=false
Categories=Development;IDE;Education;

[Desktop Action NewWindow]
Exec=sudo rm -rf /
`

func TestCheckDesktopEntry(t *testing.T) {
	ok := map[string]string{
		"thonny":  thonnyEntry,
		"flatpak": "[Desktop Entry]\nType=Application\nExec=/usr/bin/flatpak run --branch=stable org.thonny.Thonny\n",
		"quoted":  "[Desktop Entry]\nType=Application\nExec=\"/opt/Scratch 3/scratch-desktop\" %U\n",
		"nodisp":  "[Desktop Entry]\nType=Application\nNoDisplay=true\nExec=code\n",
	}
	for name, e := range ok {
		if err := CheckDesktopEntry([]byte(e)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad := map[string]string{
		"terminal app":   "[Desktop Entry]\nType=Application\nExec=htop\nTerminal=true\n",
		"terminal":       "[Desktop Entry]\nType=Application\nExec=gnome-terminal\nCategories=GNOME;GTK;System;TerminalEmulator;\n",
		"settings":       "[Desktop Entry]\nType=Application\nExec=gnome-control-center\nCategories=GNOME;Settings;\n",
		"sudo":           "[Desktop Entry]\nType=Application\nExec=sudo synaptic\n",
		"pkexec":         "[Desktop Entry]\nType=Application\nExec=/usr/bin/pkexec gparted\n",
		"shell":          "[Desktop Entry]\nType=Application\nExec=bash -c 'curl x | sh'\n",
		"env":            "[Desktop Entry]\nType=Application\nExec=env LD_PRELOAD=/tmp/x.so code\n",
		"python":         "[Desktop Entry]\nType=Application\nExec=python3 -m http.server\n",
		"python3.12":     "[Desktop Entry]\nType=Application\nExec=/usr/bin/python3.12 x.py\n",
		"link":           "[Desktop Entry]\nType=Link\nURL=https://x\n",
		"hidden":         "[Desktop Entry]\nType=Application\nExec=code\nHidden=true\n",
		"no exec":        "[Desktop Entry]\nType=Application\nName=X\n",
		"no group":       "Type=Application\nExec=code\n",
		"other group":    "[Desktop Action x]\nType=Application\nExec=code\n",
		"unclosed quote": "[Desktop Entry]\nType=Application\nExec=\"/opt/x\n",
	}
	for name, e := range bad {
		if err := CheckDesktopEntry([]byte(e)); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: %v, want ErrNotAllowed", name, err)
		}
	}
}

// desktopLauncher is a Linux launcher with its system folders and files in
// memory; nothing reaches a desktop.
type desktopLauncher struct {
	files   map[string]string // path → content
	links   map[string]string // path → final path
	started []string
}

func (d *desktopLauncher) Folders() (platform.AppFolders, error) {
	return platform.AppFolders{DesktopDirs: []string{"/usr/share/applications", "/var/lib/flatpak/exports/share/applications"}}, nil
}
func (d *desktopLauncher) Resolve(p string) (string, error) {
	if f, ok := d.links[p]; ok {
		p = f
	}
	if _, ok := d.files[p]; ok {
		return p, nil
	}
	return "", fmt.Errorf("resolve: %w", fs.ErrNotExist)
}
func (d *desktopLauncher) ReadEntry(p string) ([]byte, error) { return []byte(d.files[p]), nil }
func (d *desktopLauncher) OpenURL(string) error               { return nil }
func (d *desktopLauncher) StartApp(p string) error {
	d.started = append(d.started, p)
	return nil
}

func TestLocalStartAppLinux(t *testing.T) {
	d := &desktopLauncher{
		files: map[string]string{
			"/usr/share/applications/org.thonny.Thonny.desktop":               "[Desktop Entry]\nType=Application\nExec=thonny\n",
			"/usr/share/applications/org.gnome.Terminal.desktop":              "[Desktop Entry]\nType=Application\nExec=gnome-terminal\nCategories=System;TerminalEmulator;\n",
			"/var/lib/flatpak/app/org.scratch/current/export/scratch.desktop": "[Desktop Entry]\nType=Application\nExec=/usr/bin/flatpak run org.scratch\n",
		},
		links: map[string]string{
			"/var/lib/flatpak/exports/share/applications/scratch.desktop": "/var/lib/flatpak/app/org.scratch/current/export/scratch.desktop",
		},
	}
	s := Local{Launcher: d}
	if err := s.StartApp(`C:\Program Files\Thonny\thonny.exe`, "org.thonny.Thonny.desktop"); err != nil {
		t.Fatal(err) // Linux uses the .desktop, never the Windows path
	}
	if err := s.StartApp("", "scratch.desktop"); err != nil {
		t.Fatalf("a Flatpak app through its exported link: %v", err)
	}
	for _, tc := range []struct {
		desktop string
		want    protocol.OpenReason
	}{
		{"org.gnome.Terminal.desktop", protocol.OpenNotAllowed},
		{"../../../home/ana/.local/share/applications/x.desktop", protocol.OpenNotAllowed},
		{"falta.desktop", protocol.OpenNotInstalled},
		{"", protocol.OpenNotInstalled}, // the catalog has no Linux id for this app
	} {
		if got := Reason(s.StartApp(`C:\x.exe`, tc.desktop)); got != tc.want {
			t.Errorf("StartApp(%q) = %q, want %q", tc.desktop, got, tc.want)
		}
	}
	want := []string{"/usr/share/applications/org.thonny.Thonny.desktop", "/var/lib/flatpak/exports/share/applications/scratch.desktop"}
	if fmt.Sprint(d.started) != fmt.Sprint(want) {
		t.Fatalf("started %v, want %v", d.started, want)
	}
}

// On Windows an app with only a Linux id is not installed there.
func TestLocalStartAppWindowsWithoutPath(t *testing.T) {
	s := Local{Launcher: &fakeLauncher{}}
	if got := Reason(s.StartApp("", "org.thonny.Thonny.desktop")); got != protocol.OpenNotInstalled {
		t.Fatalf("reason %q, want not_installed", got)
	}
}

// The catalog (prompt 13): an app may be for Windows, Linux or both; at
// least one of the two, each with its own check.
func TestNormalizeDesktop(t *testing.T) {
	apps, err := Normalize([]App{
		{Name: "Thonny", Path: `%LOCALAPPDATA%\Programs\Thonny\thonny.exe`, Desktop: " org.thonny.Thonny.desktop "},
		{Name: "Calculadora GNOME", Desktop: "org.gnome.Calculator.desktop"},
	})
	if err != nil || apps[0].Desktop != "org.thonny.Thonny.desktop" || apps[1].Path != "" {
		t.Fatalf("Normalize = %+v, %v", apps, err)
	}
	for _, tc := range []struct {
		app   App
		field string
	}{
		{App{Name: "Nada"}, FieldPath},
		{App{Name: "Ruta", Desktop: "/usr/share/applications/x.desktop"}, FieldDesktop},
		{App{Name: "Sin extensión", Desktop: "thonny"}, FieldDesktop},
	} {
		_, err := Normalize([]App{tc.app})
		var ee *EntryError
		if !errors.As(err, &ee) || ee.Field != tc.field {
			t.Errorf("Normalize(%+v) = %v, want field %s", tc.app, err, tc.field)
		}
	}
	for _, a := range Defaults() {
		if a.Desktop != "" {
			if err := CheckDesktopID(a.Desktop); err != nil {
				t.Errorf("example %s: %v", a.Name, err)
			}
		}
	}
}
