package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

// Wails binds every exported method of App; the web harness of the student
// app (scripts/ux/dobles-alumno.js) must fake each one, or its walkthrough
// tests a window that no longer exists (prompt 19).
func TestHarnessFakesEveryBinding(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "ux", "dobles-alumno.js"))
	if err != nil {
		t.Fatal(err)
	}
	fakes := map[string]bool{}
	for _, m := range regexp.MustCompile(`\b([A-Z][A-Za-z]*): call\("([A-Za-z]+)"`).FindAllStringSubmatch(string(b), -1) {
		if m[1] != m[2] {
			t.Errorf("dobles-alumno.js: %s records its calls as %q", m[1], m[2])
		}
		fakes[m[1]] = true
	}
	app := reflect.TypeOf(&App{})
	for i := range app.NumMethod() {
		name := app.Method(i).Name
		if !fakes[name] {
			t.Errorf("scripts/ux/dobles-alumno.js does not fake the binding App.%s", name)
		}
		delete(fakes, name)
	}
	for name := range fakes {
		t.Errorf("scripts/ux/dobles-alumno.js fakes App.%s, which no longer exists", name)
	}
}
