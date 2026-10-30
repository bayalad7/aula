package main

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

// These tests never touch the real platform.Power: recordingPower only
// records what would have been done.

type recordingPower struct {
	mu    sync.Mutex
	calls []string
}

func (p *recordingPower) add(s string) error {
	p.mu.Lock()
	p.calls = append(p.calls, s)
	p.mu.Unlock()
	return nil
}

func (p *recordingPower) Shutdown(force bool) error { return p.add(fmt.Sprintf("shutdown %v", force)) }
func (p *recordingPower) Reboot(force bool) error   { return p.add(fmt.Sprintf("reboot %v", force)) }
func (p *recordingPower) Logoff(s uint32, force bool) error {
	return p.add(fmt.Sprintf("logoff %d %v", s, force))
}

func (p *recordingPower) list() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.calls)
}

// manualClock fires the countdown when the test says so.
type manualClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*manualTimer
}

type manualTimer struct {
	f    func()
	dead bool
}

func (t *manualTimer) Stop() bool { t.dead = true; return true }

func (c *manualClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }

func (c *manualClock) AfterFunc(_ time.Duration, f func()) power.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &manualTimer{f: f}
	c.timers = append(c.timers, t)
	return t
}

// waitTimer waits until a timer is pending (the countdown arms them from
// other goroutines).
func (c *manualClock) waitTimer(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for _, tm := range c.timers {
			if !tm.dead {
				c.mu.Unlock()
				return
			}
		}
		c.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no timer armed")
}

// fireLast runs the countdown's current timer.
func (c *manualClock) fireLast(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	var last *manualTimer
	for _, tm := range c.timers {
		if !tm.dead {
			last = tm
		}
	}
	c.mu.Unlock()
	if last == nil {
		t.Fatal("no countdown running")
	}
	last.dead = true
	last.f()
}

// acks collects what the agents answer to the console.
type acks struct {
	mu   sync.Mutex
	list []protocol.Payload
}

func (a *acks) onMessage(_ context.Context, _ *transport.Conn, env protocol.Envelope) {
	switch env.Payload.(type) {
	case protocol.PowerAck, protocol.ErrorPayload:
		a.mu.Lock()
		a.list = append(a.list, env.Payload)
		a.mu.Unlock()
	}
}

// wait returns the first message after the n-th that matches.
func (a *acks) wait(t *testing.T, what string, cond func(protocol.Payload) bool) protocol.Payload {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		for _, p := range a.list {
			if cond(p) {
				a.mu.Unlock()
				return p
			}
		}
		a.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; got %+v", what, a.list)
	return nil
}

func ackStatus(st protocol.PowerStatus) func(protocol.Payload) bool {
	return func(p protocol.Payload) bool {
		a, ok := p.(protocol.PowerAck)
		return ok && a.Status == st
	}
}

// A console that sends power.action before auth.ok is rejected and nothing
// is ever shut down (docs/privacidad.md rule 29).
func TestPowerBeforeAuthIsRejected(t *testing.T) {
	dir := t.TempDir()
	id, err := security.CreateIdentity(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := security.LoadOrCreateCertificate(dir)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(cert), Logger: quiet})
	go func() { _ = srv.Serve(ctx, ln) }()

	cfg := Config{Channel: "1", ConsolePublicKey: security.EncodePublicKey(id.Public()),
		Direct: net.JoinHostPort("127.0.0.1", strconv.Itoa(ln.Addr().(*net.TCPAddr).Port))}
	pub, err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp(cfg, pub, nil, localHello(), nil, nil, quiet)
	rp := &recordingPower{}
	a.enablePower(rp)
	a.power.FlushWait = time.Millisecond
	a.start(ctx)
	defer a.stop()

	unauthorized := false
	timeout := time.After(10 * time.Second)
	for !unauthorized {
		select {
		case ev := <-srv.Events():
			if ev.Kind != transport.EventMessage {
				continue
			}
			switch p := ev.Envelope.Payload.(type) {
			case protocol.AuthChallenge:
				_, _ = ev.Conn.Send(ctx, protocol.PowerAction{ActionID: protocol.NewID(), Action: protocol.PowerShutdown, Force: true})
			case protocol.ErrorPayload:
				unauthorized = p.Code == protocol.CodeUnauthorized
			}
		case <-timeout:
			t.Fatal("power.action before auth.ok was not rejected")
		}
	}
	time.Sleep(100 * time.Millisecond)
	if calls := rp.list(); len(calls) != 0 {
		t.Fatalf("the OS was asked %v before auth.ok", calls)
	}
	if v := a.View(); v.Power.Notice != nil {
		t.Fatalf("notice shown before auth.ok: %+v", v.Power)
	}
	cancel()
	go func() {
		for range srv.Events() {
		}
	}()
}

// With an authenticated console: the notice comes first, the console hears
// each step, and the OS is asked once, only at the end.
func TestPowerCountdownWithLiveConsole(t *testing.T) {
	got := &acks{}
	c := startLiveConsoleWith(t, got.onMessage)
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: c.addr}
	pub, _ := cfg.Validate()
	a, rec := newTestApp(nil)
	a.cfg, a.pub = cfg, pub
	rp := &recordingPower{}
	a.enablePower(rp)
	clock := &manualClock{now: time.Now()}
	a.power.Clock, a.power.FlushWait = clock, time.Millisecond
	var onTop []bool
	var topMu sync.Mutex
	a.onTop = func(on bool) { topMu.Lock(); onTop = append(onTop, on); topMu.Unlock() }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx)
	defer a.stop()

	s := c.waitAgent(t, "agent with power", func(s session.Session) bool { return s.Can(protocol.CapPower) })
	order := protocol.PowerAction{ActionID: protocol.NewID(), Action: protocol.PowerReboot, DelayS: 60}
	if _, err := c.console.Send(ctx, s.ID, order); err != nil {
		t.Fatal(err)
	}
	got.wait(t, "countdown", ackStatus(protocol.PowerCountdown))
	v := a.View()
	if v.Power.Notice == nil || v.Power.Notice.RemainingS != 60 || v.Power.Verb != "reiniciar este equipo" || v.Power.Channel != "1" {
		t.Fatalf("notice %+v", v.Power)
	}
	if !strings.HasSuffix(v.Status.Banner, "· reinicio pendiente") {
		t.Fatalf("banner %q", v.Status.Banner)
	}
	if rec.last(EventPower) == nil || rec.raised == 0 {
		t.Fatal("the notice was not shown nor the window raised")
	}

	if err := a.PowerPostpone(); err != nil {
		t.Fatal(err)
	}
	p := got.wait(t, "postponed", ackStatus(protocol.PowerPostponed)).(protocol.PowerAck)
	if p.PostponesLeft != 1 || p.RemainingS != 360 {
		t.Fatalf("postponed ack %+v", p)
	}
	if calls := rp.list(); len(calls) != 0 {
		t.Fatalf("ran %v during the countdown", calls)
	}

	clock.fireLast(t)
	got.wait(t, "executing", ackStatus(protocol.PowerExecuting))
	if calls := rp.list(); !slices.Equal(calls, []string{"reboot false"}) {
		t.Fatalf("OS calls %v", calls)
	}
	topMu.Lock()
	defer topMu.Unlock()
	if len(onTop) == 0 || !onTop[0] {
		t.Fatalf("window not kept on top: %v", onTop)
	}
}

// As a service: without a helper only power is declared, a reboot is
// immediate and a log off is refused; with a helper the window postpones
// through the service and a log off without force goes to the helper.
func TestServiceAgentPower(t *testing.T) {
	got := &acks{}
	c := startLiveConsoleWith(t, got.onMessage)
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: c.addr}
	pub, _ := cfg.Validate()
	sa := newServiceAgent(cfg, pub, nil, quiet)
	rp := &recordingPower{}
	sa.app.power.Power = rp
	clock := &manualClock{now: time.Now()}
	sa.app.power.Clock, sa.app.power.FlushWait = clock, time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sa.app.start(ctx)
	defer sa.app.stop()

	s := c.waitAgent(t, "agent without helper", func(session.Session) bool { return true })
	if !slices.Equal(capsOf(s), []string{"power"}) || s.Username != "" {
		t.Fatalf("without a helper: %v as %q", capsOf(s), s.Username)
	}
	logoff, _ := c.console.Send(ctx, s.ID, protocol.PowerAction{ActionID: protocol.NewID(), Action: protocol.PowerLogoff})
	got.wait(t, "logoff refused", func(p protocol.Payload) bool {
		e, ok := p.(protocol.ErrorPayload)
		return ok && e.RefID == logoff && e.Code == protocol.CodeUnsupportedCapability
	})
	if _, err := c.console.Send(ctx, s.ID, protocol.PowerAction{ActionID: protocol.NewID(), Action: protocol.PowerReboot, DelayS: 600}); err != nil {
		t.Fatal(err)
	}
	got.wait(t, "immediate reboot", ackStatus(protocol.PowerExecuting))
	deadline := time.Now().Add(5 * time.Second)
	for len(rp.list()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if calls := rp.list(); !slices.Equal(calls, []string{"reboot false"}) {
		t.Fatalf("OS calls %v", calls)
	}
	// Two minutes later the computer is still on (it is a test): the notice
	// clears and a new order is accepted.
	clock.waitTimer(t)
	clock.fireLast(t)
	if v := sa.app.View(); v.Power.Notice != nil {
		t.Fatalf("stale notice %+v", v.Power)
	}

	// Ana logs on; her helper is ready.
	svcEnd, helperEnd := net.Pipe()
	go sa.serve(ctx, platform.Session{ID: 4, User: "ana", SID: "S-1-5-21-1-1001"}, svcEnd)
	h := newFakeHelper(t, helperEnd)
	_ = h.link.Notify(agentsvc.EventReady, agentsvc.Ready{Clipboard: true})
	s = c.waitAgent(t, "agent with helper", func(s session.Session) bool { return s.Username == "ana" })
	if !slices.Equal(capsOf(s), []string{"code.view", "msg.show", "power"}) {
		t.Fatalf("with a helper: %v", capsOf(s))
	}
	order := protocol.PowerAction{ActionID: protocol.NewID(), Action: protocol.PowerLogoff}
	if _, err := c.console.Send(ctx, s.ID, order); err != nil {
		t.Fatal(err)
	}
	got.wait(t, "countdown", func(p protocol.Payload) bool {
		a, ok := p.(protocol.PowerAck)
		return ok && a.ActionID == order.ActionID && a.Status == protocol.PowerCountdown
	})
	var v View
	if err := h.binding(t, &v, "View"); err != nil || v.Power.Notice == nil || v.Power.Verb != "cerrar tu sesión" {
		t.Fatalf("view through the helper: %+v, %v", v.Power, err)
	}
	if err := h.binding(t, nil, "PowerPostpone"); err != nil {
		t.Fatal(err)
	}
	got.wait(t, "postponed", ackStatus(protocol.PowerPostponed))
	clock.fireLast(t)
	h.waitPower(t, []string{"logoff 4 false"})
	clock.waitTimer(t)
	clock.fireLast(t) // still on two minutes later (it is a test)

	// The lab case: shutting down without force while Ana is logged on runs
	// in her session, never from SYSTEM (ERROR_SHUTDOWN_USERS_LOGGED_ON).
	if _, err := c.console.Send(ctx, s.ID, protocol.PowerAction{ActionID: protocol.NewID(), Action: protocol.PowerShutdown}); err != nil {
		t.Fatal(err)
	}
	clock.waitTimer(t)
	clock.fireLast(t)
	h.waitPower(t, []string{"logoff 4 false", "shutdown 0 false"})
	clock.waitTimer(t)
	clock.fireLast(t)

	// With force, the service does it.
	if _, err := c.console.Send(ctx, s.ID, protocol.PowerAction{ActionID: protocol.NewID(), Action: protocol.PowerShutdown, Force: true}); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for len(rp.list()) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if calls := rp.list(); !slices.Equal(calls, []string{"reboot false", "shutdown true"}) {
		t.Fatalf("service OS calls %v", calls)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !slices.Contains(h.evs, agentsvc.EventOnTop) {
		t.Fatalf("events %v", h.evs)
	}
}

// waitPower waits until the helper ran exactly want.
func (h *fakeHelper) waitPower(t *testing.T, want []string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		got := slices.Clone(h.power)
		h.mu.Unlock()
		if slices.Equal(got, want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	t.Fatalf("helper ran %v, want %v", h.power, want)
}
