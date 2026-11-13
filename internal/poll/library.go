package poll

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// QuizzesFolder is the folder of the saved quizzes inside the teacher's
// Documents\Aula (docs/privacidad.md rule 89).
const QuizzesFolder = "cuestionarios"

// GradesFolder is where «Exportar calificaciones» writes, inside the
// teacher's Documents\Aula, one folder per date.
const GradesFolder = "calificaciones"

// MaxSavedQuizSize is the largest quiz file the console reads.
const MaxSavedQuizSize = 256 << 10

// Errors of the Library.
var (
	ErrQuizMissing = errors.New("poll: no such saved quiz")
	ErrQuizFile    = errors.New("poll: the quiz file cannot be read")
)

// savedVersion is the version of the quiz file.
const savedVersion = 1

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,59}$`)

// Library is the teacher's saved quizzes: one JSON file per quiz under Dir
// (<Documentos>\Aula\cuestionarios), their material to reuse. It never holds
// answers nor scores.
type Library struct {
	Dir string
}

// Saved is one saved quiz as the console lists it. A file that cannot be
// read is listed with Problem and the file name as its title, so the teacher
// sees it instead of losing it silently.
type Saved struct {
	ID       string    `json:"id"`
	Quiz     Quiz      `json:"quiz"`
	Modified time.Time `json:"modified"`
	Problem  string    `json:"problem"` // "" or "unreadable"
}

type savedFile struct {
	Version int `json:"version"`
	Quiz
}

// List returns the saved quizzes, by title. A missing folder is no quizzes.
func (l Library) List() ([]Saved, error) {
	entries, err := os.ReadDir(l.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Saved{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("quizzes: %w", err)
	}
	out := []Saved{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !e.Type().IsRegular() || !idPattern.MatchString(id) {
			continue
		}
		s := Saved{ID: id}
		if info, err := e.Info(); err == nil {
			s.Modified = info.ModTime()
		}
		q, err := l.Load(id)
		if err != nil {
			s.Problem, s.Quiz = "unreadable", Quiz{Title: id, Questions: []QuizQuestion{}}
		} else {
			s.Quiz = q
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b Saved) int {
		if c := strings.Compare(fold(a.Quiz.Title), fold(b.Quiz.Title)); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

// Load reads one saved quiz.
func (l Library) Load(id string) (Quiz, error) {
	path, err := l.path(id)
	if err != nil {
		return Quiz{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Quiz{}, ErrQuizMissing
	}
	if err != nil {
		return Quiz{}, fmt.Errorf("%w: %w", ErrQuizFile, err)
	}
	if len(data) > MaxSavedQuizSize {
		return Quiz{}, ErrQuizFile
	}
	var f savedFile
	if err := json.Unmarshal(data, &f); err != nil || f.Version != savedVersion {
		return Quiz{}, ErrQuizFile
	}
	q := f.Quiz.Normalize()
	if err := q.Validate(); err != nil {
		return Quiz{}, fmt.Errorf("%w: %w", ErrQuizFile, err)
	}
	return q, nil
}

// Save writes q: over the quiz id, or as a new one named after its title
// when id is empty. It returns the id.
func (l Library) Save(id string, q Quiz) (string, error) {
	q = q.Normalize()
	if err := q.Validate(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return "", fmt.Errorf("quizzes: %w", err)
	}
	if id == "" {
		id = l.freeID(slug(q.Title))
	}
	path, err := l.path(id)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(savedFile{Version: savedVersion, Quiz: q}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("quizzes: %w", err)
	}
	tmp, err := os.CreateTemp(l.Dir, ".nuevo-*.tmp")
	if err != nil {
		return "", fmt.Errorf("quizzes: %w", err)
	}
	_, werr := tmp.Write(append(data, '\n'))
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("quizzes: %w", errors.Join(werr, cerr))
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("quizzes: %w", err)
	}
	return id, nil
}

// Delete removes a saved quiz.
func (l Library) Delete(id string) error {
	path, err := l.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, fs.ErrNotExist) {
		return ErrQuizMissing
	} else if err != nil {
		return fmt.Errorf("quizzes: %w", err)
	}
	return nil
}

func (l Library) path(id string) (string, error) {
	if l.Dir == "" || !idPattern.MatchString(id) {
		return "", ErrQuizMissing
	}
	return filepath.Join(l.Dir, id+".json"), nil
}

// freeID returns base, or base-2, base-3… if a file already has it.
func (l Library) freeID(base string) string {
	id := base
	for n := 2; ; n++ {
		if _, err := os.Stat(filepath.Join(l.Dir, id+".json")); errors.Is(err, fs.ErrNotExist) {
			return id
		}
		id = fmt.Sprintf("%s-%d", strings.TrimRight(base[:min(len(base), 55)], "-"), n)
	}
}

// slug turns a title into a file name: lower case ASCII letters, digits and
// dashes, at most 50 characters.
func slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range fold(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 50 {
			break
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "cuestionario"
	}
	return s
}

// fold is a title in lower case without accents, to sort and name files.
func fold(s string) string {
	return strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n").Replace(strings.ToLower(s))
}

// GradesName is the file name of the exported grades of a quiz: its title,
// safe for Windows, and «.csv».
func GradesName(title string) string {
	return slug(title) + ".csv"
}
