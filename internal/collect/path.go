package collect

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/protocol"
)

// BaseFolder groups every collection, inside the student's work folder and
// inside the teacher's Aula folder (docs/privacidad.md rules 60 and 64).
const BaseFolder = "recolecciones"

// ErrInvalid is a date, an activity, a pattern or a collected path that
// breaks the rules of docs/protocol.md §5.21.
var ErrInvalid = errors.New("collect: invalid")

// CheckDate checks a date as AAAA-MM-DD.
func CheckDate(date string) error {
	if len(date) != len(protocol.FileCollectDateLayout) {
		return fmt.Errorf("%w: date %q", ErrInvalid, date)
	}
	if _, err := time.Parse(protocol.FileCollectDateLayout, date); err != nil {
		return fmt.Errorf("%w: date %q", ErrInvalid, date)
	}
	return nil
}

// CheckActivity checks the activity's name: a folder name by the path rules
// of file.push, 1–80 characters.
func CheckActivity(activity string) error {
	if n := utf8.RuneCountInString(activity); n < 1 || n > protocol.MaxCollectActivity {
		return fmt.Errorf("%w: activity must be 1-%d characters", ErrInvalid, protocol.MaxCollectActivity)
	}
	if err := filepush.CheckName(activity); err != nil {
		return fmt.Errorf("%w: activity: %v", ErrInvalid, err)
	}
	return nil
}

// Folder is the activity folder relative to the work folder:
// recolecciones/<date>/<activity>.
func Folder(date, activity string) (string, error) {
	if err := CheckDate(date); err != nil {
		return "", err
	}
	if err := CheckActivity(activity); err != nil {
		return "", err
	}
	return BaseFolder + "/" + date + "/" + activity, nil
}

// CheckPath checks a collected path, relative to the activity folder: up to
// protocol.MaxFileCollectDepth levels, each one by the path rules. It never
// cleans the path first.
func CheckPath(rel string) error {
	if strings.Count(rel, "/")+1 > protocol.MaxFileCollectDepth {
		return fmt.Errorf("%w: more than %d levels", ErrInvalid, protocol.MaxFileCollectDepth)
	}
	dest, name := "", rel
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		dest, name = rel[:i], rel[i+1:]
		if dest == "" {
			return fmt.Errorf("%w: absolute path", ErrInvalid)
		}
	}
	if err := filepush.CheckName(name); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if dest == "" {
		return nil
	}
	for _, seg := range strings.Split(dest, "/") {
		if err := filepush.CheckName(seg); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
	}
	return nil
}

// ParsePatterns turns what the teacher typed ("*.py, *.txt" or "*.py *.txt")
// into patterns: trimmed, without repeats, each checked. Empty means every
// file.
func ParsePatterns(s string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		key := strings.ToLower(f)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	if err := CheckPatterns(out); err != nil {
		return nil, err
	}
	return out, nil
}

// CheckPatterns checks a list of name patterns.
func CheckPatterns(patterns []string) error {
	if len(patterns) > protocol.MaxCollectPatterns {
		return fmt.Errorf("%w: at most %d patterns", ErrInvalid, protocol.MaxCollectPatterns)
	}
	for _, p := range patterns {
		if n := utf8.RuneCountInString(p); n < 1 || n > protocol.MaxCollectPattern || strings.ContainsAny(p, `/\`) {
			return fmt.Errorf("%w: pattern %q", ErrInvalid, p)
		}
		if _, err := path.Match(strings.ToLower(p), ""); err != nil {
			return fmt.Errorf("%w: pattern %q", ErrInvalid, p)
		}
	}
	return nil
}

// Match reports whether a file name matches one of the patterns, ignoring
// case; no patterns match every name.
func Match(patterns []string, name string) bool {
	if len(patterns) == 0 {
		return true
	}
	name = strings.ToLower(name)
	for _, p := range patterns {
		if ok, _ := path.Match(strings.ToLower(p), name); ok {
			return true
		}
	}
	return false
}

// CheckWorkPath checks a path the helper is asked to create, list or read
// in service mode: an activity folder (recolecciones/<date>/<activity>) or,
// with file, a collected path inside one. It is the second check of
// docs/privacidad.md rule 60: nothing outside an activity folder.
func CheckWorkPath(rel string, file bool) error {
	segs := strings.SplitN(rel, "/", 4)
	if len(segs) < 3 || segs[0] != BaseFolder {
		return fmt.Errorf("%w: %q is not an activity folder", ErrInvalid, rel)
	}
	if _, err := Folder(segs[1], segs[2]); err != nil {
		return err
	}
	if !file {
		if len(segs) != 3 {
			return fmt.Errorf("%w: a folder inside an activity", ErrInvalid)
		}
		return nil
	}
	if len(segs) != 4 {
		return fmt.Errorf("%w: no file", ErrInvalid)
	}
	return CheckPath(segs[3])
}
