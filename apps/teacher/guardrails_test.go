package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/simclass"
)

// These tests keep a new feature from skipping what the console already
// promises (CLAUDE.md, «Definición de terminado»).

// Wails binds every exported method of App; the web harness (scripts/ux)
// must fake each one, or its walkthrough tests a console that no longer
// exists.
func TestHarnessFakesEveryBinding(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "ux", "dobles.js"))
	if err != nil {
		t.Fatal(err)
	}
	fakes := map[string]bool{}
	for _, m := range regexp.MustCompile(`\b([A-Z][A-Za-z]*): call\("([A-Za-z]+)"`).FindAllStringSubmatch(string(b), -1) {
		if m[1] != m[2] {
			t.Errorf("dobles.js: %s records its calls as %q", m[1], m[2])
		}
		fakes[m[1]] = true
	}
	app := reflect.TypeOf(&App{})
	for i := range app.NumMethod() {
		name := app.Method(i).Name
		if !fakes[name] {
			t.Errorf("scripts/ux/dobles.js does not fake the binding App.%s", name)
		}
		delete(fakes, name)
	}
	for name := range fakes {
		t.Errorf("scripts/ux/dobles.js fakes App.%s, which no longer exists", name)
	}
}

// protocolConsts returns the string constants of internal/protocol/<file>
// declared in the const block whose comment starts with doc.
func protocolConsts(t *testing.T, file, doc string) []string {
	t.Helper()
	src := filepath.Join("..", "..", "internal", "protocol", file)
	f, err := parser.ParseFile(token.NewFileSet(), src, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST || g.Doc == nil || !strings.HasPrefix(g.Doc.Text(), doc) {
			continue
		}
		for _, spec := range g.Specs {
			for _, v := range spec.(*ast.ValueSpec).Values {
				s, err := strconv.Unquote(v.(*ast.BasicLit).Value)
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, s)
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no constants under %q in %s; did the declaration change?", doc, src)
	}
	return out
}

// Every implemented capability reaches the student's file («Qué puede hacer
// su equipo»): Student has a can* field for it and main.js lists that field.
// When a reserved capability (remote, poll…) is implemented, this asks for
// both.
func TestStudentFileShowsEveryCapability(t *testing.T) {
	main, err := os.ReadFile(filepath.Join("frontend", "dist", "main.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range protocolConsts(t, "capability.go", "Implemented capabilities") {
		st := studentsOf([]session.Session{{ID: "s", State: session.StateConnected,
			Capabilities: []protocol.Capability{protocol.Capability(c)}}}).Students[0]
		b, err := json.Marshal(st)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(b, &fields); err != nil {
			t.Fatal(err)
		}
		var set []string
		for k, v := range fields {
			if strings.HasPrefix(k, "can") && v == true {
				set = append(set, k)
			}
		}
		if len(set) != 1 {
			t.Errorf("capability %q sets %v in Student; want exactly one can* field (events.go, studentsOf)", c, set)
			continue
		}
		if !strings.Contains(string(main), `["`+set[0]+`", "ficha.`) {
			t.Errorf("capability %q (%s) is missing from «Qué puede hacer su equipo» in main.js (pintarFicha)", c, set[0])
		}
	}
}

// Simulated students only declare what a simulated student can honour
// without touching a machine. Anything that controls or changes a computer
// (power, restrictions, broadcast, remote control…) must never reach them;
// adding one here is a deliberate decision, with its privacy rule.
func TestSimulatedStudentsOnlyDeclareHarmlessCapabilities(t *testing.T) {
	// screen.snapshot only reads the drawn screen (rule 46: filed apart).
	// hand and msg.student start on the student's side and change nothing on
	// a computer (rules 34 and 50-52; prompt 10.6).
	// timer only shows a countdown in the window (rule 54; prompt 10.8).
	// file.push is acknowledged with an in-memory folder: nothing is written
	// (rule 59; prompt 10.9).
	// file.collect hands in their last code from memory: no disk is read and
	// nothing changes on a computer (rule 65; prompt 10.10).
	allowed := []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow, protocol.CapScreenThumb, protocol.CapScreenSnapshot,
		protocol.CapHand, protocol.CapMsgStudent, protocol.CapTimer, protocol.CapFilePush, protocol.CapFileCollect,
		// poll only shows a question and answers it (rules 71-74; prompt 10.12).
		protocol.CapPoll,
		// poll.quiz answers a quiz by chance and is never exported (rule 91;
		// prompt 10.20).
		protocol.CapPollQuiz,
		// class.end only clears their simulated window (rule 86; prompt 10.14).
		protocol.CapClassEnd,
		// computer.label only names their seat in the room, in memory (rule
		// 31 ter; prompt 10.15).
		protocol.CapComputerLabel}
	for _, c := range simclass.Capabilities {
		if !slices.Contains(allowed, c) {
			t.Errorf("simulated students may declare %q; review docs/privacidad.md (rules 32–35) before allowing it", c)
		}
	}
}

// Every button and every form field of the window carries a reference icon
// (user's request, 2026-11-11): a button has an <svg> or data-icono (drawn by
// sistema/iconos.js), a text field, list or text area has data-icono, a radio
// is an «opción agrupada» (.opcion-seg) with its icon, and a yes/no box of a
// form is an interruptor. Image buttons, links styled as buttons and the
// selection boxes of a list are the exceptions.
func TestEveryButtonAndFieldHasAnIcon(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend", "dist", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	line := func(i int) int { return strings.Count(html[:i], "\n") + 1 }
	exempt := []string{"nav-lateral__clase-compacta", "cap-marco", "alumno__pantalla", "enlace"}
	// A link styled as a button counts as a button.
	for _, m := range regexp.MustCompile(`(?s)<(?:button\b|a class="btn)([^>]*)>(.*?)</(?:button|a)>`).FindAllStringSubmatchIndex(html, -1) {
		attrs, inner := html[m[2]:m[3]], html[m[4]:m[5]]
		if strings.Contains(inner, "<svg") || strings.Contains(attrs, "data-icono=") {
			continue
		}
		if slices.ContainsFunc(exempt, func(c string) bool { return strings.Contains(attrs, c) }) {
			continue
		}
		t.Errorf("index.html:%d: a button without an icon (add data-icono): %s", line(m[0]), strings.TrimSpace(attrs))
	}
	for _, m := range regexp.MustCompile(`<(input|select|textarea)\b([^>]*)>`).FindAllStringSubmatchIndex(html, -1) {
		attrs := html[m[4]:m[5]]
		switch {
		case strings.Contains(attrs, `type="radio"`):
			if !strings.HasSuffix(strings.TrimSpace(html[max(0, m[0]-40):m[0]]), `<label class="opcion-seg">`) {
				t.Errorf("index.html:%d: a radio outside an «opción agrupada» (.opcion-seg)", line(m[0]))
			}
		case strings.Contains(attrs, `type="checkbox"`):
			if !strings.Contains(attrs, "interruptor") && !strings.Contains(attrs, "marca") && !strings.Contains(attrs, "alumnos-todos") {
				t.Errorf("index.html:%d: a form checkbox that is not an interruptor", line(m[0]))
			}
		case strings.Contains(attrs, `type="hidden"`), strings.Contains(attrs, `type="file"`):
		default:
			// data-icono-fijo: the icon is already in the markup (a field with a prefix).
			if !strings.Contains(attrs, "data-icono=") && !strings.Contains(attrs, "data-icono-fijo=") {
				t.Errorf("index.html:%d: a field without its reference icon (add data-icono)", line(m[0]))
			}
		}
	}
}
