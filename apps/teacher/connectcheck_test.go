package main

import (
	"context"
	"net"
	"testing"

	"github.com/bayalad7/aula/internal/connectcheck"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
)

type fixedNetwork struct{ st platform.NetworkState }

func (f fixedNetwork) Inspect(context.Context, string, int) (platform.NetworkState, error) {
	return f.st, nil
}

func TestCheckConnectionNeedsAClass(t *testing.T) {
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	if _, err := app.CheckConnection(); errorCode(err) != codeNoClass {
		t.Fatalf("err = %v, want %s", err, codeNoClass)
	}
}

// The checker against the real class: it reports every step, sends its
// progress, and reaches no student, real or simulated (the aula simulada
// stays as it was and nobody new shows up).
func TestCheckConnectionOnALiveClass(t *testing.T) {
	rec := &recorder{}
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	app.port, app.emit = 0, rec.emit
	view, err := app.StartClass("7", "Prueba")
	if err != nil {
		t.Fatal(err)
	}
	defer app.EndClass()
	if _, err := app.SimulationStart(2); err != nil {
		t.Fatal(err)
	}
	pub, _ := security.ParsePublicKey(view.Class.PublicKey)
	startAgents(t, 1, view.Class.Port, pub)
	waitUntil(t, "one real and two simulated students", func() bool {
		v := app.View()
		return v.Students.Connected == 1 && v.Students.Simulated == 2
	})
	before := app.View().Students

	app.checks.source = func(channel string) session.ConsoleSource {
		return session.StaticSource{localConsole(view.Class.Port, channel)}
	}
	app.checks.localAddrs = func() []net.IP { return []net.IP{net.IPv4(127, 0, 0, 1)} }
	app.checks.loopback = true
	app.checks.net = fixedNetwork{platform.NetworkState{
		Networks: []platform.Network{{Name: "Red 2", Category: platform.NetworkPublic}},
		Firewall: []platform.FirewallProfile{{Name: platform.NetworkPublic, Enabled: true}},
	}}

	r, err := app.CheckConnection()
	if err != nil {
		t.Fatal(err)
	}
	want := []connectcheck.Status{connectcheck.StatusOK, connectcheck.StatusOK, connectcheck.StatusOK, connectcheck.StatusFail}
	for i, s := range r.Steps {
		if s.Status != want[i] {
			t.Fatalf("step %s = %+v (all: %+v)", s.ID, s, r.Steps)
		}
	}
	if r.Cause != session.CauseNoConsole || r.Steps[3].Reason != connectcheck.ReasonPublicBlocked {
		t.Fatalf("report = %+v", r)
	}
	if last, ok := rec.last(EventConnectCheck).(connectcheck.Report); !ok || !last.Done {
		t.Fatalf("last %s event = %+v", EventConnectCheck, rec.last(EventConnectCheck))
	}

	after := app.View().Students
	if after.Connected != before.Connected || after.Simulated != before.Simulated || len(after.Students) != len(before.Students) {
		t.Fatalf("students changed: before %d/%d, after %d/%d", before.Connected, len(before.Students),
			after.Connected, len(after.Students))
	}
	for _, s := range app.Simulation().Students {
		if s.Notices != 0 || len(s.Snippets) != 0 {
			t.Fatalf("simulated student %s got something: %+v", s.Name, s)
		}
	}
}

func TestCheckConnectionOneAtATime(t *testing.T) {
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	app.port = 0
	if _, err := app.StartClass("7", "Prueba"); err != nil {
		t.Fatal(err)
	}
	defer app.EndClass()
	app.checks.mu.Lock()
	defer app.checks.mu.Unlock()
	if _, err := app.CheckConnection(); errorCode(err) != codeCheckRunning {
		t.Fatalf("err = %v, want %s", err, codeCheckRunning)
	}
}
