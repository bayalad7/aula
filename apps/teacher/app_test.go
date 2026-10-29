package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeClass stands in for the network: it records broadcasts and answers like
// the console (envelope IDs per session, capability errors).
type fakeClass struct {
	mu       sync.Mutex
	sessions []session.Session
	sent     []protocol.Payload
	envIDs   []string // of the last broadcast
	direct   []directSend
	stopped  bool
}

// directSend is a message sent to one session (screen thumbnails).
type directSend struct {
	sessionID, envID string
	p                protocol.Payload
}

func (f *fakeClass) Send(_ context.Context, sessionID string, p protocol.Payload) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := protocol.NewID()
	f.direct = append(f.direct, directSend{sessionID: sessionID, envID: id, p: p})
	return id, nil
}

// takeDirect returns and forgets the messages sent to single sessions.
func (f *fakeClass) takeDirect() []directSend {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.direct
	f.direct = nil
	return d
}

func (f *fakeClass) Broadcast(_ context.Context, p protocol.Payload) (map[string]string, map[string]error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, p)
	f.envIDs = nil
	sent, failed := map[string]string{}, map[string]error{}
	for _, s := range f.sessions {
		if s.State != session.StateConnected {
			continue
		}
		if c, need := protocol.RequiredCapability(p.MessageType()); need && !s.Can(c) {
			failed[s.ID] = fmt.Errorf("x: %w", session.ErrUnsupportedCapability)
			continue
		}
		id := protocol.NewID()
		sent[s.ID] = id
		f.envIDs = append(f.envIDs, id)
	}
	return sent, failed
}

func (f *fakeClass) Sessions() []session.Session { f.mu.Lock(); defer f.mu.Unlock(); return f.sessions }
func (f *fakeClass) Stop()                       { f.mu.Lock(); f.stopped = true; f.mu.Unlock() }

func (f *fakeClass) last() protocol.Payload {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return nil
	}
	return f.sent[len(f.sent)-1]
}

type fakeClipboard struct{ text string }

func (c *fakeClipboard) WriteText(_ context.Context, s string) error { c.text = s; return nil }
func (c *fakeClipboard) ReadText(context.Context) (string, error)    { return c.text, nil }

// fakeHotkeys keeps the callback so the test can "press" the hotkey.
type fakeHotkeys struct {
	pressed func()
	err     error
}

func (h *fakeHotkeys) Register(_ context.Context, _ platform.Hotkey, fn func()) error {
	h.pressed = fn
	return h.err
}

type recorder struct {
	mu     sync.Mutex
	events map[string][]any
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.events == nil {
		r.events = map[string][]any{}
	}
	r.events[name] = append(r.events[name], data)
}

func (r *recorder) last(name string) any {
	r.mu.Lock()
	defer r.mu.Unlock()
	l := r.events[name]
	if len(l) == 0 {
		return nil
	}
	return l[len(l)-1]
}

func student(id, host string, caps ...protocol.Capability) session.Session {
	return session.Session{ID: id, Hostname: host, OS: protocol.OSWindows, State: session.StateConnected, Capabilities: caps}
}

type harness struct {
	app   *App
	class *fakeClass
	clip  *fakeClipboard
	keys  *fakeHotkeys
	rec   *recorder
	hooks classHooks
}

func newHarness(t *testing.T, sessions ...session.Session) *harness {
	t.Helper()
	h := &harness{class: &fakeClass{sessions: sessions}, clip: &fakeClipboard{}, keys: &fakeHotkeys{}, rec: &recorder{}}
	h.app = NewApp(t.TempDir(), h.clip, nil, h.keys, quiet)
	h.app.emit = h.rec.emit
	h.app.start = func(_ context.Context, cfg classConfig, hooks classHooks, _ *slog.Logger) (classroom, ClassInfo, error) {
		h.hooks = hooks
		return h.class, ClassInfo{Channel: cfg.Channel, ConsoleName: cfg.ConsoleName, PublicKey: "aula-ed25519:x", Port: cfg.Port}, nil
	}
	h.app.startup(context.Background())
	if _, err := h.app.StartClass(" 01 ", "Prof. Ayala"); err != nil {
		t.Fatal(err)
	}
	return h
}

const pySnippet = "def f(n):\n    return 1 if n <= 1 else n * f(n - 1)\n"

// The emit binding builds the right code.share, broadcasts it through the
// class and counts the acks that reference the envelopes it sent.
func TestEmitBroadcastsAndCountsAcks(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView, protocol.CapMsgShow),
		student("s2", "PC02", protocol.CapCodeView, protocol.CapMsgShow),
		student("s3", "PC03", protocol.CapMsgShow)) // no code.view

	d, err := h.app.Emit(EmitRequest{Title: "Factorial", Lang: "python", Content: "def f(n):\r\n    return 1\r\n", Mode: "replace"})
	if err != nil {
		t.Fatal(err)
	}
	share, ok := h.class.last().(protocol.CodeShare)
	if !ok {
		t.Fatalf("broadcast %T, want CodeShare", h.class.last())
	}
	if share.Mode != protocol.ModeReplace || share.Exercise != "1" || share.Title != "Factorial" ||
		share.Lang != "python" || share.Content != "def f(n):\n    return 1\n" {
		t.Fatalf("code.share = %+v", share)
	}
	if d.Sent != 2 || d.NoCapability != 1 || d.Acked != 0 || d.Label != "Ejercicio 1 · Factorial" {
		t.Fatalf("delivery = %+v", d)
	}

	// Acks arrive from the console goroutine.
	h.hooks.onAck(h.class.envIDs[0])
	h.hooks.onAck(h.class.envIDs[0]) // duplicate: counted once
	h.hooks.onAck(protocol.NewID())  // unknown
	h.hooks.onAck(h.class.envIDs[1])
	got := h.rec.last(EventDeliveries).(Deliveries).Deliveries[0]
	if got.ID != d.ID || got.Acked != 2 {
		t.Fatalf("after acks: %+v", got)
	}
}

func TestHistoryEmitAppendReemit(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapCodeView))
	first, err := h.app.Emit(EmitRequest{Title: "Uno", Lang: "python", Content: pySnippet, Mode: "replace"})
	if err != nil {
		t.Fatal(err)
	}
	firstShare := h.class.last().(protocol.CodeShare)
	if _, err := h.app.Emit(EmitRequest{Title: "Dos", Lang: "go", Content: "package main\n", Mode: "append"}); err != nil {
		t.Fatal(err)
	}
	if s := h.class.last().(protocol.CodeShare); s.Mode != protocol.ModeAppend || s.Exercise != "2" || s.SnippetID == firstShare.SnippetID {
		t.Fatalf("append share = %+v", s)
	}

	hist := h.rec.last(EventHistory).(History).Entries
	if len(hist) != 2 || hist[0].Number != 1 || hist[1].Number != 2 {
		t.Fatalf("history = %+v", hist)
	}

	// Re-emit the first one: same snippet and number, replace mode, no new entry.
	again, err := h.app.Reemit(hist[0].SnippetID)
	if err != nil {
		t.Fatal(err)
	}
	s := h.class.last().(protocol.CodeShare)
	if s.SnippetID != firstShare.SnippetID || s.Exercise != "1" || s.Mode != protocol.ModeReplace || s.Content != pySnippet {
		t.Fatalf("re-emitted share = %+v", s)
	}
	if again.ID == first.ID || !strings.Contains(again.Label, "reemitido") {
		t.Fatalf("re-emit delivery = %+v", again)
	}
	v := h.app.View()
	if len(v.History) != 2 || v.History[0].Times != 2 {
		t.Fatalf("history after re-emit = %+v", v.History)
	}
	if _, err := h.app.Reemit("no-existe"); err == nil {
		t.Fatal("re-emitting an unknown snippet succeeded")
	}
}

func TestClearAndNotice(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapCodeView, protocol.CapMsgShow))
	if _, err := h.app.ClearScreens(); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.class.last().(protocol.CodeClear); !ok {
		t.Fatal("ClearScreens did not broadcast code.clear")
	}
	d, err := h.app.SendNotice("  Guarden su trabajo  ", "warning")
	if err != nil {
		t.Fatal(err)
	}
	if m := h.class.last().(protocol.MsgShow); m.Text != "Guarden su trabajo" || m.Level != protocol.LevelWarning || d.Kind != "notice" {
		t.Fatalf("notice = %+v / %+v", m, d)
	}
	h.hooks.onAck(h.class.envIDs[0]) // msg.ack counts too
	if got := h.rec.last(EventDeliveries).(Deliveries).Deliveries[0]; got.Acked != 1 {
		t.Fatalf("msg.ack not counted: %+v", got)
	}
	if _, err := h.app.SendNotice("", "info"); err == nil {
		t.Fatal("empty notice accepted")
	}
	if _, err := h.app.SendNotice("x", "urgente"); err == nil {
		t.Fatal("unknown level accepted")
	}
}

func TestEmitValidation(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapCodeView))
	for _, req := range []EmitRequest{
		{Lang: "python", Content: "   \n", Mode: "replace"},
		{Lang: "python", Content: "x", Mode: "prepend"},
		{Lang: "Python 3", Content: "x", Mode: "replace"},
		{Lang: "python", Content: strings.Repeat("x", protocol.MaxCodeContent+1), Mode: "replace"},
	} {
		if _, err := h.app.Emit(req); err == nil {
			t.Errorf("Emit(%.40v) accepted", req)
		}
	}
	if len(h.app.View().History) != 0 {
		t.Fatal("invalid emissions reached the history")
	}
	h.app.EndClass()
	if !h.class.stopped {
		t.Fatal("EndClass did not stop the class")
	}
	if _, err := h.app.Emit(EmitRequest{Lang: "python", Content: "x", Mode: "replace"}); !errors.Is(err, errNoClass) {
		t.Fatalf("Emit without class = %v", err)
	}
}

// The hotkey loads the clipboard into a draft and never emits on its own.
func TestHotkeyCapturesClipboardWithoutEmitting(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapCodeView))
	if !h.app.View().Hotkey.Active || h.keys.pressed == nil {
		t.Fatal("hotkey not registered at startup")
	}
	h.clip.text = "import math\r\n\r\ndef area(r):\r\n    return math.pi * r ** 2\r\n"
	h.keys.pressed()

	c, ok := h.rec.last(EventCapture).(Capture)
	if !ok || c.Draft == nil {
		t.Fatalf("capture event = %#v", h.rec.last(EventCapture))
	}
	if c.Draft.Content != "import math\n\ndef area(r):\n    return math.pi * r ** 2\n" ||
		c.Draft.Lang != "python" || c.Draft.Lines != 4 || c.Draft.Source != "hotkey" {
		t.Fatalf("draft = %+v", c.Draft)
	}
	if h.class.last() != nil {
		t.Fatal("capturing emitted something")
	}

	// Empty clipboard: an error event, still nothing emitted.
	h.clip.text = "  \n"
	h.keys.pressed()
	if c := h.rec.last(EventCapture).(Capture); c.Error == "" || c.Draft != nil {
		t.Fatalf("empty capture = %+v", c)
	}
	// The floating button does the same.
	h.clip.text = "SELECT * FROM alumnos;"
	if d, err := h.app.CaptureClipboard(); err != nil || d.Lang != "sql" || d.Source != "button" {
		t.Fatalf("button capture = %+v, %v", d, err)
	}
	if h.class.last() != nil {
		t.Fatal("capturing emitted something")
	}
}

func TestHotkeyUnavailableIsExplained(t *testing.T) {
	a := NewApp(t.TempDir(), nil, errors.New("sin portapapeles"), &fakeHotkeys{err: platform.ErrHotkeyInUse}, quiet)
	a.startup(context.Background())
	if hk := a.View().Hotkey; hk.Active || !strings.Contains(hk.Error, "Capturar") {
		t.Fatalf("hotkey status = %+v", hk)
	}
	if _, err := a.CaptureClipboard(); err == nil {
		t.Fatal("capture without clipboard succeeded")
	}
}

func TestStudentsViewAndChannelRules(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapCodeView, protocol.CapMsgShow), student("s2", "PC02", protocol.CapMsgShow))
	h.class.mu.Lock()
	h.class.sessions[1].LastCodeAckAt = time.Date(2026, 10, 28, 10, 0, 5, 0, time.Local)
	h.class.mu.Unlock()
	h.hooks.onChange()
	s := h.rec.last(EventStudents).(Students)
	if s.Connected != 2 || s.CanCode != 1 || s.CanNotice != 2 || s.Students[1].CanCode || s.Students[1].AckedAt != "10:00:05" {
		t.Fatalf("students = %+v", s)
	}
	if v := h.app.View(); v.Class.Channel != "1" { // normalized to the canonical number
		t.Fatalf("channel = %q", v.Class.Channel)
	}
	if _, err := h.app.StartClass("aula 1", "x"); err == nil {
		t.Fatal("invalid channel accepted")
	}
	if got := h.app.Setup().RecentChannels; len(got) != 1 || got[0] != "1" {
		t.Fatalf("recent channels = %v", got)
	}
}

func TestGuessLang(t *testing.T) {
	cases := map[string]string{
		"def f():\n    pass\n":                           "python",
		"package main\n\nfunc main() {}\n":               "go",
		"#include <iostream>\nint main(){std::cout<<1;}": "cpp",
		"#include <stdio.h>\nint main(){printf(\"x\");}": "c",
		"public class A { }":                             "java",
		"console.log('hola');":                           "javascript",
		"select nombre from alumnos where id = 1;":       "sql",
		"<!doctype html>\n<html></html>":                 "html",
	}
	for code, want := range cases {
		if got := guessLang(code, "plaintext"); got != want {
			t.Errorf("guessLang(%q) = %s, want %s", code, got, want)
		}
	}
	if got := guessLang("x = 1", "kotlin"); got != "kotlin" {
		t.Errorf("no hint should keep the previous language, got %s", got)
	}
}

// Event names and bindings used by the frontend must exist in Go.
func TestEventContractMatchesFrontend(t *testing.T) {
	js, err := os.ReadFile(filepath.Join("frontend", "dist", "events.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{EventStudents, EventHistory, EventDeliveries, EventCapture, EventClass, EventFrame, EventBroadcast} {
		if !strings.Contains(string(js), `"`+name+`"`) {
			t.Errorf("events.js does not define %q", name)
		}
	}
	main, _ := os.ReadFile(filepath.Join("frontend", "dist", "main.js"))
	for _, m := range []string{"Setup", "StartClass", "View", "Emit", "Reemit", "ClearScreens", "SendNotice", "CaptureClipboard",
		"OpenScreens", "CloseScreens", "ViewScreen", "Displays", "StartBroadcast", "StopBroadcast"} {
		if !strings.Contains(string(main), "api."+m+"(") {
			t.Errorf("main.js does not call App.%s", m)
		}
	}
	if strings.Contains(string(main), "innerHTML") {
		t.Error("main.js must not use innerHTML")
	}
	for _, lang := range languages {
		if !strings.Contains(string(main), `"`+lang+`"`) {
			t.Errorf("language %q missing from the selector", lang)
		}
	}
}

func TestPortFromSettings(t *testing.T) {
	dir := t.TempDir()
	if err := saveSettings(dir, settings{Port: 48000}); err != nil {
		t.Fatal(err)
	}
	if a := NewApp(dir, nil, nil, &fakeHotkeys{}, quiet); a.port != 48000 {
		t.Fatalf("port = %d, want 48000", a.port)
	}
	if a := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet); a.port != DefaultPort {
		t.Fatalf("default port = %d", a.port)
	}
}
