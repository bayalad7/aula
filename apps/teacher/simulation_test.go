package main

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
)

// lockedBuffer collects logs written from several goroutines.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuffer) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func deliveryOf(app *App, id string) (d struct{ Sent, Acked, SimSent, SimAcked int }) {
	for _, x := range app.View().Deliveries {
		if x.ID == id {
			d.Sent, d.Acked, d.SimSent, d.SimAcked = x.Sent, x.Acked, x.SimSent, x.SimAcked
		}
	}
	return d
}

// TestSimulatedClassEndToEnd is the goal of the aula simulada: with no lab,
// the teacher opens a class, starts 2 simulated students and sees the first
// code arrive, all through the real stack, with the simulated students
// counted apart and kept away from power and restrictions.
func TestSimulatedClassEndToEnd(t *testing.T) {
	logs := &lockedBuffer{}
	rec := &recorder{}
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	app.port = 0
	app.emit = rec.emit
	if _, err := app.StartClass("7", "Prueba"); err != nil {
		t.Fatal(err)
	}
	defer app.EndClass()

	v, err := app.SimulationStart(0)
	if err != nil || !v.Active || len(v.Students) != DefaultSimulated {
		t.Fatalf("SimulationStart = %+v, %v", v, err)
	}
	waitUntil(t, "2 simulated students connected", func() bool { return app.View().Students.Simulated == 2 })
	st := app.View().Students
	if st.Connected != 0 || st.CanCode != 2 || st.CanPower != 0 {
		t.Errorf("counts = connected %d, canCode %d, canPower %d; simulated must not count as real", st.Connected, st.CanCode, st.CanPower)
	}
	for _, s := range st.Students {
		if !s.Simulated {
			t.Errorf("%s is not marked simulated", s.Hostname)
		}
	}

	// The first code arrives and is acknowledged, counted apart.
	d, err := app.Emit(EmitRequest{Title: "Factorial", Lang: "python", Content: pySnippet, Mode: "replace"})
	if err != nil || d.Sent != 0 || d.SimSent != 2 {
		t.Fatalf("Emit = %+v, %v", d, err)
	}
	waitUntil(t, "2 simulated acks", func() bool { return deliveryOf(app, d.ID).SimAcked == 2 })
	if got := deliveryOf(app, d.ID); got.Acked != 0 {
		t.Errorf("simulated acks counted as real: %+v", got)
	}
	waitUntil(t, "the code on their screens", func() bool {
		sim := app.Simulation()
		return len(sim.Students) == 2 && len(sim.Students[0].Snippets) == 1 && len(sim.Students[1].Snippets) == 1
	})
	if _, ok := rec.last(EventSimulation).(SimulationView); !ok {
		t.Error("no aula:simulation event")
	}

	// A student that does not acknowledge.
	first := app.Simulation().Students[0]
	if _, err := app.SimulationFault(first.ID, "noack", 0); err != nil {
		t.Fatal(err)
	}
	d2, _ := app.SendNotice("Hola", "info")
	waitUntil(t, "the other student's ack", func() bool { return deliveryOf(app, d2.ID).SimAcked == 1 })
	time.Sleep(300 * time.Millisecond)
	if got := deliveryOf(app, d2.ID); got.SimAcked != 1 {
		t.Errorf("noack student acknowledged: %+v", got)
	}

	// Power never reaches simulated students: there is nobody real to send it to.
	if _, err := app.PowerAction(PowerRequest{Action: "reboot"}); err == nil {
		t.Error("PowerAction with only simulated students should fail: they never get power")
	}

	// Their screens show in the screens view.
	if err := app.OpenScreens(); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "a simulated thumbnail", func() bool { return rec.last(EventFrame) != nil })
	app.CloseScreens()

	// Every log line about a simulated student says so.
	for _, line := range strings.Split(logs.String(), "\n") {
		if strings.Contains(line, "Simulado ") && !strings.Contains(line, "simulated=true") {
			t.Errorf("log line about a simulated student without simulated=true: %s", line)
		}
	}

	// Changing channel closes the aula simulada.
	if _, err := app.StartClass("8", "Prueba"); err != nil {
		t.Fatal(err)
	}
	if app.Simulation().Active {
		t.Error("the aula simulada survived a change of channel")
	}
	if app.View().Students.Simulated != 0 {
		t.Error("simulated students remain after a change of channel")
	}
}

// TestRealStudentNamedLikeSimulatedStaysReal: being simulated depends on the
// listener a student reached, never on its name.
func TestRealStudentNamedLikeSimulatedStaysReal(t *testing.T) {
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	app.port = 0
	view, err := app.StartClass("7", "Prueba")
	if err != nil {
		t.Fatal(err)
	}
	defer app.EndClass()
	pub, err := security.ParsePublicKey(view.Class.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	startAgentsOn(t, 1, "7", "Simulado", session.StaticSource{localConsole(view.Class.Port, "7")}, pub)
	waitUntil(t, "the real student", func() bool { return app.View().Students.Connected == 1 })
	st := app.View().Students
	if st.Simulated != 0 || len(st.Students) != 1 || st.Students[0].Simulated {
		t.Errorf("a real agent called %q was taken as simulated: %+v", st.Students[0].Hostname, st)
	}
}

func TestSimulationNeedsAClass(t *testing.T) {
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	if _, err := app.SimulationStart(1); err == nil {
		t.Error("SimulationStart without a class should fail")
	}
	if v := app.SimulationStop(); v.Active {
		t.Error("SimulationStop without a class reported an active simulation")
	}
}
