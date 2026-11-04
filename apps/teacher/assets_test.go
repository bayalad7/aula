package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/bayalad7/aula/internal/ui"
)

// The window serves the shared design system (internal/ui) under sistema/,
// and no app keeps its own copy (CLAUDE.md: one source).
func TestFrontendServesTheSharedDesignSystem(t *testing.T) {
	files, err := frontend()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "main.js", "textos.es-MX.js", "sistema/tokens.css", "sistema/componentes.css", "sistema/iconos.svg"} {
		if _, err := fs.Stat(files, name); err != nil {
			t.Errorf("the console does not serve %s: %v", name, err)
		}
	}
	got, _ := fs.ReadFile(files, "sistema/tokens.css")
	want, _ := fs.ReadFile(ui.Files(), "tokens.css")
	if string(got) != string(want) {
		t.Error("sistema/tokens.css is not internal/ui's")
	}
	for _, app := range []string{"teacher", "student"} {
		dir := filepath.Join("..", app, "frontend", "dist", ui.Dir)
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("%s keeps a copy of the design system; it lives in internal/ui", dir)
		}
	}
}
