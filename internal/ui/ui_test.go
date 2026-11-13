package ui

import (
	"io/fs"
	"regexp"
	"strings"
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

// Every url() of the design system's CSS (the embedded title font) is served
// by Files, so the apps never need the internet to draw it.
func TestCSSURLsAreEmbedded(t *testing.T) {
	urlRe := regexp.MustCompile(`url\("?([^")]+)"?\)`)
	found := 0
	for _, css := range []string{"tokens.css", "componentes.css"} {
		data, err := fs.ReadFile(Files(), css)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range urlRe.FindAllSubmatch(data, -1) {
			ref := string(m[1])
			if strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "#") {
				continue
			}
			found++
			if _, err := fs.Stat(Files(), ref); err != nil {
				t.Errorf("%s: url(%s) is not embedded: %v", css, ref, err)
			}
		}
	}
	if found == 0 {
		t.Error("tokens.css no longer loads the title font")
	}
	if _, err := fs.Stat(Files(), "fuentes/OFL.txt"); err != nil {
		t.Errorf("the font ships without its license: %v", err)
	}
}

func TestColorReadsTheThemes(t *testing.T) {
	light, err := Color("c-vista-solido", false)
	if err != nil {
		t.Fatal(err)
	}
	dark, err := Color("c-vista-solido", true)
	if err != nil {
		t.Fatal(err)
	}
	if light == dark {
		t.Fatalf("light and dark are both %v", light)
	}
	if light.R != 0x0b || light.G != 0x5d || light.B != 0x6b || light.A != 0xff {
		t.Errorf("light c-vista-solido = %v, want #0b5d6b", light)
	}
	if _, err := Color("no-existe", false); err == nil {
		t.Error("an unknown token must fail")
	}
}
