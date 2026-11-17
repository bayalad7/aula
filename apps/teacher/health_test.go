package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/health"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

func sentOfType[T protocol.Payload](d []directSend) map[string]T {
	out := map[string]T{}
	for _, x := range d {
		if p, ok := x.p.(T); ok {
			out[x.sessionID] = p
		}
	}
	return out
}

// Who joins with health gets the destinations once; who does not declare it
// gets nothing (§5.27.1).
func TestHealthConfigOnJoin(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapHealth, protocol.CapHealthSelfTest),
		student("s2", "PC02", protocol.CapCodeView))
	h.class.takeDirect()
	h.hooks.onChange()
	h.hooks.onChange() // a second change does not send it again
	cfgs := sentOfType[protocol.HealthConfig](h.class.takeDirect())
	if len(cfgs) != 1 || cfgs["s1"].Internet != nil {
		t.Fatalf("health.config sent to %v", cfgs)
	}
}

// A report becomes alerts, the badge in the list and the Laboratorio
// count; a computer that leaves keeps its last report.
func TestHealthReportAlertsAndBadge(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapHealth), student("s2", "PC02", protocol.CapHealth))
	r := health.Simulated(2, 1, protocol.HealthFirst, "") // an almost full disk
	h.hooks.onHealth("s1", protocol.New(r))
	h.hooks.onHealth("s2", protocol.New(health.Simulated(1, 1, protocol.HealthFirst, "")))
	v := h.app.Lab()
	if v.Connected != 2 || v.WithAlerts != 1 {
		t.Fatalf("lab %+v", v)
	}
	st, ok := h.rec.last(EventStudents).(Students)
	if !ok {
		t.Fatal("no aula:students after an alert appeared")
	}
	for _, s := range st.Students {
		want := 0
		if s.ID == "s1" {
			want = 2 // system and documents
		}
		if s.HealthAlerts != want || (want > 0) != (s.HealthLevel == "warn") || !s.CanHealth {
			t.Errorf("%s: badge %d %q, canHealth %v", s.Hostname, s.HealthAlerts, s.HealthLevel, s.CanHealth)
		}
	}
	h.class.mu.Lock()
	h.class.sessions[0].State = session.StateDisconnected
	h.class.mu.Unlock()
	h.hooks.onChange()
	c, _ := h.app.lab.Get(labKey("PC01", false))
	if c.Connected || c.Report == nil {
		t.Fatalf("a computer that left: %+v", c)
	}
	if v := h.app.Lab(); v.WithAlerts != 0 {
		t.Fatalf("a disconnected computer does not count: %+v", v)
	}
}

// The clock comes from the registry (heartbeats): over 2 min is an alert.
func TestHealthClockFromHeartbeats(t *testing.T) {
	s := student("s1", "PC01", protocol.CapHealth)
	s.ClockSkew, s.ClockSamples = 5*time.Minute, 5
	h := newHarness(t, s)
	h.app.refreshHealth(h.class)
	c, _ := h.app.lab.Get(labKey("PC01", false))
	if len(c.Alerts) != 1 || c.Alerts[0].ID != health.AlertClock || *c.ClockMS != 300000 {
		t.Fatalf("clock: %+v", c)
	}
}

// «Revisar ahora»: to everybody with health, once every 10 s; nobody
// connected with health is its own error.
func TestCheckHealth(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapHealth), student("s2", "PC02", protocol.CapCodeView))
	now := time.Now()
	h.app.now = func() time.Time { return now }
	h.class.takeDirect()
	n, err := h.app.CheckHealth(nil)
	if err != nil || n != 1 {
		t.Fatalf("CheckHealth = %d, %v", n, err)
	}
	if reqs := sentOfType[protocol.HealthRequest](h.class.takeDirect()); len(reqs) != 1 {
		t.Fatalf("requests %v", reqs)
	}
	if _, err := h.app.CheckHealth(nil); errorCode(err) != codeHealthTooSoon {
		t.Fatalf("second within 10 s: %v", err)
	}
	if v := h.app.Lab(); v.RequestInMs != 10000 {
		t.Fatalf("requestInMs = %d", v.RequestInMs)
	}
	now = now.Add(health.RequestGap)
	if _, err := h.app.CheckHealth([]string{"s2"}); errorCode(err) != codeHealthNoTargets {
		t.Fatalf("a selection without health: %v", err)
	}
}

// The self-test: pending until it answers, and not twice at once.
func TestRunSelfTest(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapHealth, protocol.CapHealthSelfTest), student("s2", "PC02", protocol.CapHealth))
	h.class.takeDirect()
	n, err := h.app.RunSelfTest(nil)
	if err != nil || n != 1 {
		t.Fatalf("RunSelfTest = %d, %v", n, err)
	}
	tests := sentOfType[protocol.HealthSelfTest](h.class.takeDirect())
	if len(tests) != 1 {
		t.Fatalf("self-tests %v", tests)
	}
	if _, err := h.app.RunSelfTest(nil); errorCode(err) != codeHealthBusy {
		t.Fatalf("a second one while pending: %v", err)
	}
	res := health.SimulatedSelfTest(2, nil, protocol.NewID(), tests["s1"].TestID)
	h.hooks.onHealth("s1", protocol.New(res))
	c, _ := h.app.lab.Get(labKey("PC01", false))
	if c.Test == nil || c.Test.Status != health.TestDone || c.Test.Failed != 1 {
		t.Fatalf("test %+v", c.Test)
	}
	if _, err := h.app.RunSelfTest([]string{"s2"}); errorCode(err) != codeHealthNoTargets {
		t.Fatalf("PC02 cannot: %v", err)
	}
}

// Ajustes › Salud: a wrong destination or threshold is refused with its
// code; a good one is saved and sent again to the class.
func TestSaveHealthSettings(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapHealth))
	h.hooks.onChange()
	h.class.takeDirect()
	bad := h.app.DefaultHealthSettings()
	bad.Targets.Internet = "www.google.com"
	if _, err := h.app.SaveHealthSettings(bad); errorCode(err) != codeHealthTarget {
		t.Fatalf("no port: %v", err)
	}
	bad = h.app.DefaultHealthSettings()
	bad.Thresholds.LinkMinMbps = 50
	if _, err := h.app.SaveHealthSettings(bad); errorCode(err) != codeHealthThreshold {
		t.Fatalf("50 Mb/s: %v", err)
	}
	good := h.app.DefaultHealthSettings()
	good.Targets = health.Targets{Internet: "www.google.com:443", Services: []health.Service{{Name: "Licencias", Address: "10.0.0.5:27000"}}}
	if _, err := h.app.SaveHealthSettings(good); err != nil {
		t.Fatal(err)
	}
	cfgs := sentOfType[protocol.HealthConfig](h.class.takeDirect())
	if c := cfgs["s1"]; c.Internet == nil || c.Internet.Port != 443 || len(c.Services) != 1 {
		t.Fatalf("new destinations not sent: %+v", cfgs)
	}
	if _, err := os.Stat(filepath.Join(h.app.dir, healthFile)); err != nil {
		t.Fatalf("not saved: %v", err)
	}
	if s := h.app.HealthSettings(); s.Targets.Internet != "www.google.com:443" {
		t.Fatalf("read back %+v", s)
	}
	if !h.app.Lab().HasTargets {
		t.Fatal("Lab must say there are destinations")
	}
}

// Simulated students get health too, marked apart (rule 95): never mixed
// with a real computer of the same name.
func TestHealthSimulatedApart(t *testing.T) {
	if labKey("PC01", true) == labKey("PC01", false) {
		t.Fatal("a simulated computer shares its key with a real one")
	}
	h := newHarness(t, student("s1", "PC01", protocol.CapHealth))
	h.app.lab.Report(labKey("PC01", true), "PC01", true, health.Simulated(2, 1, protocol.HealthFirst, ""))
	h.hooks.onHealth("s1", protocol.New(health.Simulated(1, 1, protocol.HealthFirst, "")))
	real, _ := h.app.lab.Get(labKey("PC01", false))
	sim, _ := h.app.lab.Get(labKey("PC01", true))
	if real.Simulated || !sim.Simulated || len(real.Alerts) != 0 || len(sim.Alerts) == 0 {
		t.Fatalf("real %+v / simulated %+v", real, sim)
	}
}
