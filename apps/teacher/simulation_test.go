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
	// Nor remote control: they do not declare it, and only real sessions are looked up.
	if _, err := app.TakeControl(app.View().Students.Students[0].ID); errorCode(err) != codeRemoteUnsupported {
		t.Errorf("TakeControl of a simulated student = %v, want %s", err, codeRemoteUnsupported)
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

// «Participación» without a lab: a simulated student raises its hand and
// writes through the real stack; the teacher answers and lowers the hand;
// closing the aula simulada forgets what it sent.
func TestSimulatedStudentRaisesHandAndWrites(t *testing.T) {
	rec := &recorder{}
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	app.port = 0
	app.emit = rec.emit
	if _, err := app.StartClass("7", "Prueba"); err != nil {
		t.Fatal(err)
	}
	defer app.EndClass()
	if _, err := app.SimulationStart(1); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "a simulated student", func() bool { return app.View().Students.Simulated == 1 })
	sim := app.Simulation().Students[0]
	if _, err := app.SimulationHand(sim.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SimulationMessage(sim.ID, ""); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "hand and message", func() bool {
		p := app.Participation()
		return len(p.Hands) == 1 && len(p.Messages) == 1
	})
	p := app.Participation()
	if !p.Hands[0].Simulated || p.Hands[0].SessionID == "" || p.Pending != 2 {
		t.Fatalf("participation = %+v", p)
	}
	if row := app.View().Students.Students[0]; row.Hand != 1 || row.NewMessages != 1 {
		t.Errorf("badge = hand %d, messages %d", row.Hand, row.NewMessages)
	}
	waitUntil(t, "the ack", func() bool {
		s := app.Simulation().Students[0]
		return s.Sent == 1
	})
	d, err := app.ReplyToStudent(ReplyRequest{Student: p.Messages[0].Student, MessageID: p.Messages[0].MessageID, Text: "Sí"})
	if err != nil || d.SimSent != 1 {
		t.Fatalf("reply = %+v, %v", d, err)
	}
	waitUntil(t, "the reply on its screen", func() bool { return app.Simulation().Students[0].Notices == 1 })
	if got := app.Simulation().Students[0]; got.Private != 1 {
		t.Errorf("the reply did not arrive as a private message: %+v", got)
	}
	if _, err := app.SendPrivateNotice(p.Hands[0].SessionID, "Una pista", "info"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the private message", func() bool { return app.Simulation().Students[0].Private == 2 })
	if err := app.LowerHand(p.Hands[0].Student); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "its hand down", func() bool { return app.Simulation().Students[0].Hand == "lowered_by_teacher" })

	app.SimulationStop()
	if p := app.Participation(); len(p.Messages) != 0 {
		t.Errorf("simulated messages outlived the aula simulada: %+v", p.Messages)
	}
}
