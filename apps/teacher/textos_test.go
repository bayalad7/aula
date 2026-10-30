package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every text the console shows lives in frontend/dist/textos.es-MX.js
// (one file, ready for other languages). This test fails if index.html or
// main.js use a key that does not exist, or if the file keeps unused keys.
func TestTextsAreComplete(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("frontend", "dist", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	textos, html := read("textos.es-MX.js"), read("index.html")
	var js strings.Builder
	for _, line := range strings.Split(read("main.js"), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			js.WriteString(line + "\n")
		}
	}
	main := js.String()

	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*"([A-Za-z0-9.]+)":\s*"`).FindAllStringSubmatch(textos, -1) {
		if defined[m[1]] {
			t.Errorf("textos: %q is defined twice", m[1])
		}
		defined[m[1]] = true
	}
	if len(defined) < 100 {
		t.Fatalf("textos.es-MX.js: only %d keys found; is the format still `\"key\": \"text\"`?", len(defined))
	}

	used := map[string]string{} // key → where
	need := func(key, where string) {
		if _, ok := used[key]; !ok {
			used[key] = where
		}
	}
	for _, m := range regexp.MustCompile(`data-t(?:-[a-z-]+)?="([A-Za-z0-9.]+)"`).FindAllStringSubmatch(html, -1) {
		need(m[1], "index.html")
	}
	// A data-info="x" button shows info.x.etiqueta and info.x.texto.
	for _, m := range regexp.MustCompile(`data-info="([a-z]+)"`).FindAllStringSubmatch(html, -1) {
		need("info."+m[1]+".etiqueta", "index.html (data-info)")
		need("info."+m[1]+".texto", "index.html (data-info)")
	}
	for _, m := range regexp.MustCompile(`\bT\("([A-Za-z0-9.]+)"`).FindAllStringSubmatch(main, -1) {
		need(m[1], "main.js")
	}
	for _, m := range regexp.MustCompile(`\bP\("([A-Za-z0-9.]+)"`).FindAllStringSubmatch(main, -1) {
		need(m[1]+".uno", "main.js (plural)")
		need(m[1]+".varios", "main.js (plural)")
	}
	// Keys built at run time ("error.x" + ".titulo", ACCION + "Sustantivo",
	// lists of keys): every literal that looks like a key must be a key, or
	// the base of one.
	sections := map[string]bool{}
	for key := range defined {
		sections[strings.SplitN(key, ".", 2)[0]] = true
	}
	for _, m := range regexp.MustCompile(`"([a-z][A-Za-z]*\.[A-Za-z0-9]+)"`).FindAllStringSubmatch(main, -1) {
		lit := m[1]
		if !sections[strings.SplitN(lit, ".", 2)[0]] {
			continue // a domain, a capability…: not a text
		}
		if defined[lit+"Sustantivo"] {
			need(lit+"Sustantivo", "main.js (composed)")
		}
		switch {
		case defined[lit]:
			need(lit, "main.js (literal)")
		case defined[lit+".titulo"] && defined[lit+".texto"]:
			need(lit+".titulo", "main.js (composed)")
			need(lit+".texto", "main.js (composed)")
		case defined[lit+".uno"] && defined[lit+".varios"]:
			need(lit+".uno", "main.js (composed)")
			need(lit+".varios", "main.js (composed)")
		default:
			t.Errorf("main.js uses %q, which is neither a text key nor the base of one", lit)
		}
	}

	// Error texts are chosen by the code Go returns (errores.go).
	for _, code := range errorCodes {
		need("error."+code+".titulo", "errores.go")
		need("error."+code+".texto", "errores.go")
	}

	// So are the texts of the students' protocol errors (agenterrors.go).
	for code := range agentErrorHelp {
		need("errorAlumno."+agentErrorKey(code)+".titulo", "agenterrors.go")
		need("errorAlumno."+agentErrorKey(code)+".texto", "agenterrors.go")
	}

	for key, where := range used {
		if !defined[key] {
			t.Errorf("%s uses the text %q, missing from textos.es-MX.js", where, key)
		}
	}
	for key := range defined {
		if _, ok := used[key]; !ok {
			t.Errorf("textos.es-MX.js: %q is never used", key)
		}
	}
	if strings.Contains(main, "innerHTML") {
		t.Error("main.js must not use innerHTML")
	}
}
