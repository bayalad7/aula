package openapp

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// ErrNotAllowed: the path breaks the fixed rules (open.ack not_allowed).
var ErrNotAllowed = errors.New("openapp: not allowed")

// Vars are the only variables a path may start with, as %Name%.
var Vars = []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA", "SystemRoot"}

// denied are interpreters and system tools: never started, even installed
// under a root (docs/privacidad.md rule 68).
var denied = map[string]bool{}

func init() {
	for _, n := range []string{
		"cmd", "powershell", "powershell_ise", "pwsh", "wscript", "cscript", "mshta", "rundll32",
		"regsvr32", "msiexec", "reg", "regedit", "schtasks", "at", "wmic", "bitsadmin", "certutil",
		"bash", "wsl", "wslhost", "conhost", "explorer", "runas", "sc", "net", "net1", "taskkill",
		"forfiles", "installutil", "msbuild", "regasm", "regsvcs", "cmstp", "control", "hh",
		"mmc", "diskpart", "format", "bcdedit", "vssadmin", "wevtutil", "netsh", "shutdown",
	} {
		denied[n+".exe"] = true
	}
}

// Expand replaces a leading %Var% of Vars with its value in vars (the
// student's session). Any other %, or a variable with no value, is refused.
func Expand(p string, vars map[string]string) (string, error) {
	if !strings.HasPrefix(p, "%") {
		if strings.Contains(p, "%") {
			return "", fmt.Errorf("%w: only a leading variable", ErrNotAllowed)
		}
		return p, nil
	}
	end := strings.Index(p[1:], "%")
	if end < 0 {
		return "", fmt.Errorf("%w: unclosed variable", ErrNotAllowed)
	}
	name, rest := p[1:1+end], p[2+end:]
	if strings.Contains(rest, "%") {
		return "", fmt.Errorf("%w: only a leading variable", ErrNotAllowed)
	}
	for _, v := range Vars {
		if strings.EqualFold(v, name) {
			val := vars[v]
			if val == "" {
				return "", fmt.Errorf("%w: %%%s%% has no value here", ErrNotAllowed, v)
			}
			if rest != "" && rest[0] != '\\' && rest[0] != '/' {
				return "", fmt.Errorf("%w: a separator must follow %%%s%%", ErrNotAllowed, v)
			}
			return val + rest, nil
		}
	}
	return "", fmt.Errorf("%w: variable %%%s%% is not allowed", ErrNotAllowed, name)
}

// clean normalizes a Windows path with a drive letter: "\" separators, no
// "." or "..", no device or network forms. It reports false otherwise.
func clean(p string) (string, bool) {
	p = strings.ReplaceAll(p, "/", `\`)
	if len(p) < 3 || p[1] != ':' || p[2] != '\\' || !isLetter(p[0]) {
		return "", false // also \\server, \\?\ and \\.\ forms
	}
	parts := strings.Split(p[3:], `\`)
	out := []string{strings.ToUpper(p[:1]) + ":"}
	for _, s := range parts {
		switch {
		case s == "":
			continue
		case s == "." || s == "..":
			return "", false
		case strings.ContainsAny(s, `:*?"<>|`) || strings.TrimRight(s, ". ") != s:
			return "", false // streams, wildcards and names Windows trims
		}
		out = append(out, s)
	}
	return strings.Join(out, `\`), true
}

func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }

// inside reports whether p is dir or inside it, without case.
func inside(p, dir string) bool {
	d, ok := clean(dir)
	if !ok {
		return false
	}
	lp, ld := strings.ToLower(p), strings.ToLower(d)
	return lp == ld || strings.HasPrefix(lp, ld+`\`)
}

// CheckPath applies the fixed rules to an absolute path already expanded:
// a clean .exe inside a root of f, outside every forbidden folder, and not a
// denied tool. It returns the clean path.
func CheckPath(p string, f platform.AppFolders) (string, error) {
	c, ok := clean(p)
	if !ok {
		return "", fmt.Errorf("%w: not a clean absolute path", ErrNotAllowed)
	}
	base := strings.ToLower(c[strings.LastIndex(c, `\`)+1:])
	if !strings.HasSuffix(base, ".exe") || base == ".exe" {
		return "", fmt.Errorf("%w: not an .exe", ErrNotAllowed)
	}
	if denied[base] {
		return "", fmt.Errorf("%w: %s is a system tool", ErrNotAllowed, base)
	}
	in := false
	for _, r := range f.Roots {
		in = in || inside(c, r)
	}
	if !in {
		return "", fmt.Errorf("%w: outside the program folders", ErrNotAllowed)
	}
	for _, d := range f.Forbidden {
		if inside(c, d) {
			return "", fmt.Errorf("%w: inside a user folder", ErrNotAllowed)
		}
	}
	return c, nil
}

// CheckSyntax is the console's check of a catalog path, without the
// student's folders: it must start with one of Vars or a drive, be a clean
// .exe and not a denied tool. The agent still applies CheckPath.
func CheckSyntax(p string) error {
	if p == "" || utf8.RuneCountInString(p) > protocol.MaxOpenAppPath {
		return fmt.Errorf("%w: 1-%d characters", ErrNotAllowed, protocol.MaxOpenAppPath)
	}
	vars := map[string]string{}
	for _, v := range Vars {
		vars[v] = `C:\` + v // any absolute value: only the shape is checked
	}
	e, err := Expand(p, vars)
	if err != nil {
		return err
	}
	root := e[:min(len(e), 3)]
	if strings.HasPrefix(strings.ToLower(p), "%localappdata%") {
		root = vars["LOCALAPPDATA"] + `\Programs` // the only root under it
	}
	_, err = CheckPath(e, platform.AppFolders{Roots: []string{root}})
	return err
}

// Shown is how the student's notice names a URL: host and path, without the
// query or fragment, at most 80 characters (docs/privacidad.md rule 66).
func Shown(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	s := strings.ToLower(u.Hostname())
	if p := strings.TrimSuffix(path.Clean("/"+u.EscapedPath()), "/"); p != "" && p != "." {
		if up, err := url.PathUnescape(p); err == nil {
			p = up
		}
		s += p
	}
	if utf8.RuneCountInString(s) > 80 {
		s = string([]rune(s)[:79]) + "…"
	}
	return s
}
