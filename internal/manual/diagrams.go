package manual

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// The diagrams are drawn beforehand by scripts/docs/generar-diagramas, which
// writes a light and a dark SVG of each one to <diagramsDir>/generados/ and a
// manifest with the hash of the Mermaid block they were drawn from. The
// console only shows an SVG whose hash matches the diagram's current block;
// otherwise it falls back to the Mermaid text (docs tests forbid that state).

// GeneratedDir is where the pipeline writes, inside the diagrams folder.
const GeneratedDir = "generados"

// ManifestEntry is one diagram in generados/manifiesto.json.
type ManifestEntry struct {
	ID    string `json:"id"`
	Hash  string `json:"hash"`   // MermaidHash of the block it was drawn from
	Light string `json:"claro"`  // path relative to the diagrams folder
	Dark  string `json:"oscuro"` // idem
}

// Manifest is generados/manifiesto.json.
type Manifest struct {
	Generator string          `json:"generador"`
	Diagrams  []ManifestEntry `json:"diagramas"`
}

// MermaidHash identifies a Mermaid block: "sha256:<hex>" of its text, with \n
// line ends. The pipeline computes the same.
func MermaidHash(src string) string {
	sum := sha256.Sum256([]byte(strings.ReplaceAll(src, "\r\n", "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ReadManifest reads the manifest of the diagrams in diagramsDir. A missing
// manifest is not an error: it returns nil (nothing drawn yet).
func ReadManifest(fsys fs.FS, diagramsDir string) (*Manifest, error) {
	b, err := fs.ReadFile(fsys, path.Join(diagramsDir, GeneratedDir, "manifiesto.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("manifiesto.json: %w", err)
	}
	return &m, nil
}

// attachDrawings gives each diagram its SVGs, as data: URIs for an <img>
// (where an SVG never runs scripts), when the manifest says they were drawn
// from its current Mermaid block.
func (m *Manual) attachDrawings(fsys fs.FS, diagramsDir string) error {
	man, err := ReadManifest(fsys, diagramsDir)
	if err != nil || man == nil {
		return err
	}
	uri := func(rel string) (string, error) {
		b, err := fs.ReadFile(fsys, path.Join(diagramsDir, rel))
		if err != nil {
			return "", err
		}
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(b), nil
	}
	for _, e := range man.Diagrams {
		d, ok := m.Diagrams[e.ID]
		if !ok || e.Hash != MermaidHash(d.Mermaid) {
			continue
		}
		if d.Light, err = uri(e.Light); err != nil {
			return err
		}
		if d.Dark, err = uri(e.Dark); err != nil {
			return err
		}
		m.Diagrams[e.ID] = d
	}
	return nil
}

// section returns the blocks under the "## title" heading of a body, up to
// the next "## ".
func section(body, title string) []Block {
	_, rest, ok := strings.Cut(body, "\n## "+title+"\n")
	if !ok {
		return nil
	}
	if i := strings.Index(rest, "\n## "); i >= 0 {
		rest = rest[:i]
	}
	return parseBlocks(rest)
}
