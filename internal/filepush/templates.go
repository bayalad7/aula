package filepush

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bayalad7/aula/internal/protocol"
)

// TemplatesFolder is the folder of the templates inside the teacher's
// Documents\Aula.
const TemplatesFolder = "plantillas"

// Errors of Templates.
var (
	ErrTemplateExists  = errors.New("filepush: a template with that name exists")
	ErrTemplateMissing = errors.New("filepush: no such template")
	// ErrTemplateNotDeleted: the system refused (a file open in another
	// program, no permission); what could be removed is gone.
	ErrTemplateNotDeleted = errors.New("filepush: the template could not be deleted")
)

// Templates are the reusable exercise templates: one folder per template
// under Dir (<Documentos>\Aula\plantillas on the teacher's computer), so the
// teacher can also edit them in Explorer. Every file in the folder, also in
// subfolders, is part of the template.
type Templates struct {
	Dir string
}

// Template is one template as the console lists it.
type Template struct {
	Name  string         `json:"name"`
	Files []TemplateFile `json:"files"`
	Size  int64          `json:"size"`
	// Problem says why it cannot be sent as it is (too many files, too
	// large, a name the students' computers refuse); empty when it can.
	Problem string `json:"problem"`
}

// TemplateFile is one file of a template.
type TemplateFile struct {
	Rel  string `json:"rel"` // "/"-separated, inside the template
	Size int64  `json:"size"`
}

// Problems of a Template.
const (
	ProblemEmpty    = "empty"
	ProblemTooMany  = "too_many"
	ProblemTooLarge = "too_large"
	ProblemBadName  = "bad_name"
)

// List returns the templates, by name. A missing folder is no templates.
func (t Templates) List() ([]Template, error) {
	entries, err := os.ReadDir(t.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Template{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}
	out := []Template{}
	for _, e := range entries {
		// Hidden folders (".nueva-…", a Save in progress) are not templates.
		if !e.IsDir() || e.Type()&fs.ModeSymlink != 0 || strings.HasPrefix(e.Name(), ".") || CheckName(e.Name()) != nil {
			continue
		}
		tpl, err := t.read(e.Name())
		if err != nil {
			continue
		}
		out = append(out, tpl)
	}
	slices.SortFunc(out, func(a, b Template) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out, nil
}

// read lists one template's files. Links are skipped, never followed.
func (t Templates) read(name string) (Template, error) {
	root := filepath.Join(t.Dir, name)
	tpl := Template{Name: name, Files: []TemplateFile{}}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		tpl.Files = append(tpl.Files, TemplateFile{Rel: rel, Size: info.Size()})
		tpl.Size += info.Size()
		if tpl.Problem == "" && !validRel(rel) {
			tpl.Problem = ProblemBadName
		}
		return nil
	})
	if err != nil {
		return Template{}, fmt.Errorf("template %s: %w", name, err)
	}
	switch {
	case len(tpl.Files) == 0:
		tpl.Problem = ProblemEmpty
	case len(tpl.Files) > protocol.MaxFilePushFiles:
		tpl.Problem = ProblemTooMany
	case tpl.Size > protocol.MaxFilePushTotal:
		tpl.Problem = ProblemTooLarge
	}
	for _, f := range tpl.Files {
		if tpl.Problem == "" && f.Size > protocol.MaxFilePushBytes {
			tpl.Problem = ProblemTooLarge
		}
	}
	return tpl, nil
}

// validRel reports whether a file of a template can travel as it is: its
// name and folders follow the path rules, with one level left for the
// destination the teacher types.
func validRel(rel string) bool {
	segs := strings.Split(rel, "/")
	if len(segs) > protocol.MaxFilePushDepth {
		return false
	}
	if CheckName(segs[len(segs)-1]) != nil {
		return false
	}
	return len(segs) == 1 || CheckDest(strings.Join(segs[:len(segs)-1], "/")) == nil
}

// Sources returns the files of template name, ready for NewPush.
func (t Templates) Sources(name string) ([]Source, error) {
	if err := CheckName(name); err != nil {
		return nil, err
	}
	tpl, err := t.read(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrTemplateMissing, name)
	}
	if err != nil {
		return nil, err
	}
	out := make([]Source, len(tpl.Files))
	for i, f := range tpl.Files {
		out[i] = Source{Path: filepath.Join(t.Dir, name, filepath.FromSlash(f.Rel)), Rel: f.Rel}
	}
	return out, nil
}

// Path is the folder of template name, for «Abrir carpeta».
func (t Templates) Path(name string) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}
	p := filepath.Join(t.Dir, name)
	if info, err := os.Stat(p); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrTemplateMissing, name)
	}
	return p, nil
}

// Delete removes template name: its folder, directly under Dir, with what is
// inside. Nothing outside Dir is touched: the name must pass CheckName, and a
// link or a junction in its place is not a template (it is never followed).
// What was already sent to the students does not change.
func (t Templates) Delete(name string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	if t.Dir == "" {
		return fmt.Errorf("%w: %s", ErrTemplateMissing, name)
	}
	p := filepath.Join(t.Dir, name)
	if filepath.Dir(p) != filepath.Clean(t.Dir) {
		return fmt.Errorf("%w: %s", ErrInvalidPath, name)
	}
	info, err := os.Lstat(p)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrTemplateMissing, name)
	}
	if err := os.RemoveAll(p); err != nil {
		return fmt.Errorf("%w: %v", ErrTemplateNotDeleted, err)
	}
	return nil
}

// Save makes template name from the files of a push. It never replaces a
// template: the files are copied to a temporary folder that takes its name
// at the end.
func (t Templates) Save(name string, sources []Source) error {
	name = strings.TrimSpace(name)
	if err := CheckName(name); err != nil {
		return err
	}
	if len(sources) == 0 {
		return ErrNoFiles
	}
	final := filepath.Join(t.Dir, name)
	if _, err := os.Lstat(final); err == nil {
		return fmt.Errorf("%w: %s", ErrTemplateExists, name)
	}
	if err := os.MkdirAll(t.Dir, 0o755); err != nil {
		return fmt.Errorf("templates: %w", err)
	}
	tmp, err := os.MkdirTemp(t.Dir, ".nueva-")
	if err != nil {
		return fmt.Errorf("templates: %w", err)
	}
	defer os.RemoveAll(tmp)
	for _, s := range sources {
		rel := strings.Trim(strings.ReplaceAll(s.Rel, `\`, "/"), "/")
		if !validRel(rel) {
			return &FileError{File: rel, Err: ErrInvalidPath}
		}
		data, err := os.ReadFile(s.Path)
		if err != nil {
			return &FileError{File: rel, Err: fmt.Errorf("%w: %v", ErrUnreadable, err)}
		}
		dst := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("templates: %w", err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("templates: %w", err)
		}
	}
	if err := os.Rename(tmp, final); err != nil {
		if _, statErr := os.Lstat(final); statErr == nil {
			return fmt.Errorf("%w: %s", ErrTemplateExists, name)
		}
		return fmt.Errorf("templates: %w", err)
	}
	return nil
}
