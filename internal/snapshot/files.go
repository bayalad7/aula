package snapshot

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FolderName is the folder that groups every screenshot, under the
// console's Aula folder in Documents (docs/privacidad.md rule 46).
const FolderName = "capturas"

// maxNameRunes bounds a student's folder name.
const maxNameRunes = 80

// Root is the folder that groups every screenshot: <documents>\Aula\capturas.
func Root(documents string) string { return filepath.Join(documents, "Aula", FolderName) }

// Dir is where a screenshot goes: root\{alumno}\{AAAA-MM-DD}, with the
// date of the capture in local time.
func Dir(root string, s Snapshot) string {
	return filepath.Join(root, StudentFolder(s.Hostname, s.Username, s.Simulated), s.TakenAt.Local().Format("2006-01-02"))
}

// StudentFolder names a student's folder: "LAB3-PC07 (alumno07)", only the
// computer when no one is logged in, and "… (simulado)" for a simulated
// student, so a test never mixes with real evidence. Characters Windows does
// not allow in a name become "_".
func StudentFolder(hostname, username string, simulated bool) string {
	name := clean(hostname)
	if name == "" {
		name = "equipo"
	}
	if u := clean(username); u != "" {
		name += " (" + u + ")"
	}
	if simulated {
		name += " (simulado)"
	}
	return safeName(name)
}

// WriteFile writes data as Dir(root, s)\{HH-MM-SS}.jpg, adding "-2", "-3"…
// when that name exists (two screenshots in the same second), and returns
// the path. It never overwrites a file. The folders are created as needed
// and readable only by the teacher's account where the OS allows it.
func WriteFile(root string, s Snapshot, data []byte) (string, error) {
	dir := Dir(root, s)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("save screenshot: %w", err)
	}
	base := s.TakenAt.Local().Format("15-04-05")
	for n := 1; n <= 1000; n++ {
		name := base + ".jpg"
		if n > 1 {
			name = base + "-" + strconv.Itoa(n) + ".jpg"
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("save screenshot: %w", err)
		}
		_, werr := f.Write(data)
		cerr := f.Close()
		if err := errors.Join(werr, cerr); err != nil {
			_ = os.Remove(path) // never leave half an image
			return "", fmt.Errorf("save screenshot: %w", err)
		}
		return path, nil
	}
	return "", fmt.Errorf("save screenshot: too many files named %s in %s", base, dir)
}

// clean drops control characters and the ones Windows forbids in names.
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r < 0x20 || r == 0x7f || unicode.IsControl(r):
			return -1
		case strings.ContainsRune(`<>:"/\|?*`, r):
			return '_'
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// safeName bounds the length and avoids what Windows cannot create: a name
// ending in a dot or a space, "..", and the reserved device names.
func safeName(s string) string {
	if utf8.RuneCountInString(s) > maxNameRunes {
		s = string([]rune(s)[:maxNameRunes])
	}
	s = strings.TrimRight(s, ". ")
	if s == "" {
		return "equipo"
	}
	stem := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	stem = strings.TrimSpace(strings.SplitN(stem, " ", 2)[0])
	switch stem {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return "_" + s
	}
	return s
}
