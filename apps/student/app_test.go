package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeClipboard struct{ text string }

func (f *fakeClipboard) WriteText(_ context.Context, s string) error { f.text = s; return nil }
func (f *fakeClipboard) ReadText(context.Context) (string, error)    { return f.text, nil }

type recorder struct {
	mu     sync.Mutex
	events map[string][]any
	raised int
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

func newTestApp(clip *fakeClipboard) (*App, *recorder) {
	rec := &recorder{}

	a := NewApp(Config{Channel: "1"}, nil, nil, localHello(), nil, errors.New("sin portapapeles"), quiet)
	if clip != nil {
		a.clip = clip
	}
	a.emit = rec.emit
	a.raise = func() { rec.mu.Lock(); rec.raised++; rec.mu.Unlock() }
	return a, rec
}

const indented = "def f(n):\n    if n <= 1:\n        return 1\n\treturn n * f(n - 1)\n\n"

func TestCodeShareShowsAndCopiesExactly(t *testing.T) {
	clip := &fakeClipboard{}
	a, rec := newTestApp(clip)
	share := protocol.CodeShare{SnippetID: protocol.NewID(), Title: "F", Lang: "python", Content: indented, Mode: protocol.ModeReplace}
	if ack, ok := a.recv.Handle(protocol.New(share)).(protocol.CodeAck); !ok || ack.SnippetID != share.SnippetID {
		t.Fatal("no code.ack")
	}
	b, ok := rec.last(EventBoard).(Board)
	if !ok || len(b.Snippets) != 1 || b.Snippets[0].Content != indented {
		t.Fatalf("board event = %#v", rec.last(EventBoard))
	}
	if rec.raised != 1 {
		t.Fatal("window not raised on code.share")
	}
	if err := a.Copy(share.SnippetID); err != nil {
		t.Fatal(err)
	}
	if clip.text != indented {
		t.Fatalf("copied %q, want %q", clip.text, indented)
	}
	if err := a.Copy("otro"); err == nil {
		t.Fatal("copying an unknown snippet succeeded")
	}

	a.recv.Handle(protocol.New(protocol.CodeClear{}))
	if b := rec.last(EventBoard).(Board); b.Snippets == nil || len(b.Snippets) != 0 {
		t.Fatalf("clear must emit an empty (not null) list: %#v", b)
	}
	if v := a.View(); v.Snippets == nil || len(v.Snippets) != 0 {
		t.Fatal("View after clear")
	}
}

func TestCopyWithoutClipboardExplains(t *testing.T) {
	a, _ := newTestApp(nil)
	share := protocol.CodeShare{SnippetID: protocol.NewID(), Lang: "go", Content: "x", Mode: protocol.ModeReplace}
	a.recv.Handle(protocol.New(share))
	if err := a.Copy(share.SnippetID); err == nil || !strings.Contains(err.Error(), "portapapeles") {
		t.Fatalf("Copy = %v", err)
	}
}

func TestNoticesAckedCappedAndDismissed(t *testing.T) {
	a, rec := newTestApp(nil)
	var first string
	for i := range maxNotices + 2 {
		env := protocol.New(protocol.MsgShow{Text: "aviso", Level: protocol.LevelInfo})
		if i == 2 {
			first = env.ID
		}
		if ack, ok := a.recv.Handle(env).(protocol.MsgAck); !ok || ack.RefID != env.ID {
			t.Fatal("msg.show not acked")
		}
	}
	n := rec.last(EventNotices).(Notices)
	if len(n.Notices) != maxNotices || n.Notices[0].ID != first {
		t.Fatalf("notices = %d, oldest kept %s", len(n.Notices), n.Notices[0].ID)
	}
	a.DismissNotice(first)
	if n := rec.last(EventNotices).(Notices); len(n.Notices) != maxNotices-1 {
		t.Fatal("dismiss failed")
	}
	if len(a.View().Notices) != maxNotices-1 {
		t.Fatal("View does not reflect dismissal")
	}
}

func TestBannerAlwaysIdentifiesClass(t *testing.T) {
	a, rec := newTestApp(nil)
	a.onState(session.AgentConnected, discovery.Console{Name: "Lab 3 — Prof. Ayala"})
	s := rec.last(EventStatus).(Status)
	if s.Banner != "Aula — conectado a la clase de Lab 3 — Prof. Ayala (canal 1)" {
		t.Fatalf("banner = %q", s.Banner)
	}
	a.onState(session.AgentDisconnected, discovery.Console{Name: "Lab 3"})
	if s := rec.last(EventStatus).(Status); s.State != StateDisconnected || !strings.Contains(s.Banner, "Lab 3") {
		t.Fatalf("disconnected banner = %+v", s)
	}
	u := NewApp(Config{}, nil, errors.New("falta la clave"), localHello(), nil, nil, quiet)
	if v := u.View(); v.Status.State != StateUnconfigured || v.Status.Detail != "falta la clave" {
		t.Fatalf("unconfigured status = %+v", v.Status)
	}
}

func TestLoadConfig(t *testing.T) {
	id, _ := security.GenerateIdentity()
	key := security.EncodePublicKey(id.Public())
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"channel":"1","console_public_key":"`+key+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, pub, err := loadConfig([]string{"-config", path})
	if err != nil || cfg.Channel != "1" || !pub.Equal(id.Public()) {
		t.Fatalf("from file: %+v %v", cfg, err)
	}
	cfg, _, err = loadConfig([]string{"-config", path, "-canal", "2", "-directo", "10.0.0.5:47830"})
	if err != nil || cfg.Channel != "2" || cfg.Direct != "10.0.0.5:47830" {
		t.Fatalf("flags override: %+v %v", cfg, err)
	}
	missing := filepath.Join(dir, "no-existe.json")
	if _, _, err := loadConfig([]string{"-config", missing}); err == nil || !strings.Contains(err.Error(), "-clave") {
		t.Fatalf("missing config: %v", err)
	}
	if _, _, err := loadConfig([]string{"-config", missing, "-canal", "1", "-clave", "aula-ed25519:xx"}); err == nil {
		t.Fatal("invalid key accepted")
	}
	if _, _, err := loadConfig([]string{"-config", missing, "-canal", "aula 101", "-clave", key}); err == nil {
		t.Fatal("invalid channel accepted")
	}
}

// The machine file written by the installer wins over the user's own file.
func TestMachineConfigWinsOverUserConfig(t *testing.T) {
	teacher, _ := security.GenerateIdentity()
	rogue, _ := security.GenerateIdentity()
	dir := t.TempDir()
	machine, user := filepath.Join(dir, "machine.json"), filepath.Join(dir, "user.json")
	oldM, oldU := machineConfigPath, userConfigPath
	t.Cleanup(func() { machineConfigPath, userConfigPath = oldM, oldU })
	machineConfigPath = func() string { return machine }
	userConfigPath = func() (string, error) { return user, nil }

	write := func(path, channel string, id *security.Identity) {
		data, err := Config{Channel: channel, ConsolePublicKey: security.EncodePublicKey(id.Public())}.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(user, "666", rogue)
	if cfg, _, err := loadConfig(nil); err != nil || cfg.Channel != "666" {
		t.Fatalf("user file alone: %+v %v", cfg, err)
	}
	write(machine, "1", teacher)
	cfg, pub, err := loadConfig(nil)
	if err != nil || cfg.Channel != "1" || !pub.Equal(teacher.Public()) {
		t.Fatalf("machine file must win: %+v %v", cfg, err)
	}
	_ = os.Remove(user)
	_ = os.Remove(machine)
	if _, _, err := loadConfig(nil); err == nil || !strings.Contains(err.Error(), "aula-alumno.json") {
		t.Fatalf("no config: %v", err)
	}
}

// The event names in events.js must be the ones the backend emits.
func TestEventContractMatchesFrontend(t *testing.T) {
	js, err := os.ReadFile(filepath.Join("frontend", "dist", "events.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{EventStatus, EventBoard, EventNotices} {
		if !strings.Contains(string(js), `"`+name+`"`) {
			t.Errorf("events.js does not define %q", name)
		}
	}
	main, _ := os.ReadFile(filepath.Join("frontend", "dist", "main.js"))
	for _, method := range []string{"View", "Copy", "DismissNotice"} {
		if !strings.Contains(string(main), "api."+method+"(") {
			t.Errorf("main.js does not call App.%s", method)
		}
	}
}

// Received text must never reach innerHTML except through Prism.
func TestFrontendDoesNotInjectHTML(t *testing.T) {
	main, _ := os.ReadFile(filepath.Join("frontend", "dist", "main.js"))
	if n := strings.Count(string(main), "innerHTML"); n != 1 || !strings.Contains(string(main), "innerHTML = window.Prism.highlight(") {
		t.Fatalf("main.js uses innerHTML %d times outside Prism.highlight", n)
	}
}
