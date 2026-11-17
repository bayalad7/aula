package ui

import (
	"io/fs"
	"os"
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
	for _, name := range []string{"index.html", "main.js", "sistema/tokens.css", "sistema/componentes.css", "sistema/utilidades.css", "sistema/iconos.svg"} {
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
	for _, css := range []string{"tokens.css", "componentes.css", "utilidades.css"} {
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

var cssCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

// The utilities only lay things out (docs/ux/DESIGN_SYSTEM.md §6): no color,
// shadow, radius, font or motion, and no loose color values. Those belong to
// the components, so a utility never paints or animates (not even in .alumno).
func TestUtilitiesOnlyArrange(t *testing.T) {
	data, err := fs.ReadFile(Files(), "utilidades.css")
	if err != nil {
		t.Fatal(err)
	}
	css := cssCommentRe.ReplaceAllString(string(data), "")
	if !strings.HasPrefix(strings.TrimSpace(css), "@layer utilidades {") {
		t.Error("utilidades.css must be wrapped in @layer utilidades")
	}
	forbidden := regexp.MustCompile(`^(color|background|border|outline|box-shadow|text-shadow|filter|backdrop-filter|opacity|transition|animation|will-change|transform|font|line-height|letter-spacing|fill|stroke|z-index)`)
	declRe := regexp.MustCompile(`([a-z-]+)\s*:\s*([^;{}]+);`)
	n := 0
	for _, m := range declRe.FindAllStringSubmatch(css, -1) {
		prop, value := m[1], m[2]
		n++
		if forbidden.MatchString(prop) && prop != "font-variant-numeric" {
			t.Errorf("utilidades.css: %s: a utility does not set %s", prop, prop)
		}
		if strings.Contains(value, "#") || strings.Contains(value, "rgb") || strings.Contains(value, "hsl") {
			t.Errorf("utilidades.css: %s: %s is a loose color", prop, value)
		}
		if regexp.MustCompile(`\d(px|em)\b`).MatchString(value) {
			t.Errorf("utilidades.css: %s: %s: use a token or rem", prop, value)
		}
	}
	if n == 0 {
		t.Error("utilidades.css has no declarations")
	}
}

// The cascade goes in layers (docs/decisiones.md, D-022): tokens.css declares
// the order first and every sheet of the design system and of the apps wraps
// its rules in its own layer, so an unlayered rule never wins by accident.
func TestCSSIsLayered(t *testing.T) {
	const order = "@layer tokens, componentes, utilidades, vista;"
	sheets := map[string]string{
		"tokens.css":      "tokens",
		"componentes.css": "componentes",
		"utilidades.css":  "utilidades",
		"../../apps/teacher/frontend/dist/style.css": "vista",
		"../../apps/student/frontend/dist/style.css": "vista",
	}
	for file, layer := range sheets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		css := strings.TrimSpace(cssCommentRe.ReplaceAllString(string(data), ""))
		if file == "tokens.css" {
			if !strings.HasPrefix(css, order) {
				t.Errorf("tokens.css must start with %q", order)
				continue
			}
			css = strings.TrimSpace(strings.TrimPrefix(css, order))
		}
		if !strings.HasPrefix(css, "@layer "+layer+" {") || !strings.HasSuffix(css, "}") {
			t.Errorf("%s: its rules must all be inside @layer %s { … }", file, layer)
		}
	}
}
