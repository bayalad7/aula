package remote

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/screen"
)

// fakeInput is the test double of the injector: it records what would be
// injected. CI never types or clicks on a real desktop.
type fakeInput struct {
	mu     sync.Mutex
	got    []platform.InputEvent
	err    error
	local  time.Time
	locErr error
}

func (f *fakeInput) Inject(events []platform.InputEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.got = append(f.got, events...)
	return nil
}

func (f *fakeInput) LocalInput() (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.local, f.locErr
}

func (f *fakeInput) set(fn func(f *fakeInput)) {
	f.mu.Lock()
	fn(f)
	f.mu.Unlock()
}

func (f *fakeInput) events() []platform.InputEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]platform.InputEvent(nil), f.got...)
}

type fakeVideo struct {
	mu    sync.Mutex
	log   []string
	fails bool
}

func (v *fakeVideo) StartControl(_ context.Context, _ screen.Sender, refID string, p protocol.ScreenThumbStart, text string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.fails {
		return errors.New("no indicator")
	}
	v.log = append(v.log, fmt.Sprintf("start %s %d %s", refID, p.MaxWidth, text))
	return nil
}

func (v *fakeVideo) RetuneControl(p protocol.ScreenThumbStart) {
	v.mu.Lock()
	v.log = append(v.log, fmt.Sprintf("retune %d", p.MaxWidth))
	v.mu.Unlock()
}

func (v *fakeVideo) StopControl() {
	v.mu.Lock()
	v.log = append(v.log, "stop")
	v.mu.Unlock()
}

func (v *fakeVideo) list() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return strings.Join(v.log, "|")
}

// sink collects the acks the agent sends on its own.
type sink struct{ ch chan protocol.Payload }

func newSink() *sink { return &sink{ch: make(chan protocol.Payload, 32)} }

func (s *sink) send(_ context.Context, p protocol.Payload) error { s.ch <- p; return nil }

func (s *sink) next(t *testing.T) protocol.RemoteAck {
	t.Helper()
	select {
	case p := <-s.ch:
		a, ok := p.(protocol.RemoteAck)
		if !ok {
			t.Fatalf("sent %T", p)
		}
		return a
	case <-time.After(3 * time.Second):
		t.Fatal("no ack")
		return protocol.RemoteAck{}
	}
}

type harness struct {
	a      *Agent
	in     *fakeInput
	video  *fakeVideo
	out    *sink
	mu     sync.Mutex
	states []State
}

func newHarness() *harness {
	h := &harness{in: &fakeInput{}, video: &fakeVideo{}, out: newSink()}
	h.a = &Agent{Input: h.in, Video: h.video, PollEvery: 5 * time.Millisecond, PauseFor: 60 * time.Millisecond,
		OnState: func(s State) { h.mu.Lock(); h.states = append(h.states, s); h.mu.Unlock() }}
	return h
}

func (h *harness) stateLog() []State {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]State(nil), h.states...)
}

func (h *harness) handle(t *testing.T, p protocol.Payload) (protocol.Envelope, protocol.Payload) {
	t.Helper()
	env := protocol.New(p)
	reply, ok := h.a.Handle(t.Context(), h.out.send, env)
	if !ok {
		t.Fatalf("%s not handled", p.MessageType())
	}
	return env, reply
}

func start(id string) protocol.RemoteStart {
	p := ViewParams
	p.ControlID = id
	return p
}

func key(code string, down bool) protocol.InputEvent {
	return protocol.InputEvent{T: protocol.InputKey, Code: code, Down: down}
}

var (
	ctlA = protocol.NewID()
	ctlB = protocol.NewID()
)

func TestStartShowsTheNoticeBeforeAcceptingInput(t *testing.T) {
	h := newHarness()
	// Input before any control is dropped.
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 1, Events: []protocol.InputEvent{key("KeyA", true)}})
	env, reply := h.handle(t, start(ctlA))
	ack, ok := reply.(protocol.RemoteAck)
	if !ok || ack.Status != protocol.RemoteActive || ack.RefID != env.ID || ack.ControlID != ctlA {
		t.Fatalf("reply %+v", reply)
	}
	if s := h.stateLog(); len(s) != 1 || s[0] != Active {
		t.Fatalf("states %v", s)
	}
	if v := h.video.list(); v != "start "+env.ID+" 1280 "+IndicatorText {
		t.Fatalf("video %q", v)
	}
	if len(h.in.events()) != 0 {
		t.Fatal("input before the control was injected")
	}
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 1, Events: []protocol.InputEvent{
		{T: protocol.InputMove, X: 0.5, Y: 0.5}, {T: protocol.InputButton, B: protocol.ButtonLeft, Down: true, X: 0.5, Y: 0.5}, key("KeyA", true)}})
	if got := h.in.events(); len(got) != 3 || got[1].Button != platform.ButtonLeft || got[2].Code != "KeyA" {
		t.Fatalf("injected %+v", got)
	}
	// Retuning keeps the control; a stale seq and another control_id are dropped.
	h.handle(t, protocol.RemoteStart{ControlID: ctlA, IntervalMS: 1000, Quality: 50, MaxWidth: 640})
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 1, Events: []protocol.InputEvent{key("KeyB", true)}})
	h.handle(t, protocol.RemoteInput{ControlID: ctlB, Seq: 9, Events: []protocol.InputEvent{key("KeyC", true)}})
	if got := h.in.events(); len(got) != 3 {
		t.Fatalf("dropped input was injected: %+v", got)
	}
	// Stop releases what stays pressed, stops the video and the notice.
	stop, reply := h.handle(t, protocol.RemoteStop{ControlID: ctlA})
	ack = reply.(protocol.RemoteAck)
	if ack.Status != protocol.RemoteEnded || ack.Reason != protocol.RemoteByTeacher || ack.RefID != stop.ID {
		t.Fatalf("stop ack %+v", ack)
	}
	got := h.in.events()[3:]
	if len(got) != 2 || got[0].Code != "KeyA" || got[0].Down || got[1].Button != platform.ButtonLeft || got[1].Down {
		t.Fatalf("release %+v", got)
	}
	if v := h.video.list(); !strings.HasSuffix(v, "retune 640|stop") {
		t.Fatalf("video %q", v)
	}
	if s := h.stateLog(); s[len(s)-1] != Off {
		t.Fatalf("states %v", s)
	}
	// After the stop, nothing is injected.
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 5, Events: []protocol.InputEvent{key("KeyD", true)}})
	if n := len(h.in.events()); n != 5 {
		t.Fatalf("input after the stop: %d events", n)
	}
	// A stop of nothing is ignored.
	if _, reply := h.handle(t, protocol.RemoteStop{ControlID: ctlA}); reply != nil {
		t.Fatalf("stop without control replied %+v", reply)
	}
}

func TestOneControlAtATime(t *testing.T) {
	h := newHarness()
	envA, _ := h.handle(t, start(ctlA))
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 1, Events: []protocol.InputEvent{key("ShiftLeft", true)}})
	envB, reply := h.handle(t, start(ctlB))
	old := h.out.next(t)
	if old.ControlID != ctlA || old.Status != protocol.RemoteEnded || old.Reason != protocol.RemoteReplaced || old.RefID != envA.ID {
		t.Fatalf("replaced ack %+v", old)
	}
	if ack := reply.(protocol.RemoteAck); ack.ControlID != ctlB || ack.RefID != envB.ID {
		t.Fatalf("new ack %+v", ack)
	}
	if got := h.in.events(); len(got) != 2 || got[1].Code != "ShiftLeft" || got[1].Down {
		t.Fatalf("the replaced control's keys were not released: %+v", got)
	}
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 2, Events: []protocol.InputEvent{key("KeyA", true)}})
	if len(h.in.events()) != 2 {
		t.Fatal("input of the replaced control was injected")
	}
	h.a.Stop()
}

func TestTheStudentHasPriority(t *testing.T) {
	h := newHarness()
	h.handle(t, start(ctlA))
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 1, Events: []protocol.InputEvent{key("ControlLeft", true)}})
	h.in.set(func(f *fakeInput) { f.local = time.Now() })
	if a := h.out.next(t); a.Status != protocol.RemotePaused {
		t.Fatalf("ack %+v", a)
	}
	if got := h.in.events(); len(got) != 2 || got[1].Down {
		t.Fatalf("pause must release the keys: %+v", got)
	}
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 2, Events: []protocol.InputEvent{key("KeyV", true)}})
	if len(h.in.events()) != 2 {
		t.Fatal("input injected while paused")
	}
	if a := h.out.next(t); a.Status != protocol.RemoteResumed {
		t.Fatalf("ack %+v", a)
	}
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 3, Events: []protocol.InputEvent{key("KeyV", true)}})
	if len(h.in.events()) != 3 {
		t.Fatal("input not injected after resuming")
	}
	if s := h.stateLog(); len(s) != 3 || s[1] != Paused || s[2] != Active {
		t.Fatalf("states %v", s)
	}
	h.a.Stop()
}

func TestEndsWhenIdleOrTheHelperIsGone(t *testing.T) {
	h := newHarness()
	h.a.IdleAfter = 30 * time.Millisecond
	h.handle(t, start(ctlA))
	if a := h.out.next(t); a.Status != protocol.RemoteEnded || a.Reason != protocol.RemoteIdle {
		t.Fatalf("ack %+v", a)
	}
	h = newHarness()
	h.handle(t, start(ctlA))
	h.in.set(func(f *fakeInput) { f.locErr = agentsvc.ErrClosed })
	if a := h.out.next(t); a.Reason != protocol.RemoteSession {
		t.Fatalf("ack %+v", a)
	}
	// Refused input ends the control only after FailAfter.
	h = newHarness()
	h.a.FailAfter = 20 * time.Millisecond
	h.handle(t, start(ctlA))
	h.in.set(func(f *fakeInput) { f.err = errors.New("access denied") })
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 1, Events: []protocol.InputEvent{key("KeyA", true)}})
	time.Sleep(30 * time.Millisecond)
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 2, Events: []protocol.InputEvent{key("KeyA", true)}})
	if a := h.out.next(t); a.Reason != protocol.RemoteError {
		t.Fatalf("ack %+v", a)
	}
}

func TestRateLimitAndConnectionClose(t *testing.T) {
	h := newHarness()
	now := time.Now()
	h.a.Now = func() time.Time { return now }
	h.handle(t, start(ctlA))
	for i := 1; i <= protocol.MaxRemoteRate+10; i++ {
		h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: i, Events: []protocol.InputEvent{{T: protocol.InputMove, X: 0.1, Y: 0.1}}})
	}
	if n := len(h.in.events()); n != protocol.MaxRemoteRate {
		t.Fatalf("injected %d messages in one second, want %d", n, protocol.MaxRemoteRate)
	}
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 999, Events: []protocol.InputEvent{{T: protocol.InputButton, B: protocol.ButtonRight, Down: true}}})
	now = now.Add(time.Second)
	h.handle(t, protocol.RemoteInput{ControlID: ctlA, Seq: 1000, Events: []protocol.InputEvent{{T: protocol.InputButton, B: protocol.ButtonRight, Down: true}}})
	h.a.Stop()
	got := h.in.events()
	if last := got[len(got)-1]; last.Button != platform.ButtonRight || last.Down {
		t.Fatalf("closing the connection must release the button: %+v", last)
	}
	select {
	case p := <-h.out.ch:
		t.Fatalf("Stop sent %T", p)
	default:
	}
	if v := h.video.list(); !strings.HasSuffix(v, "|stop") {
		t.Fatalf("video %q", v)
	}
}

func TestWithoutInjectorOrIndicator(t *testing.T) {
	a := &Agent{}
	if len(a.Capabilities()) != 0 {
		t.Fatal("must not declare remote without an injector")
	}
	env := protocol.New(start(ctlA))
	reply, _ := a.Handle(t.Context(), newSink().send, env)
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply %+v", reply)
	}
	h := newHarness()
	if c := h.a.Capabilities(); len(c) != 1 || c[0] != protocol.CapRemote {
		t.Fatalf("capabilities %v", c)
	}
	h.video.fails = true
	_, reply = h.handle(t, start(ctlA))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInternal {
		t.Fatalf("reply %+v", reply)
	}
	if s := h.stateLog(); len(s) != 2 || s[1] != Off {
		t.Fatalf("a control without its indicator must not stay: %v", s)
	}
}

// Every key the protocol accepts can be injected.
func TestEveryProtocolKeyIsKnownToThePlatform(t *testing.T) {
	for _, c := range protocol.KeyCodes {
		if !platform.KnownKey(c) {
			t.Errorf("%s has no scancode", c)
		}
	}
}

// --- Desk --------------------------------------------------------------------------

func TestDeskOneAtATime(t *testing.T) {
	var d Desk
	p, c, err := d.Take("s1", "PC01")
	if err != nil || p.ControlID != c.ControlID || c.Status != StatusStarting || p.MaxWidth != ViewParams.MaxWidth {
		t.Fatalf("take = %+v %+v %v", p, c, err)
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	d.Started("env-1")
	if _, busy, err := d.Take("s2", "PC02"); !errors.Is(err, ErrBusy) || busy.Hostname != "PC01" {
		t.Fatalf("second take = %+v, %v", busy, err)
	}
	if _, ok := d.Ack("s2", protocol.RemoteAck{ControlID: c.ControlID, Status: protocol.RemoteActive}); ok {
		t.Fatal("ack of another session applied")
	}
	if c, ok := d.Ack("s1", protocol.RemoteAck{ControlID: c.ControlID, Status: protocol.RemoteActive}); !ok || c.Status != protocol.RemoteActive {
		t.Fatalf("ack = %+v %v", c, ok)
	}
	sid, stop, err := d.Release()
	if err != nil || sid != "s1" || stop.ControlID != c.ControlID {
		t.Fatalf("release = %s %+v %v", sid, stop, err)
	}
	d.Ack("s1", protocol.RemoteAck{ControlID: c.ControlID, Status: protocol.RemoteEnded, Reason: protocol.RemoteByTeacher})
	if _, _, err := d.Release(); !errors.Is(err, ErrNoControl) {
		t.Fatal("release after the end must fail")
	}
	if _, _, err := d.Take("s2", "PC02"); err != nil {
		t.Fatalf("take after the end: %v", err)
	}
	if c, ok := d.Gone("s2"); !ok || c.Status != StatusGone || c.Live() {
		t.Fatalf("gone = %+v %v", c, ok)
	}
	d.Take("s3", "PC03")
	d.Started("env-3")
	if _, ok := d.Error("s3", "other"); ok {
		t.Fatal("error of another message applied")
	}
	if c, ok := d.Error("s3", "env-3"); !ok || c.Status != StatusFailed {
		t.Fatalf("error = %+v %v", c, ok)
	}
}

func TestDeskBatchesAndPacesInput(t *testing.T) {
	var d Desk
	now := time.Now()
	d.Now = func() time.Time { return now }
	_, c, _ := d.Take("s1", "PC01")
	if _, out, _ := d.Input([]protocol.InputEvent{key("KeyA", true)}); len(out) != 0 {
		t.Fatal("input sent before the agent was active")
	}
	d.Ack("s1", protocol.RemoteAck{ControlID: c.ControlID, Status: protocol.RemoteActive})
	move := func(x float64) protocol.InputEvent { return protocol.InputEvent{T: protocol.InputMove, X: x} }
	_, out, err := d.Input([]protocol.InputEvent{move(0.1), move(0.2), move(0.3), key("KeyA", true), move(0.4), move(0.5)})
	if err != nil || len(out) != 1 {
		t.Fatalf("input = %+v %v", out, err)
	}
	if ev := out[0].Events; len(ev) != 3 || ev[0].X != 0.3 || ev[1].Code != "KeyA" || ev[2].X != 0.5 || out[0].Seq != 1 {
		t.Fatalf("coalesced %+v", out[0])
	}
	// More than the pace allows waits; nothing typed is lost.
	batch := make([]protocol.InputEvent, protocol.MaxRemoteEvents)
	for i := range batch {
		batch[i] = key("KeyB", i%2 == 0)
	}
	total, events := 0, 0
	for range protocol.MaxRemoteRate + 5 {
		_, out, _ = d.Input(batch)
		total += len(out)
		for _, m := range out {
			events += len(m.Events)
			if err := m.Validate(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if total != protocol.MaxRemoteRate-1 {
		t.Fatalf("sent %d messages in one second, want %d", total, protocol.MaxRemoteRate-1)
	}
	now = now.Add(time.Second)
	_, out, _ = d.Input(nil)
	for _, m := range out {
		events += len(m.Events)
	}
	if want := (protocol.MaxRemoteRate + 5) * protocol.MaxRemoteEvents; events != want {
		t.Fatalf("%d keys left, want all %d (keys are never dropped)", events, want)
	}
	// Closing the view stops the input and slows the video.
	sid, p, err := d.View(false)
	if err != nil || sid != "s1" || p.IntervalMS != HiddenParams.IntervalMS || p.ControlID != c.ControlID {
		t.Fatalf("view = %+v %v", p, err)
	}
	if _, _, err := d.Input([]protocol.InputEvent{key("KeyA", true)}); !errors.Is(err, ErrNoControl) {
		t.Fatal("input without the view must fail")
	}
	// Paused: nothing leaves.
	d.View(true)
	d.Ack("s1", protocol.RemoteAck{ControlID: c.ControlID, Status: protocol.RemotePaused})
	if _, out, _ := d.Input([]protocol.InputEvent{key("KeyA", true)}); len(out) != 0 {
		t.Fatal("input sent while paused")
	}
	if c, _ := d.Ack("s1", protocol.RemoteAck{ControlID: c.ControlID, Status: protocol.RemoteResumed}); c.Status != protocol.RemoteActive {
		t.Fatalf("resumed = %+v", c)
	}
}

func TestTrimKeepsKeys(t *testing.T) {
	in := []protocol.InputEvent{{T: protocol.InputMove}, key("KeyA", true), {T: protocol.InputMove}, key("KeyA", false)}
	if out := trim(in, 2); len(out) != 2 || out[0].Code != "KeyA" || out[1].Code != "KeyA" {
		t.Fatalf("trim %+v", out)
	}
}
