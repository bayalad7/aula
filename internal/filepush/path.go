package filepush

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/protocol"
)

// ErrInvalidPath is a name or a destination that breaks the path rules of
// docs/protocol.md §5.20.
var ErrInvalidPath = errors.New("filepush: invalid path")

// reservedNames are Windows device names, invalid as a file name even with an
// extension ("nul.txt").
var reservedNames = map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"CONIN$": true, "CONOUT$": true}

func init() {
	for i := 1; i <= 9; i++ {
		reservedNames["COM"+strconv.Itoa(i)] = true
		reservedNames["LPT"+strconv.Itoa(i)] = true
	}
	// Superscript digits are devices too on Windows.
	for _, d := range []string{"¹", "²", "³"} {
		reservedNames["COM"+d] = true
		reservedNames["LPT"+d] = true
	}
}

// CheckName checks one file name (no folders).
func CheckName(name string) error {
	if err := checkSegment(name); err != nil {
		return fmt.Errorf("%w: name: %v", ErrInvalidPath, err)
	}
	return nil
}

// CheckDest checks a destination folder relative to the work folder:
// "/"-separated, up to protocol.MaxFilePushDepth levels; "" is the work
// folder itself.
func CheckDest(dest string) error {
	if dest == "" {
		return nil
	}
	if utf8.RuneCountInString(dest) > protocol.MaxFilePushDest {
		return fmt.Errorf("%w: dest exceeds %d characters", ErrInvalidPath, protocol.MaxFilePushDest)
	}
	segs := strings.Split(dest, "/")
	if len(segs) > protocol.MaxFilePushDepth {
		return fmt.Errorf("%w: dest has more than %d levels", ErrInvalidPath, protocol.MaxFilePushDepth)
	}
	for _, s := range segs {
		if err := checkSegment(s); err != nil {
			return fmt.Errorf("%w: dest: %v", ErrInvalidPath, err)
		}
	}
	return nil
}

// checkSegment applies the rules every folder or file name follows. The
// separators "/" and "\" and the drive colon are forbidden characters, so a
// checked segment can never climb, be absolute or name a stream.
func checkSegment(s string) error {
	if s == "" {
		return errors.New("empty segment")
	}
	if !utf8.ValidString(s) {
		return errors.New("not valid UTF-8")
	}
	if utf8.RuneCountInString(s) > protocol.MaxFilePushName {
		return fmt.Errorf("longer than %d characters", protocol.MaxFilePushName)
	}
	if s == "." || s == ".." {
		return errors.New("relative segment")
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"/\|?*`, r) {
			return fmt.Errorf("forbidden character %q", r)
		}
		// Control and format characters (bidirectional overrides can
		// disguise an extension: "tarea‮txt.exe").
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return fmt.Errorf("invisible character U+%04X", r)
		}
	}
	if last := s[len(s)-1]; last == '.' || last == ' ' {
		return errors.New("ends in a dot or a space")
	}
	base := s
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	if reservedNames[strings.ToUpper(strings.TrimRight(base, " "))] {
		return errors.New("reserved device name")
	}
	return nil
}

// Join puts a checked name in a checked destination.
func Join(dest, name string) string {
	if dest == "" {
		return name
	}
	return dest + "/" + name
}

// CleanDest turns what the teacher typed into a destination: backslashes
// become "/", and spaces and separators at the ends go. It does not make an
// invalid path valid: the result still has to pass CheckDest.
func CleanDest(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), `\`, "/")
	if strings.HasPrefix(s, "//") {
		return s // a network path (UNC) stays as typed, and invalid
	}
	s = strings.Trim(s, "/ ")
	segs := strings.Split(s, "/")
	for i := range segs {
		segs[i] = strings.TrimSpace(segs[i])
	}
	return strings.Join(segs, "/")
}

// MaxAlternative is the last "(n)" tried when a name is taken.
const MaxAlternative = 99

// Alternative returns the n-th name to try when name is taken: n = 1 is
// name itself, then "datos (2).csv", "datos (3).csv"… A leading-dot name
// (".gitignore") has no extension.
func Alternative(name string, n int) string {
	if n <= 1 {
		return name
	}
	ext := path.Ext(name)
	if ext == name {
		ext = ""
	}
	return strings.TrimSuffix(name, ext) + " (" + strconv.Itoa(n) + ")" + ext
}

// CheckRel checks a path as the agent writes it: a checked destination and a
// checked name joined by "/" (Join). It never cleans the path first, so
// "a/../x.txt" is refused rather than turned into "x.txt".
func CheckRel(rel string) error {
	dest, name := "", rel
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		dest, name = rel[:i], rel[i+1:]
		if dest == "" {
			return fmt.Errorf("%w: absolute path", ErrInvalidPath)
		}
	}
	if err := CheckName(name); err != nil {
		return err
	}
	return CheckDest(dest)
}
