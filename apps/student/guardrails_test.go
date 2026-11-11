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
