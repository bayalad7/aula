package filepush

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/bayalad7/aula/internal/protocol"
)

// Errors of NewPush. A *FileError tells which file.
var (
	ErrNoFiles      = errors.New("filepush: no files")
	ErrTooManyFiles = errors.New("filepush: too many files")
	ErrFileTooLarge = errors.New("filepush: file too large")
	ErrPushTooLarge = errors.New("filepush: push too large")
	ErrDuplicate    = errors.New("filepush: two files with the same name")
	ErrUnreadable   = errors.New("filepush: file cannot be read")
)

// FileError is a problem with one file of a push.
type FileError struct {
	File string // its path inside the push, "/"-separated
	Err  error
}

func (e *FileError) Error() string { return e.File + ": " + e.Err.Error() }
func (e *FileError) Unwrap() error { return e.Err }

// Source is a file on the teacher's computer and where it goes inside the
// push: Rel is "/"-separated ("main.py", "datos/ventas.csv").
type Source struct {
	Path string `json:"path"`
	Rel  string `json:"rel"`
}

// Push is one send of files, read and checked, ready to cut in parts.
type Push struct {
	ID        string
	Label     string
	Dest      string // base destination, already clean and checked
	Overwrite bool
	Files     []File
	Sources   []Source // what it was read from, for a resend
}

// File is one file of a push.
type File struct {
	ID     string
	Name   string
	Dest   string // full destination folder in the work folder
	Data   []byte
	SHA256 string
}

// Size is the bytes of every file together.
func (p *Push) Size() int {
	n := 0
	for _, f := range p.Files {
		n += len(f.Data)
	}
	return n
}

// Names lists the files' names, in order.
func (p *Push) Names() []string {
	out := make([]string, len(p.Files))
	for i, f := range p.Files {
		out[i] = f.Name
	}
	return out
}

// NewPush reads sources and checks them against the limits and the path
// rules of docs/protocol.md §5.20: dest is the folder the teacher typed
// (CleanDest is applied), relative to the students' work folder.
func NewPush(label, dest string, sources []Source, overwrite bool) (*Push, error) {
	dest = CleanDest(dest)
	if err := CheckDest(dest); err != nil {
		return nil, err
	}
	switch {
	case len(sources) == 0:
		return nil, ErrNoFiles
	case len(sources) > protocol.MaxFilePushFiles:
		return nil, fmt.Errorf("%w: %d, limit %d", ErrTooManyFiles, len(sources), protocol.MaxFilePushFiles)
	}
	label = strings.TrimSpace(label)
	if r := []rune(label); len(r) > protocol.MaxFilePushLabel {
		label = string(r[:protocol.MaxFilePushLabel-1]) + "…"
	}
	p := &Push{ID: protocol.NewID(), Label: label, Dest: dest, Overwrite: overwrite, Sources: sources}
	seen := map[string]bool{}
	total := 0
	for _, s := range sources {
		rel := strings.Trim(strings.ReplaceAll(s.Rel, `\`, "/"), "/")
		dir, name := path.Split(rel)
		dir = strings.TrimSuffix(dir, "/")
		full := dest
		if dir != "" {
			full = Join(dest, dir)
		}
		if err := CheckName(name); err != nil {
			return nil, &FileError{File: rel, Err: err}
		}
		if err := CheckDest(full); err != nil {
			return nil, &FileError{File: rel, Err: err}
		}
		key := strings.ToLower(Join(full, name))
		if seen[key] {
			return nil, &FileError{File: rel, Err: ErrDuplicate}
		}
		seen[key] = true
		info, err := os.Stat(s.Path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, &FileError{File: rel, Err: ErrUnreadable}
		}
		if info.Size() > protocol.MaxFilePushBytes {
			return nil, &FileError{File: rel, Err: ErrFileTooLarge}
		}
		data, err := os.ReadFile(s.Path)
		if err != nil {
			return nil, &FileError{File: rel, Err: fmt.Errorf("%w: %v", ErrUnreadable, err)}
		}
		if len(data) > protocol.MaxFilePushBytes {
			return nil, &FileError{File: rel, Err: ErrFileTooLarge}
		}
		if total += len(data); total > protocol.MaxFilePushTotal {
			return nil, fmt.Errorf("%w: limit %d bytes", ErrPushTooLarge, protocol.MaxFilePushTotal)
		}
		sum := sha256.Sum256(data)
		p.Files = append(p.Files, File{ID: protocol.NewID(), Name: name, Dest: full, Data: data,
			SHA256: hex.EncodeToString(sum[:])})
	}
	return p, nil
}

// Parts cuts file i in its file.push messages; every session gets the same.
func (p *Push) Parts(i int) []protocol.FilePush {
	f := p.Files[i]
	chunks := protocol.SplitParts(f.Data, protocol.FilePushPartSize)
	if len(chunks) == 0 {
		chunks = [][]byte{{}} // an empty file: one part with no data
	}
	out := make([]protocol.FilePush, len(chunks))
	for j, c := range chunks {
		out[j] = protocol.FilePush{PushID: p.ID, FileID: f.ID, Part: j + 1, Parts: len(chunks), Data: c}
	}
	out[0].Name, out[0].Dest, out[0].Size, out[0].SHA256 = f.Name, f.Dest, len(f.Data), f.SHA256
	out[0].Overwrite, out[0].Label, out[0].Index, out[0].Files = p.Overwrite, p.Label, i+1, len(p.Files)
	return out
}
