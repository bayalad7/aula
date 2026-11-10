package openapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/protocol"
)

// MaxApps is how many apps the catalog holds.
const MaxApps = 50

// App is one entry of the allowed apps (Ajustes › Apps permitidas).
type App struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Catalog is the file of the allowed apps: apps-permitidas.json.
type Catalog struct {
	Apps []App `json:"apps"`
}

// Defaults are the examples a new console starts with; the administrator
// changes them in Ajustes. They are programs a school lab usually has.
func Defaults() []App {
	return []App{
		{ID: "bloc-de-notas", Name: "Bloc de notas", Path: `%SystemRoot%\System32\notepad.exe`},
		{ID: "calculadora", Name: "Calculadora", Path: `%SystemRoot%\System32\calc.exe`},
		{ID: "paint", Name: "Paint", Path: `%SystemRoot%\System32\mspaint.exe`},
		{ID: "vscode", Name: "Visual Studio Code", Path: `%LOCALAPPDATA%\Programs\Microsoft VS Code\Code.exe`},
		{ID: "thonny", Name: "Thonny", Path: `%LOCALAPPDATA%\Programs\Thonny\thonny.exe`},
		{ID: "scratch", Name: "Scratch 3", Path: `%ProgramFiles(x86)%\Scratch 3\Scratch 3.exe`},
	}
}

// Fields of an EntryError.
const (
	FieldName = "name"
	FieldPath = "path"
	FieldList = "list" // too many apps
)

// EntryError says which entry and field of a catalog is wrong.
type EntryError struct {
	Index int    // 0-based; -1 for FieldList
	Field string // FieldName, FieldPath or FieldList
	Err   error
}

func (e *EntryError) Error() string {
	return fmt.Sprintf("app %d, %s: %v", e.Index+1, e.Field, e.Err)
}

func (e *EntryError) Unwrap() error { return e.Err }

var errDuplicate = errors.New("another app has that name")

var (
	notID   = regexp.MustCompile(`[^a-z0-9]+`)
	validID = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
)

// idFor makes an app ID from its name: lowercase letters and digits joined
// by "-", without accents.
func idFor(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch r {
		case 'á', 'à', 'ä', 'â':
			r = 'a'
		case 'é', 'è', 'ë', 'ê':
			r = 'e'
		case 'í', 'ì', 'ï', 'î':
			r = 'i'
		case 'ó', 'ò', 'ö', 'ô':
			r = 'o'
		case 'ú', 'ù', 'ü', 'û':
			r = 'u'
		case 'ñ':
			r = 'n'
		}
		b.WriteRune(r)
	}
	id := strings.Trim(notID.ReplaceAllString(b.String(), "-"), "-")
	if len(id) > 32 {
		id = strings.TrimRight(id[:32], "-")
	}
	if id == "" {
		id = "app"
	}
	return id
}

// Normalize trims the entries, checks them (EntryError) and gives each one
// a unique ID from its name, keeping the IDs it already had.
func Normalize(apps []App) ([]App, error) {
	if len(apps) > MaxApps {
		return nil, &EntryError{Index: -1, Field: FieldList, Err: fmt.Errorf("at most %d apps", MaxApps)}
	}
	out := make([]App, 0, len(apps))
	names, ids := map[string]bool{}, map[string]bool{}
	for i, a := range apps {
		a.Name, a.Path = strings.TrimSpace(a.Name), strings.TrimSpace(a.Path)
		if n := utf8.RuneCountInString(a.Name); n == 0 || n > protocol.MaxOpenAppName || strings.ContainsFunc(a.Name, unicode.IsControl) {
			return nil, &EntryError{Index: i, Field: FieldName, Err: fmt.Errorf("1-%d characters", protocol.MaxOpenAppName)}
		}
		if names[strings.ToLower(a.Name)] {
			return nil, &EntryError{Index: i, Field: FieldName, Err: errDuplicate}
		}
		names[strings.ToLower(a.Name)] = true
		if err := CheckSyntax(a.Path); err != nil {
			return nil, &EntryError{Index: i, Field: FieldPath, Err: err}
		}
		out = append(out, a)
	}
	for i := range out { // the IDs already given first, so they stay
		if validID.MatchString(out[i].ID) && !ids[out[i].ID] {
			ids[out[i].ID] = true
		} else {
			out[i].ID = ""
		}
	}
	for i := range out {
		if out[i].ID != "" {
			continue
		}
		base := idFor(out[i].Name)
		id := base
		for n := 2; ids[id]; n++ {
			suffix := fmt.Sprintf("-%d", n)
			id = strings.TrimRight(base[:min(len(base), 32-len(suffix))], "-") + suffix
		}
		ids[id] = true
		out[i].ID = id
	}
	return out, nil
}

// Parse reads a catalog file and normalizes it.
func Parse(data []byte) ([]App, error) {
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("apps permitidas: %w", err)
	}
	return Normalize(c.Apps)
}

// Marshal writes a catalog file.
func Marshal(apps []App) ([]byte, error) {
	if apps == nil {
		apps = []App{}
	}
	return json.MarshalIndent(Catalog{Apps: apps}, "", "  ")
}
