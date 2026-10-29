package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bayalad7/aula/internal/platform"
)

// JournalName is the journal's file name, in the agent's machine directory
// (C:\ProgramData\Aula\alumno\politica-activa.json).
const JournalName = "politica-activa.json"

// Record is what the web filter changed on the computer, written before
// changing it, so it can always be undone: by Clear, or at the next start
// after a crash or a power cut.
type Record struct {
	NRPT  bool                    `json:"nrpt"`  // the rule that sends DNS to the resolver
	DoH   []platform.SavedSetting `json:"doh"`   // browser policies as they were
	Since time.Time               `json:"since"` // when the filter was applied
}

// Journal stores the Record in a file.
type Journal struct{ Path string }

// Write stores r atomically (a temporary file renamed over the journal).
func (j Journal) Write(r Record) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(j.Path), 0o700); err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	tmp := j.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	if err := os.Rename(tmp, j.Path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("journal: %w", err)
	}
	return nil
}

// Read returns the stored Record; ok is false if there is none.
func (j Journal) Read() (r Record, ok bool, err error) {
	data, err := os.ReadFile(j.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("journal: %w", err)
	}
	if err := json.Unmarshal(data, &r); err != nil {
		// A damaged journal still means a filter may be left: undo what can
		// be undone without it.
		return Record{NRPT: true}, true, fmt.Errorf("journal damaged: %w", err)
	}
	return r, true, nil
}

// Remove deletes the journal; it is not an error if there is none.
func (j Journal) Remove() error {
	if err := os.Remove(j.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("journal: %w", err)
	}
	return nil
}
