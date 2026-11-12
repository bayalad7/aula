package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
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

// Transparency (docs/privacidad.md; prompt 19, Fase 5.2): nothing the window
// can call hides or turns off what tells the student the teacher sees or
// controls this computer. Every binding is listed here with why it is safe; a
// new one fails this test until someone checks it and adds it.
func TestNoBindingTurnsOffTransparency(t *testing.T) {
	reviewed := map[string]string{
		"View":               "reads the state",
		"Copy":               "copies a snippet",
		"DismissNotice":      "closes a teacher's notice (msg.show), not an indicator",
		"ReportMedia":        "reports WebRTC support",
		"BroadcastAnswer":    "answers the broadcast; the band is shown before, by Go",
		"BroadcastCandidate": "part of the broadcast connection",
		"BroadcastFailed":    "reports a failed connection; the band stays while the broadcast is on",
		"PowerPostpone":      "rule 27: postpones the countdown; the notice stays",
		"DismissPower":       "closes only a failed order's notice (power.Countdown.Dismiss)",
		"SetCompact":         "shrinks the window; the screen-viewed and controlled marks stay in the strip and the native indicator",
		"ReportWindow":       "the notification area icon's text",
		"RaiseHand":          "the student raises their own hand (§5.18); no indicator involved",
		"LowerHand":          "the student lowers their own hand; no indicator involved",
		"SendTeacherMessage": "sends a message to the teacher; no indicator involved",
		"AnswerPoll":         "the student answers a poll (§5.23, rule 71); no indicator involved",
		"OpenReceivedFolder": "opens in Explorer the folder of a push it saved (rule 56); no indicator involved",
		// Rule 61: only the activity folder the agent created, by its ID.
		"OpenCollectionFolder": "opens in Explorer the activity folder of a collection (rule 61); no indicator involved",
	}
	app := reflect.TypeOf(&App{})
	for i := range app.NumMethod() {
		name := app.Method(i).Name
		if _, ok := reviewed[name]; !ok {
			t.Errorf("new binding App.%s: check it cannot hide or turn off a transparency indicator, then list it here", name)
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
