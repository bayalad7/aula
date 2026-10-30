package docs

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/manual"
)

// consoleViews returns the views of the console (data-vista-de).
func consoleViews(t *testing.T) []string {
	t.Helper()
	html, err := os.ReadFile(consoleIndex)
	if err != nil {
		t.Fatal(err)
	}
	var views []string
	for _, m := range screenRe.FindAllStringSubmatch(string(html), -1) {
		views = append(views, m[1])
	}
	return views
}

// TestManual checks the manual the console embeds: every page parses, every
// link, include and image resolves, and every console view has its help page.
func TestManual(t *testing.T) {
	m, err := manual.Load(Files(), ManualDir, DiagramsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Pages) < 8 {
		t.Fatalf("only %d manual pages", len(m.Pages))
	}
	for _, err := range m.Validate(consoleViews(t)) {
		t.Error(err)
	}
}

// A diagram included in a manual page lists that page in secciones_manual, so
// the diagram's readers know where it is explained.
func TestDiagramsKnowTheirManualPages(t *testing.T) {
	m, err := manual.Load(Files(), ManualDir, DiagramsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range m.Pages {
		for _, b := range p.Blocks {
			if b.Kind != "diagram" {
				continue
			}
			data, err := os.ReadFile(diagramsDir + "/" + b.Diagram + ".md")
			if err != nil {
				t.Fatal(err)
			}
			fm, _, err := parseFrontMatter(strings.ReplaceAll(string(data), "\r\n", "\n"))
			if err != nil {
				t.Fatal(err)
			}
			want := ManualDir + "/" + p.Slug
			if !slices.Contains(fm.SeccionesManual, want) {
				t.Errorf("%s is included in %s but secciones_manual lacks %q", b.Diagram, p.Slug, want)
			}
		}
	}
}

// The diagnostic tree of «¿Falta un alumno?» comes from diagram 12.
func TestDiagnosticTree(t *testing.T) {
	m, err := manual.Load(Files(), ManualDir, DiagramsDir)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := manual.ParseTree(m.Diagrams["12-diagnostico-conexion"].Mermaid)
	if err != nil {
		t.Fatal(err)
	}
	if !tr.Nodes[tr.Nodes[tr.Start].Next].Question {
		t.Errorf("the tree should ask right after the start: %+v", tr.Nodes[tr.Start])
	}
}
