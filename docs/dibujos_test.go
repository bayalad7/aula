package docs

import (
	"io/fs"
	"path"
	"regexp"
	"testing"

	"github.com/bayalad7/aula/internal/manual"
)

const regenerate = "Diagramas desactualizados: ejecuta scripts/docs/generar-diagramas"

// Every diagram has its light and dark SVG drawn from its current Mermaid
// block (docs/v1/diagramas/generados, by scripts/docs/generar-diagramas).
func TestDiagramsAreDrawn(t *testing.T) {
	m, err := manual.Load(Files(), ManualDir, DiagramsDir)
	if err != nil {
		t.Fatal(err)
	}
	man, err := manual.ReadManifest(Files(), DiagramsDir)
	if err != nil {
		t.Fatal(err)
	}
	if man == nil {
		t.Fatal(regenerate + " (no hay manifiesto)")
	}
	listed := map[string]bool{}
	for _, e := range man.Diagrams {
		listed[e.ID] = true
		d, ok := m.Diagrams[e.ID]
		if !ok {
			t.Errorf("manifiesto.json lists %s, which is not a diagram: %s", e.ID, regenerate)
			continue
		}
		if e.Hash != manual.MermaidHash(d.Mermaid) {
			t.Errorf("%s: %s", e.ID, regenerate)
		}
		for _, f := range []string{e.Light, e.Dark} {
			if _, err := fs.Stat(Files(), path.Join(DiagramsDir, f)); err != nil {
				t.Errorf("%s: %s (%v)", e.ID, regenerate, err)
			}
		}
	}
	for id := range m.Diagrams {
		if !listed[id] {
			t.Errorf("%s has no drawing: %s", id, regenerate)
		}
	}
}

// The SVGs are only drawing: no scripts, handlers, foreignObject or
// references outside the file. The console also shows them as <img>, where
// nothing runs, but the files must be clean on their own.
func TestDrawingsAreSanitized(t *testing.T) {
	checks := []struct {
		re   *regexp.Regexp
		what string
	}{
		{regexp.MustCompile(`(?i)<script`), "a <script>"},
		{regexp.MustCompile(`(?i)\son[a-z]+\s*=`), "an on* handler"},
		{regexp.MustCompile(`(?i)<foreignObject`), "a foreignObject"},
		{regexp.MustCompile(`(?i)\shref\s*=\s*["'][^#]`), "a link outside the file"},
		{regexp.MustCompile(`(?i)url\(\s*['"]?[^#'"\s)]`), "a url() outside the file"},
		{regexp.MustCompile(`(?i)@import`), "an @import"},
		{regexp.MustCompile(`(?i)https?://(?:[^w]|w[^w]|ww[^w]|www\.[^w]|www\.w[^3])`), "an external address"},
	}
	svgs, err := fs.Glob(Files(), path.Join(DiagramsDir, manual.GeneratedDir, "*.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(svgs) == 0 {
		t.Fatal(regenerate + " (no hay SVG)")
	}
	for _, name := range svgs {
		b, err := fs.ReadFile(Files(), name)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range checks {
			if c.re.Match(b) {
				t.Errorf("%s has %s: %q", path.Base(name), c.what, c.re.Find(b))
			}
		}
	}
}
