package collect

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/snapshot"
)

// ManifestName is the file that describes a student's round in the teacher's
// folder: who, which round, when, and each file with its sha256. Running the
// tests later (prompt 14) reads it.
const ManifestName = "aula-recoleccion.json"

// Root is the teacher's folder of every collection:
// <documents>\Aula\recolecciones.
func Root(documents string) string { return filepath.Join(documents, "Aula", BaseFolder) }

// ActivityDir is root\<date>\<activity>.
func ActivityDir(root, date, activity string) string {
	return filepath.Join(root, date, activity)
}

// StudentDir is the folder of one student in an activity, named like the
// screenshots' ("LAB3-PC07 (alumno07)", "… (simulado)").
func StudentDir(activityDir string, s Student) string {
	return filepath.Join(activityDir, snapshot.StudentFolder(s.Hostname, s.Username, s.Simulated))
}

// Manifest is the content of ManifestName.
type Manifest struct {
	CollectID string         `json:"collect_id"`
	Date      string         `json:"date"`
	Activity  string         `json:"activity"`
	Round     int            `json:"round"`
	Hostname  string         `json:"hostname"`
	Username  string         `json:"username"`
	Simulated bool           `json:"simulated,omitempty"`
	At        time.Time      `json:"at"` // when the round ended
	Files     []ManifestFile `json:"files"`
	Skipped   []SkippedFile  `json:"skipped,omitempty"`
}

// ManifestFile is one saved file of a round.
type ManifestFile struct {
	Path       string    `json:"path"` // relative to the student's folder, "/"-separated
	Size       int       `json:"size"`
	SHA256     string    `json:"sha256"`
	ModifiedAt time.Time `json:"modified_at,omitempty"`
}

// SkippedFile is a file the agent did not send, and why.
type SkippedFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Reason string `json:"reason"`
}

// staging is where one round of one student is written while it arrives;
// commit swaps it in place of the previous round.
type staging struct {
	dir   string // inside the activity folder, hidden from the student list
	final string
}

func newStaging(final string) (*staging, error) {
	parent := filepath.Dir(final)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, fmt.Errorf("collect: %w", err)
	}
	var b [6]byte
	_, _ = rand.Read(b[:])
	dir := filepath.Join(parent, ".aula-"+hex.EncodeToString(b[:])+".tmp")
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, fmt.Errorf("collect: %w", err)
	}
	return &staging{dir: dir, final: final}, nil
}

// write saves one file at rel (checked by CheckPath) inside the staging.
func (s *staging) write(rel string, data []byte) error {
	full := filepath.Join(s.dir, filepath.FromSlash(rel))
	if in, err := filepath.Rel(s.dir, full); err != nil || in == ".." || strings.HasPrefix(in, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: %q leaves the folder", ErrInvalid, rel)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		return fmt.Errorf("collect: %w", err)
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		_ = os.Remove(full)
		return fmt.Errorf("collect: %w", err)
	}
	return nil
}

// commit writes the manifest and replaces the previous round of the student
// with this one. Only the teacher's copy is replaced.
func (s *staging) commit(m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("collect: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, ManifestName), b, 0o600); err != nil {
		return fmt.Errorf("collect: %w", err)
	}
	old := ""
	if _, err := os.Stat(s.final); err == nil {
		old = s.dir + ".old"
		if err := os.Rename(s.final, old); err != nil {
			return fmt.Errorf("collect: the previous round is in use: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("collect: %w", err)
	}
	if err := os.Rename(s.dir, s.final); err != nil {
		if old != "" {
			_ = os.Rename(old, s.final)
		}
		return fmt.Errorf("collect: %w", err)
	}
	if old != "" {
		_ = os.RemoveAll(old)
	}
	return nil
}

// discard removes what arrived of an abandoned round; the previous one stays.
func (s *staging) discard() { _ = os.RemoveAll(s.dir) }

// ReadManifest reads the manifest of a student's folder.
func ReadManifest(studentDir string) (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(studentDir, ManifestName))
	if err != nil {
		return m, fmt.Errorf("collect: %w", err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("collect: manifest: %w", err)
	}
	return m, nil
}

// CopyTo copies a student's folder into parent («Guardar como…»), under the
// same name or «name (2)»… when it is taken; it never overwrites. It returns
// the new folder.
func CopyTo(src, parent string) (string, error) {
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("collect: nothing collected yet: %w", fs.ErrNotExist)
	}
	base := filepath.Base(src)
	var dest string
	for n := 1; ; n++ {
		if n > 99 {
			return "", fmt.Errorf("collect: too many copies of %s", base)
		}
		dest = filepath.Join(parent, base)
		if n > 1 {
			dest = filepath.Join(parent, fmt.Sprintf("%s (%d)", base, n))
		}
		if err := os.Mkdir(dest, 0o700); err == nil {
			break
		} else if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("collect: %w", err)
		}
	}
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !d.Type().IsRegular() {
			return nil // never follow a link
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o600)
	})
	if err != nil {
		return "", fmt.Errorf("collect: copy: %w", err)
	}
	return dest, nil
}
