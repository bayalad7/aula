package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/bayalad7/aula/internal/security"
)

// TestMain keeps the tests away from the real machine: no ProgramData site
// key, no machine console settings.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "aula-teacher-test-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("AULA_SITE_KEY_DIR", filepath.Join(tmp, "sin-clave-del-centro"))
	machineConfigPath = func() string { return filepath.Join(tmp, "sin-config.json") }
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

func TestDefaultChannelAndPort(t *testing.T) {
	cases := []struct {
		m       machineConfig
		s       settings
		channel string
		port    int
	}{
		{machineConfig{}, settings{}, "1", DefaultPort},
		{machineConfig{}, settings{RecentChannels: []string{"7", "2"}, Port: 48001}, "7", 48001},
		{machineConfig{DefaultChannel: 2, Port: 48002}, settings{RecentChannels: []string{"7"}, Port: 48001}, "2", 48002},
		{machineConfig{DefaultChannel: 99999}, settings{RecentChannels: []string{"AULA_101", "3"}}, "3", DefaultPort},
	}
	for _, c := range cases {
		if got := defaultChannel(c.m, c.s); got != c.channel {
			t.Errorf("defaultChannel(%+v, %+v) = %s, want %s", c.m, c.s, got, c.channel)
		}
		if got := serverPort(c.m, c.s); got != c.port {
			t.Errorf("serverPort(%+v, %+v) = %d, want %d", c.m, c.s, got, c.port)
		}
	}
}

func TestMachineConfigFile(t *testing.T) {
	old := machineConfigPath
	t.Cleanup(func() { machineConfigPath = old })
	path := filepath.Join(t.TempDir(), "config.json")
	machineConfigPath = func() string { return path }
	if err := os.WriteFile(path, []byte(`{"default_channel": 2, "port": 48010}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	if a.defaultChannel != "2" || a.port != 48010 {
		t.Fatalf("channel %s port %d", a.defaultChannel, a.port)
	}
}

// The site key wins over a personal identity; without it the personal one is
// used (and created on first use).
func TestConsoleKeyPrefersSiteKey(t *testing.T) {
	site, user := t.TempDir(), t.TempDir()
	t.Cleanup(func() { unprotect(site) })
	personal, src, err := consoleKey(filepath.Join(site, "no-existe"), user, true)
	if err != nil || src != keyPersonal {
		t.Fatalf("without site key: %s %v", src, err)
	}
	centre, err := security.CreateSiteKey(site, false)
	if err != nil {
		t.Fatal(err)
	}
	got, src, err := consoleKey(site, user, true)
	if err != nil || src != keyCentre || !got.Public().Equal(centre.Public()) {
		t.Fatalf("with site key: %s %v", src, err)
	}
	if got.Public().Equal(personal.Public()) {
		t.Fatal("site key and personal key are the same")
	}
	if _, _, err := consoleKey(filepath.Join(site, "no-existe"), t.TempDir(), false); err == nil {
		t.Fatal("no key and create=false must fail")
	}
}

// Like LanSchool, the console opens on its default channel by itself; a
// failure is kept for the setup screen.
func TestAutoStartOnDefaultChannel(t *testing.T) {
	h := &harness{class: &fakeClass{}, clip: &fakeClipboard{}, keys: &fakeHotkeys{}, rec: &recorder{}}
	a := NewApp(t.TempDir(), nil, nil, h.keys, quiet)
	a.emit = h.rec.emit
	a.defaultChannel, a.defaultName, a.autoStart = "2", "Prof. Ayala", true
	started := make(chan classConfig, 1)
	a.start = func(_ context.Context, cfg classConfig, _ classHooks, _ *slog.Logger) (classroom, ClassInfo, error) {
		started <- cfg
		return h.class, ClassInfo{Channel: cfg.Channel, ConsoleName: cfg.ConsoleName}, nil
	}
	a.startup(context.Background())
	cfg := <-started
	if cfg.Channel != "2" || cfg.ConsoleName != "Prof. Ayala" {
		t.Fatalf("auto start with %+v", cfg)
	}
	waitUntil(t, "class event", func() bool { v, ok := h.rec.last(EventClass).(View); return ok && v.Active })

	b := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	rec := &recorder{}
	b.emit = rec.emit
	b.defaultChannel, b.defaultName, b.autoStart = "1", "Prof. Ayala", true
	b.start = func(context.Context, classConfig, classHooks, *slog.Logger) (classroom, ClassInfo, error) {
		return nil, ClassInfo{}, errors.New("el puerto 47830 está ocupado")
	}
	b.startup(context.Background())
	waitUntil(t, "failure event", func() bool { _, ok := rec.last(EventClass).(View); return ok })
	if s := b.Setup(); s.StartError == "" || s.DefaultChannel != "1" {
		t.Fatalf("setup after failed start = %+v", s)
	}
}
