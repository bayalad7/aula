package openapp

import (
	"bufio"
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// The fixed rules of open.app on Linux (docs/protocol.md §5.22,
// docs/privacidad.md rule 104): an app is named by its .desktop id, looked up
// only in the system's application folders, and its entry must be a plain
// application that is neither a terminal nor a system tool.

var desktopID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.desktop$`)

// CheckDesktopID: only a file name like "org.thonny.Thonny.desktop", never a
// path. The console checks it too, to warn before saving.
func CheckDesktopID(id string) error {
	if len(id) > 255 || !desktopID.MatchString(id) || strings.Contains(id, "..") {
		return fmt.Errorf("%w: not a .desktop id", ErrNotAllowed)
	}
	return nil
}

// deniedCategories are kinds of apps never opened: terminals, system
// settings, package managers and monitors.
var deniedCategories = map[string]bool{
	"TerminalEmulator": true, "System": true, "Settings": true, "PackageManager": true,
	"Security": true, "Monitor": true, "HardwareSettings": true, "DesktopSettings": true,
}

// deniedPrograms are the programs an Exec line may never start with: what
// gives a shell, privileges or runs arbitrary code.
var deniedPrograms = map[string]bool{
	"sudo": true, "pkexec": true, "su": true, "doas": true, "gksu": true, "gksudo": true,
	"sh": true, "bash": true, "dash": true, "zsh": true, "fish": true, "ksh": true, "csh": true, "tcsh": true,
	"env": true, "nohup": true, "setsid": true, "xdg-open": true, "gio": true,
	"python": true, "python3": true, "perl": true, "ruby": true, "node": true, "lua": true,
	"xterm": true, "uxterm": true, "gnome-terminal": true, "konsole": true, "xfce4-terminal": true,
	"mate-terminal": true, "lxterminal": true, "tilix": true, "terminator": true, "kitty": true,
	"alacritty": true, "kgx": true, "ptyxis": true, "x-terminal-emulator": true, "urxvt": true,
	"gnome-control-center": true, "gnome-system-monitor": true, "synaptic": true, "gparted": true,
	"systemctl": true, "dconf-editor": true, "nm-connection-editor": true,
}

// CheckDesktopEntry applies the rules to the content of a .desktop file:
// the [Desktop Entry] group is an application with an Exec, not hidden, not
// run in a terminal, not of a denied category and not starting a denied
// program.
func CheckDesktopEntry(data []byte) error {
	e, err := parseDesktop(data)
	if err != nil {
		return err
	}
	if e["Type"] != "Application" {
		return fmt.Errorf("%w: not an application", ErrNotAllowed)
	}
	if strings.EqualFold(e["Hidden"], "true") {
		return fmt.Errorf("%w: a hidden entry", ErrNotAllowed)
	}
	if strings.EqualFold(e["Terminal"], "true") {
		return fmt.Errorf("%w: runs in a terminal", ErrNotAllowed)
	}
	for _, c := range strings.Split(e["Categories"], ";") {
		if deniedCategories[strings.TrimSpace(c)] {
			return fmt.Errorf("%w: a %s app", ErrNotAllowed, strings.TrimSpace(c))
		}
	}
	prog := execProgram(e["Exec"])
	if prog == "" {
		return fmt.Errorf("%w: no Exec", ErrNotAllowed)
	}
	if deniedPrograms[prog] || strings.HasPrefix(prog, "python") {
		return fmt.Errorf("%w: %s is a system tool", ErrNotAllowed, prog)
	}
	return nil
}

// parseDesktop reads the keys of the [Desktop Entry] group (without locale
// variants such as Name[es]).
func parseDesktop(data []byte) (map[string]string, error) {
	out := map[string]string{}
	in := false
	seen := false
	s := bufio.NewScanner(bytes.NewReader(data))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			in = line == "[Desktop Entry]"
			seen = seen || in
			continue
		}
		if !in {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.Contains(k, "[") {
			continue
		}
		k = strings.TrimSpace(k)
		if _, dup := out[k]; !dup { // the first one counts
			out[k] = strings.TrimSpace(v)
		}
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("%w: unreadable entry", ErrNotAllowed)
	}
	if !seen {
		return nil, fmt.Errorf("%w: no [Desktop Entry]", ErrNotAllowed)
	}
	return out, nil
}

// execProgram is the base name of the program an Exec line starts, without
// quotes ("/usr/bin/thonny %F" is "thonny").
func execProgram(exec string) string {
	exec = strings.TrimSpace(exec)
	if exec == "" {
		return ""
	}
	var first string
	if exec[0] == '"' {
		end := strings.Index(exec[1:], `"`)
		if end < 0 {
			return ""
		}
		first = exec[1 : 1+end]
	} else {
		first = strings.Fields(exec)[0]
	}
	return path.Base(first)
}
