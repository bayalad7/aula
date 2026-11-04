package ui

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

// An app's frontend gets the design system under sistema/, and keeps its own
// files; a stale sistema/ copy in the app is never served.
func TestAssetsMountsTheDesignSystem(t *testing.T) {
	app := fstest.MapFS{
		"index.html":         {Data: []byte("<html>")},
		"main.js":            {Data: []byte("//")},
		"sistema/tokens.css": {Data: []byte("stale copy")},
	}
	a := Assets(app)
	for _, name := range []string{"index.html", "main.js", "sistema/tokens.css", "sistema/componentes.css", "sistema/iconos.svg"} {
		if _, err := fs.ReadFile(a, name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	got, _ := fs.ReadFile(a, "sistema/tokens.css")
	want, _ := fs.ReadFile(Files(), "tokens.css")
	if string(got) != string(want) {
		t.Error("sistema/tokens.css came from the app, not from internal/ui")
	}
	if fi, err := fs.Stat(a, "sistema"); err != nil || !fi.IsDir() {
		t.Errorf("sistema is not a directory: %v", err)
	}
	for _, bad := range []string{"sistema/nope.css", "../x", "/index.html"} {
		if _, err := a.Open(bad); err == nil {
			t.Errorf("Open(%q) succeeded", bad)
		}
	}
}
