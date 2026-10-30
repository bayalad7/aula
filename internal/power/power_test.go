package power

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// These tests never touch the real platform.Power: fakePower only records.

// events is the ordered record of what the countdown did.
type events struct {
	mu   sync.Mutex
	list []string
	sent []protocol.Payload
	ran  chan string
}

func newEvents() *events { return &events{ran: make(chan string, 8)} }

func (e *events) add(s string) {
	e.mu.Lock()
	e.list = append(e.list, s)
	e.mu.Unlock()
}

func (e *events) all() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.list...)
}

func (e *events) payloads() []protocol.Payload {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]protocol.Payload(nil), e.sent...)
}

type fakePower struct {
	ev  *events
	err error
}

func (f fakePower) record(s string) error {
	f.ev.add("os:" + s)
	f.ev.ran <- s
	return f.err
}

func (f fakePower) Shutdown(force bool) error {
	return f.record(fmt.Sprintf("shutdown force=%v", force))
}
func (f fakePower) Reboot(force bool) error { return f.record(fmt.Sprintf("reboot force=%v", force)) }
func (f fakePower) Logoff(s uint32, force bool) error {
	return f.record(fmt.Sprintf("logoff %d force=%v", s, force))
}

// fakeClock fires timers only when advanced.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	c       *fakeClock
	at      time.Time
	f       func()
	stopped bool
	fired   bool
}

func (t *fakeTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	was := !t.stopped && !t.fired
	t.stopped = true
	return was
}

func newClock() *fakeClock { return &fakeClock{now: time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{c: c, at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the time and runs, in order, the timers that are due.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*fakeTimer
	for _, t := range c.timers {
		if !t.stopped && !t.fired && !t.at.After(c.now) {
			t.fired = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	sort.Slice(due, func(i, j int) bool { return due[i].at.Before(due[j].at) })
	for _, t := range due {
		t.f()
	}
}

const (
	actionA = "c2e1a3b4-5d6e-4f70-8a9b-0c1d2e3f4a5b"
	actionB = "d3f2b4c5-6e7f-4a81-9bac-1d2e3f4a5b6c"
	envA    = "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10"
	envB    = "1d0b8f63-7b5c-4a8f-9e44-3c2a5f7b0d21"
	envX    = "2e1c9a74-8c6d-4b9a-8f55-4d3b6a8c1e32"
)

type rig struct {
	c     *Countdown
	ev    *events
	clock *fakeClock
	send  Sender
}

func newRig(t *testing.T, osErr error) *rig {
	t.Helper()
	ev := newEvents()
	r := &rig{ev: ev, clock: newClock()}
	r.c = &Countdown{
		Power: fakePower{ev: ev, err: osErr},
		OnChange: func(n *Notice) {
			if n == nil {
				ev.add("notice:none")
				return
			}
			ev.add(fmt.Sprintf("notice:%s %d left=%d", n.State, n.RemainingS, n.PostponesLeft))
		},
		Clock:     r.clock,
		FlushWait: time.Millisecond,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	r.send = func(_ context.Context, p protocol.Payload) error {
		ev.mu.Lock()
		ev.sent = append(ev.sent, p)
		ev.mu.Unlock()
		ev.add("send:" + describe(p))
		return nil
	}
	return r
}

func describe(p protocol.Payload) string {
	switch p := p.(type) {
	case protocol.PowerAck:
		return fmt.Sprintf("ack %s %s %d left=%d", p.ActionID[:4], p.Status, p.RemainingS, p.PostponesLeft)
	case protocol.ErrorPayload:
		return fmt.Sprintf("error %s %s", p.Code, p.RefID[:4])
	}
	return string(p.MessageType())
}

// handle delivers a message as the agent would: Handle, then its reply.
func (r *rig) handle(t *testing.T, envID string, p protocol.Payload) {
	t.Helper()
	reply, ok := r.c.Handle(context.Background(), r.send, protocol.Envelope{ID: envID, Type: p.MessageType(), Payload: p})
	if !ok {
		t.Fatalf("%s not handled", p.MessageType())
	}
	if reply != nil {
		_ = r.send(context.Background(), reply)
	}
}

func (r *rig) waitRun(t *testing.T) string {
	t.Helper()
	select {
	case s := <-r.ev.ran:
		return s
	case <-time.After(5 * time.Second):
		t.Fatalf("the action never ran; events: %q", r.ev.all())
	}
	return ""
}

func (r *rig) noRun(t *testing.T) {
	t.Helper()
	select {
	case s := <-r.ev.ran:
		t.Fatalf("ran %q too early; events: %q", s, r.ev.all())
	case <-time.After(20 * time.Millisecond):
	}
}

func equal(t *testing.T, got, want []string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("events\n got %q\nwant %q", got, want)
	}
}

func TestCountdownRunsOnceAtTheEnd(t *testing.T) {
	r := newRig(t, nil)
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerReboot})
	r.clock.Advance(29 * time.Second)
	r.noRun(t)
	r.clock.Advance(time.Second)
	if got := r.waitRun(t); got != "reboot force=false" {
		t.Fatalf("ran %q", got)
	}
	equal(t, r.ev.all(), []string{
		"notice:countdown 30 left=2", // the notice before the console hears
		"send:ack c2e1 countdown 30 left=2",
		"notice:executing 0 left=2",
		"send:ack c2e1 executing 0 left=0", // before the OS
		"os:reboot force=false",
	})
	r.clock.Advance(time.Hour)
	r.noRun(t) // once
}

func TestCountdownUsesDelay(t *testing.T) {
	r := newRig(t, nil)
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerShutdown, DelayS: 120})
	r.clock.Advance(119 * time.Second)
	r.noRun(t)
	if n := r.c.Notice(); n == nil || n.RemainingS != 1 {
		t.Fatalf("notice %+v", n)
	}
	r.clock.Advance(time.Second)
	if got := r.waitRun(t); got != "shutdown force=false" {
		t.Fatalf("ran %q", got)
	}
}

func TestPostponeTwiceThenNoMore(t *testing.T) {
	r := newRig(t, nil)
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerReboot})
	r.clock.Advance(10 * time.Second)
	if err := r.c.Postpone(); err != nil {
		t.Fatal(err)
	}
	if err := r.c.Postpone(); err != nil {
		t.Fatal(err)
	}
	if err := r.c.Postpone(); !errors.Is(err, ErrNoPostpones) {
		t.Fatalf("third postpone: %v", err)
	}
	// 20 s left + 2 × 5 min.
	r.clock.Advance(20*time.Second + 10*time.Minute - time.Second)
	r.noRun(t)
	r.clock.Advance(time.Second)
	r.waitRun(t)
	equal(t, r.ev.all()[:6], []string{
		"notice:countdown 30 left=2",
		"send:ack c2e1 countdown 30 left=2",
		"notice:countdown 320 left=1",
		"send:ack c2e1 postponed 320 left=1",
		"notice:countdown 620 left=0",
		"send:ack c2e1 postponed 620 left=0",
	})
}

func TestForceSkipsTheCountdown(t *testing.T) {
	r := newRig(t, nil)
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerShutdown, DelayS: 600, Force: true})
	if got := r.waitRun(t); got != "shutdown force=true" {
		t.Fatalf("ran %q", got)
	}
	equal(t, r.ev.all(), []string{
		"notice:executing 0 left=2",
		"send:ack c2e1 executing 0 left=0",
		"os:shutdown force=true",
	})
	if err := r.c.Postpone(); !errors.Is(err, ErrNoOrder) {
		t.Fatalf("postpone while executing: %v", err)
	}
}

func TestCancel(t *testing.T) {
	r := newRig(t, nil)
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerReboot})
	r.handle(t, envX, protocol.PowerCancel{ActionID: actionB}) // another order: ignored
	r.handle(t, envB, protocol.PowerCancel{ActionID: actionA})
	r.clock.Advance(time.Hour)
	r.noRun(t)
	equal(t, r.ev.all(), []string{
		"notice:countdown 30 left=2",
		"send:ack c2e1 countdown 30 left=2",
		"notice:none",
		"send:ack c2e1 cancelled 0 left=0",
	})
	if a := r.ev.payloads()[1].(protocol.PowerAck); a.RefID != envB {
		t.Fatalf("cancelled ack ref_id %s, want the cancel's", a.RefID)
	}
}

func TestNewOrderReplacesTheOld(t *testing.T) {
	r := newRig(t, nil)
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerReboot})
	r.clock.Advance(10 * time.Second)
	r.handle(t, envB, protocol.PowerAction{ActionID: actionB, Action: protocol.PowerShutdown, DelayS: 60})
	r.clock.Advance(59 * time.Second)
	r.noRun(t) // the old one would have run at 30 s
	r.clock.Advance(time.Second)
	if got := r.waitRun(t); got != "shutdown force=false" {
		t.Fatalf("ran %q", got)
	}
	equal(t, r.ev.all()[2:5], []string{
		"send:ack c2e1 cancelled 0 left=0",
		"notice:countdown 60 left=2",
		"send:ack d3f2 countdown 60 left=2",
	})
	if a := r.ev.payloads()[1].(protocol.PowerAck); a.RefID != envA {
		t.Fatalf("cancelled ack ref_id %s, want the old order's", a.RefID)
	}
}

func TestWithoutSession(t *testing.T) {
	r := newRig(t, nil)
	r.c.Session = func() (uint32, bool) { return 0, false }
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerLogoff})
	r.noRun(t)
	r.handle(t, envB, protocol.PowerAction{ActionID: actionB, Action: protocol.PowerReboot, DelayS: 600})
	if got := r.waitRun(t); got != "reboot force=false" {
		t.Fatalf("ran %q", got)
	}
	ev := r.ev.all()
	if ev[0] != "send:error unsupported_capability 0c9a" {
		t.Fatalf("logoff without a session: %q", ev)
	}
}

func TestLogoffUsesTheSession(t *testing.T) {
	r := newRig(t, nil)
	r.c.Session = func() (uint32, bool) { return 3, true }
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerLogoff, Force: true})
	if got := r.waitRun(t); got != "logoff 3 force=true" {
		t.Fatalf("ran %q", got)
	}

	// Without force, the helper's Logoff when there is one.
	r = newRig(t, nil)
	r.c.Session = func() (uint32, bool) { return 3, true }
	r.c.Logoff = func(s uint32) error { r.ev.add(fmt.Sprintf("helper:logoff %d", s)); r.ev.ran <- "helper"; return nil }
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerLogoff})
	r.clock.Advance(30 * time.Second)
	if got := r.waitRun(t); got != "helper" {
		t.Fatalf("ran %q", got)
	}
}

func TestCountdownGoesOnWithoutConnection(t *testing.T) {
	r := newRig(t, nil)
	connected := true
	var mu sync.Mutex
	send := r.send
	r.send = func(ctx context.Context, p protocol.Payload) error {
		mu.Lock()
		defer mu.Unlock()
		if !connected {
			return errors.New("closed")
		}
		return send(ctx, p)
	}
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerShutdown})
	mu.Lock()
	connected = false
	mu.Unlock()
	if err := r.c.Postpone(); err != nil {
		t.Fatal(err)
	}
	r.clock.Advance(30*time.Second + 5*time.Minute)
	if got := r.waitRun(t); got != "shutdown force=false" {
		t.Fatalf("ran %q", got)
	}
}

func TestOSFailure(t *testing.T) {
	r := newRig(t, errors.New("access denied"))
	r.handle(t, envA, protocol.PowerAction{ActionID: actionA, Action: protocol.PowerReboot, Force: true})
	r.waitRun(t)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if n := r.c.Notice(); n != nil && n.State == NoticeFailed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no failed notice; events %q", r.ev.all())
		}
		time.Sleep(time.Millisecond)
	}
	for len(r.ev.payloads()) < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	p := r.ev.payloads()
	if e, ok := p[len(p)-1].(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInternal || e.RefID != envA {
		t.Fatalf("last message %#v", p[len(p)-1])
	}
	// A new order is accepted after a failure.
	r.handle(t, envB, protocol.PowerAction{ActionID: actionB, Action: protocol.PowerReboot})
	if n := r.c.Notice(); n == nil || n.ActionID != actionB || n.State != NoticeCountdown {
		t.Fatalf("notice %+v", n)
	}
}

func TestUnsupported(t *testing.T) {
	c := &Countdown{}
	reply, ok := c.Handle(context.Background(), nil, protocol.Envelope{ID: envA, Payload: protocol.PowerAction{ActionID: actionA, Action: protocol.PowerReboot}})
	if e, isErr := reply.(protocol.ErrorPayload); !ok || !isErr || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply %#v, %v", reply, ok)
	}
	if _, ok := c.Handle(context.Background(), nil, protocol.Envelope{Payload: protocol.MsgShow{}}); ok {
		t.Fatal("msg.show is not power's")
	}
}

func TestTracker(t *testing.T) {
	now := time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC)
	tr := &Tracker{Now: func() time.Time { return now }}
	a := protocol.PowerAction{ActionID: actionA, Action: protocol.PowerReboot}
	tr.Sent("s1", envA, a)
	if e, _ := tr.Get("s1"); e.Status != StatusSent || !e.Pending() {
		t.Fatalf("entry %+v", e)
	}
	if tr.Ack("s1", protocol.PowerAck{ActionID: actionB, Status: protocol.PowerExecuting}) {
		t.Fatal("ack of another order applied")
	}
	tr.Ack("s1", protocol.PowerAck{ActionID: actionA, Status: protocol.PowerPostponed, RemainingS: 320, PostponesLeft: 1})
	e, _ := tr.Get("s1")
	if e.Status != protocol.PowerPostponed || e.PostponesLeft != 1 || e.DeadlineMs != now.Add(320*time.Second).UnixMilli() {
		t.Fatalf("entry %+v", e)
	}
	if tr.Error("s1", envB) {
		t.Fatal("error of another envelope applied")
	}
	tr.Error("s1", envA)
	if e, _ := tr.Get("s1"); e.Status != StatusError || e.Pending() {
		t.Fatalf("entry %+v", e)
	}
	tr.Forget("s1")
	if _, ok := tr.Get("s1"); ok {
		t.Fatal("not forgotten")
	}
}
