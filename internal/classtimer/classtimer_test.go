package classtimer

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// fakeClock is a clock the test moves by hand.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: time.Unix(1_792_771_200, 0)} }

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) Add(d time.Duration) {
	f.mu.Lock()
	f.t = f.t.Add(d)
	f.mu.Unlock()
}

const (
	refA = "7d1f2e3a-4b5c-4d6e-a7f8-9a0b1c2d3e4f"
	refB = "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10"
	tid1 = "e4b1c2d3-5f6a-4b7c-8d9e-0f1a2b3c4d5e"
	tid2 = "c2e1a3b4-5d6e-4f70-8a9b-0c1d2e3f4a5b"
)

func student(id, host string, caps ...protocol.Capability) session.Session {
	if caps == nil {
		caps = []protocol.Capability{protocol.CapTimer}
	}
	return session.Session{ID: id, Hostname: host, Username: "alumno", State: session.StateConnected, Capabilities: caps}
}

// --- Agent side --------------------------------------------------------------

func TestCountdownStartEndsOnce(t *testing.T) {
	clk := newClock()
	c := NewCountdown(clk.Now)
	acks := c.Start(refA, protocol.TimerStart{TimerID: tid1, Label: "Ejercicio 1", DurationS: 60, RemainingMS: 60_000})
	if len(acks) != 1 || acks[0].Status != protocol.TimerRunning || acks[0].RemainingMS != 60_000 || acks[0].RefID != refA {
		t.Fatalf("start acks = %+v", acks)
	}
	clk.Add(20 * time.Second)
	if v := c.View(); v.State != StateRunning || v.RemainingMS != 40_000 || v.Label != "Ejercicio 1" {
		t.Fatalf("view = %+v", v)
	}
	if d, ok := c.Next(); !ok || d != 40*time.Second {
		t.Fatalf("next = %v %v", d, ok)
	}
	if _, ended, changed := c.Tick(); ended || changed {
		t.Fatal("tick before the end changed something")
	}
	clk.Add(40 * time.Second)
	ack, ended, changed := c.Tick()
	if !ended || !changed || ack.Status != protocol.TimerEnded || ack.RefID != refA || ack.TimerID != tid1 {
		t.Fatalf("tick at zero = %+v %v %v", ack, ended, changed)
	}
	if err := ack.Validate(); err != nil {
		t.Fatalf("ended ack invalid: %v", err)
	}
	if v := c.View(); v.State != StateEnded || v.RemainingMS != 0 {
		t.Fatalf("view after the end = %+v", v)
	}
	if _, ended, _ := c.Tick(); ended {
		t.Fatal("it ended twice")
	}
	clk.Add(EndedLinger)
	if _, _, changed := c.Tick(); !changed || c.View().State != StateNone {
		t.Fatalf("the ended badge did not go away: %+v", c.View())
	}
	if _, ok := c.Next(); ok {
		t.Fatal("nothing should be pending")
	}
}

func TestCountdownExtendTakesRemainingAndRevives(t *testing.T) {
	clk := newClock()
	c := NewCountdown(clk.Now)
	c.Start(refA, protocol.TimerStart{TimerID: tid1, DurationS: 60, RemainingMS: 60_000})
	clk.Add(60 * time.Second)
	c.Tick()
	ext := protocol.TimerExtend{TimerID: tid1, AddS: 300, RemainingMS: 300_000}
	ack, ok := c.Extend(refB, ext)
	if !ok || ack.Status != protocol.TimerRunning || ack.RemainingMS != 300_000 {
		t.Fatalf("extend = %+v %v", ack, ok)
	}
	// A resend of the same extension does not add twice.
	clk.Add(10 * time.Second)
	c.Extend(refB, protocol.TimerExtend{TimerID: tid1, AddS: 300, RemainingMS: 290_000})
	if v := c.View(); v.State != StateRunning || v.RemainingMS != 290_000 {
		t.Fatalf("view = %+v", v)
	}
	if _, ok := c.Extend(refB, protocol.TimerExtend{TimerID: tid2, AddS: 60, RemainingMS: 1000}); ok {
		t.Fatal("an extension of another timer was applied")
	}
}

func TestCountdownStopAndReplace(t *testing.T) {
	clk := newClock()
	c := NewCountdown(clk.Now)
	if _, ok := c.Stop(refA, protocol.TimerStop{TimerID: tid1}); ok {
		t.Fatal("stop without a timer was not ignored")
	}
	c.Start(refA, protocol.TimerStart{TimerID: tid1, DurationS: 60, RemainingMS: 60_000})
	acks := c.Start(refB, protocol.TimerStart{TimerID: tid2, Label: "Ejercicio 2", DurationS: 120, RemainingMS: 120_000})
	if len(acks) != 2 || acks[0].Status != protocol.TimerStopped || acks[0].TimerID != tid1 || acks[0].RefID != refA ||
		acks[1].Status != protocol.TimerRunning || acks[1].TimerID != tid2 {
		t.Fatalf("replace acks = %+v", acks)
	}
	if _, ok := c.Stop(refA, protocol.TimerStop{TimerID: tid1}); ok {
		t.Fatal("stop of the replaced timer was applied")
	}
	ack, ok := c.Stop(refA, protocol.TimerStop{TimerID: tid2})
	if !ok || ack.Status != protocol.TimerStopped || ack.TimerID != tid2 || c.View().State != StateNone {
		t.Fatalf("stop = %+v %v, view %+v", ack, ok, c.View())
	}
}

func TestCountdownReconnection(t *testing.T) {
	clk := newClock()
	c := NewCountdown(clk.Now)
	c.Start(refA, protocol.TimerStart{TimerID: tid1, DurationS: 600, RemainingMS: 600_000})
	// The connection drops: the count goes on.
	clk.Add(30 * time.Second)
	c.Reconnected()
	if d, _ := c.Next(); d != ReconnectGrace {
		t.Fatalf("next after reconnecting = %v", d)
	}
	// The console sends it again with what is left by its clock.
	clk.Add(time.Second)
	c.Start(refB, protocol.TimerStart{TimerID: tid1, DurationS: 600, RemainingMS: 568_000})
	clk.Add(ReconnectGrace)
	if _, _, changed := c.Tick(); changed {
		t.Fatal("a confirmed timer was dropped")
	}
	if v := c.View(); v.RemainingMS != 565_000 {
		t.Fatalf("remaining = %d", v.RemainingMS)
	}
	// Reconnected again, but the console has no timer any more.
	c.Reconnected()
	clk.Add(ReconnectGrace)
	if _, _, changed := c.Tick(); !changed || c.View().State != StateNone {
		t.Fatalf("an unconfirmed timer stayed: %+v", c.View())
	}
}

// --- Console side ------------------------------------------------------------

func TestClassStartPlanAndAck(t *testing.T) {
	clk := newClock()
	c := NewClass(clk.Now)
	a, b, old := student("s1", "LAB-01"), student("s2", "LAB-02"), student("s3", "LAB-03", protocol.CapCodeView)
	id, err := c.Start("  Ejercicio 1 ", 300, nil)
	if err != nil {
		t.Fatal(err)
	}
	clk.Add(2 * time.Second)
	cmds := c.Plan([]session.Session{a, b, old})
	if len(cmds) != 2 {
		t.Fatalf("plan = %+v", cmds)
	}
	for _, cmd := range cmds {
		p := cmd.Payload.(protocol.TimerStart)
		if p.TimerID != id || p.Label != "Ejercicio 1" || p.DurationS != 300 || p.RemainingMS != 298_000 {
			t.Fatalf("start = %+v", p)
		}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.Plan([]session.Session{a, b, old})) != 0 {
		t.Fatal("a second plan sent again")
	}
	if !c.Ack("s1", protocol.TimerAck{RefID: refA, TimerID: id, Status: protocol.TimerRunning, RemainingMS: 297_990}) {
		t.Fatal("ack not recorded")
	}
	if c.Ack("s1", protocol.TimerAck{RefID: refA, TimerID: tid2, Status: protocol.TimerRunning}) {
		t.Fatal("ack of another timer recorded")
	}
	s := c.Snapshot([]session.Session{a, old}, []session.Session{b})
	if !s.Active || s.Label != "Ejercicio 1" || !s.All || s.Eligible != 2 || s.RemainingMS != 298_000 ||
		len(s.Unsupported) != 1 || s.Unsupported[0] != "LAB-03" || len(s.Receivers) != 2 {
		t.Fatalf("snapshot = %+v", s)
	}
	if s.Receivers[0].Status != ReceiverRunning || s.Receivers[1].Status != ReceiverSent || !s.Receivers[1].Simulated {
		t.Fatalf("receivers = %+v", s.Receivers)
	}
}

func TestClassExtendStopAndEnd(t *testing.T) {
	clk := newClock()
	c := NewClass(clk.Now)
	a := student("s1", "LAB-01")
	if err := c.Extend(60); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("extend without timer = %v", err)
	}
	id, _ := c.Start("", 60, nil)
	c.Plan([]session.Session{a})
	clk.Add(60 * time.Second)
	if s := c.Snapshot([]session.Session{a}, nil); !s.Ended || s.RemainingMS != 0 {
		t.Fatalf("not ended: %+v", s)
	}
	c.Ack("s1", protocol.TimerAck{RefID: refA, TimerID: id, Status: protocol.TimerEnded})
	// A student who connects after the end gets nothing.
	late := student("s2", "LAB-02")
	if cmds := c.Plan([]session.Session{a, late}); len(cmds) != 0 {
		t.Fatalf("plan after the end = %+v", cmds)
	}
	// Extending revives it: an extension for who had it, a start for the late one.
	if err := c.Extend(300); err != nil {
		t.Fatal(err)
	}
	clk.Add(time.Second)
	cmds := c.Plan([]session.Session{a, late})
	if len(cmds) != 2 {
		t.Fatalf("plan after extending = %+v", cmds)
	}
	for _, cmd := range cmds {
		switch p := cmd.Payload.(type) {
		case protocol.TimerExtend:
			if cmd.SessionID != "s1" || p.AddS != 300 || p.RemainingMS != 299_000 || p.Validate() != nil {
				t.Fatalf("extend = %+v", p)
			}
		case protocol.TimerStart:
			if cmd.SessionID != "s2" || p.RemainingMS != 299_000 || p.DurationS != 60 || p.Validate() != nil {
				t.Fatalf("start = %+v", p)
			}
		default:
			t.Fatalf("unexpected %T", p)
		}
	}
	if s := c.Snapshot([]session.Session{a, late}, nil); s.Ended || s.Receivers[0].Status != ReceiverSent {
		t.Fatalf("snapshot after extending = %+v", s)
	}
	if err := c.Stop(); err != nil {
		t.Fatal(err)
	}
	cmds = c.Plan([]session.Session{a, late})
	if len(cmds) != 2 || cmds[0].Payload.(protocol.TimerStop).TimerID != id {
		t.Fatalf("plan after stopping = %+v", cmds)
	}
	if len(c.Plan([]session.Session{a, late})) != 0 || c.Snapshot(nil, nil).Active {
		t.Fatal("stop was sent twice or the timer stayed")
	}
	if err := c.Stop(); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("second stop = %v", err)
	}
}

func TestClassLimits(t *testing.T) {
	c := NewClass(newClock().Now)
	for _, d := range []int{0, protocol.MinTimerDurationS - 1, protocol.MaxTimerDurationS + 1} {
		if _, err := c.Start("x", d, nil); !errors.Is(err, ErrInvalid) {
			t.Errorf("duration %d: %v", d, err)
		}
	}
	if _, err := c.Start(strings.Repeat("ñ", protocol.MaxTimerLabel+1), 60, nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("long label: %v", err)
	}
	c.Start("x", protocol.MaxTimerDurationS-120, nil)
	if err := c.Extend(protocol.MinTimerExtendS - 1); !errors.Is(err, ErrInvalid) {
		t.Errorf("short extension: %v", err)
	}
	if err := c.Extend(180); !errors.Is(err, ErrTooLong) {
		t.Errorf("past 4 h: %v", err)
	}
	if s := c.Snapshot(nil, nil); s.MaxExtendS != 120 {
		t.Errorf("max extension = %d", s.MaxExtendS)
	}
	if err := c.Extend(120); err != nil {
		t.Errorf("up to 4 h: %v", err)
	}
}

func TestClassSelectionReconnectAndReplace(t *testing.T) {
	clk := newClock()
	c := NewClass(clk.Now)
	a, b := student("s1", "LAB-01"), student("s2", "LAB-02")
	first, _ := c.Start("Ejercicio 1", 600, []string{"lab-01"})
	if cmds := c.Plan([]session.Session{a, b}); len(cmds) != 1 || cmds[0].SessionID != "s1" {
		t.Fatalf("selection plan = %+v", cmds)
	}
	if s := c.Snapshot([]session.Session{a, b}, nil); s.All || s.Eligible != 1 || len(s.Hosts) != 1 {
		t.Fatalf("selection snapshot = %+v", s)
	}
	// LAB-01 reconnects: a new session gets it again with what is left.
	clk.Add(100 * time.Second)
	a2 := student("s9", "LAB-01")
	cmds := c.Plan([]session.Session{a2, b})
	if len(cmds) != 1 || cmds[0].SessionID != "s9" || cmds[0].Payload.(protocol.TimerStart).RemainingMS != 500_000 ||
		cmds[0].Payload.(protocol.TimerStart).TimerID != first {
		t.Fatalf("reconnect plan = %+v", cmds)
	}
	// A failed send is retried.
	c.Failed(cmds[0])
	if len(c.Plan([]session.Session{a2, b})) != 1 {
		t.Fatal("a failed send was not retried")
	}
	// A new timer for everyone replaces it.
	second, _ := c.Start("Ejercicio 2", 120, nil)
	cmds = c.Plan([]session.Session{a2, b})
	if len(cmds) != 2 {
		t.Fatalf("replace plan = %+v", cmds)
	}
	for _, cmd := range cmds {
		if p := cmd.Payload.(protocol.TimerStart); p.TimerID != second {
			t.Fatalf("replace sent %+v", p)
		}
	}
	// The ack that stops the replaced one changes nothing.
	if c.Ack("s9", protocol.TimerAck{RefID: refA, TimerID: first, Status: protocol.TimerStopped}) {
		t.Fatal("ack of the replaced timer changed the view")
	}
	c.Reset()
	if c.Snapshot(nil, nil).Active || len(c.Plan([]session.Session{a2, b})) != 0 {
		t.Fatal("reset kept something")
	}
}

// TestClockSkewDoesNotMatter: two agents whose wall clocks differ by
// minutes count the same, because only what is left travels.
func TestClockSkewDoesNotMatter(t *testing.T) {
	console := newClock()
	cls := NewClass(console.Now)
	cls.Start("Ejercicio 1", 60, nil)
	cmds := cls.Plan([]session.Session{student("s1", "A"), student("s2", "B")})
	fast, slow := newClock(), newClock()
	fast.Add(7 * time.Minute)
	slow.Add(-3 * time.Minute)
	ca, cb := NewCountdown(fast.Now), NewCountdown(slow.Now)
	ca.Start(refA, cmds[0].Payload.(protocol.TimerStart))
	cb.Start(refB, cmds[1].Payload.(protocol.TimerStart))
	fast.Add(15 * time.Second)
	slow.Add(15 * time.Second)
	if ra, rb := ca.View().RemainingMS, cb.View().RemainingMS; ra != 45_000 || rb != 45_000 {
		t.Fatalf("remaining %d and %d, want 45000 both", ra, rb)
	}
}
