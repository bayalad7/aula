package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
)

// Every text the student window shows lives in frontend/dist/textos.es-MX.js
// (one file, ready for other languages; prompt 19). Go sends states, not
// phrases. This test fails if index.html or main.js use a key that does not
// exist, if a state Go can send has no text, or if the file keeps unused keys.
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
	if len(defined) < 40 {
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
	for _, m := range regexp.MustCompile(`\bT\("([A-Za-z0-9.]+)"`).FindAllStringSubmatch(main, -1) {
		need(m[1], "main.js")
	}
	for _, m := range regexp.MustCompile(`\bP\("([A-Za-z0-9.]+)"`).FindAllStringSubmatch(main, -1) {
		need(m[1]+".uno", "main.js (plural)")
		need(m[1]+".varios", "main.js (plural)")
	}

	// Keys built from what Go sends: the connection state (Status.State), the
	// power order (power.Notice: action and state) and the restriction modes.
	for _, s := range []string{StateConnected, StateSearching, StateDisconnected, StateUnconfigured} {
		need("titulo."+s, "events.go (Status.State)")
		need("estado."+s, "events.go (Status.State)")
	}
	for _, a := range []protocol.PowerKind{protocol.PowerShutdown, protocol.PowerReboot, protocol.PowerLogoff} {
		need("titulo.pendiente."+string(a), "power.Notice.Action")
		for _, st := range []string{"cuenta", "ejecutando", "fallo"} {
			need("energia."+st+"."+string(a), "power.Notice.Action")
		}
	}
	for _, l := range []protocol.MsgLevel{protocol.LevelInfo, protocol.LevelWarning} {
		need("aviso."+string(l), "codedist.Notice.Level")
	}
	for _, k := range []string{"appsDeny", "appsAllow", "webDeny", "webAllow", "separador"} {
		need("banda.restricciones."+k, "policy.State (mode)")
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

	// The only HTML the page writes is Prism's, which escapes the code first.
	for i, line := range strings.Split(main, "\n") {
		if strings.Contains(line, "innerHTML") && !strings.Contains(line, "Prism.highlight") {
			t.Errorf("main.js:%d writes innerHTML outside Prism: %s", i+1, strings.TrimSpace(line))
		}
	}
	// No Spanish phrases left in the window's Go side: the window words them.
	for _, f := range []string{"app.go", "broadcast.go", "power.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "Aula — ") {
			t.Errorf("%s still builds a window text («Aula — …»): send a state instead", f)
		}
	}
}
