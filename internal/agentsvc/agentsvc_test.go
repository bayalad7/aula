package agentsvc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/session"
)

// --- Link ---------------------------------------------------------------------------

func linkPair(t *testing.T, onCallA, onCallB CallFunc, onEventB EventFunc) (*Link, *Link) {
	t.Helper()
	ca, cb := net.Pipe()
	a, b := NewLink(ca, onCallA, nil), NewLink(cb, onCallB, onEventB)
	ctx, cancel := context.WithCancel(context.Background())
	go a.Run(ctx)
	go b.Run(ctx)
	t.Cleanup(func() { cancel(); a.Close(); b.Close() })
	return a, b
}

func TestLinkCallsEventsAndErrors(t *testing.T) {
	events := make(chan string, 4)
	a, b := linkPair(t, func(_ context.Context, name string, data json.RawMessage) (any, error) {
		var s string
		_ = json.Unmarshal(data, &s)
		return name + ":" + s, nil
	}, func(_ context.Context, name string, data json.RawMessage) (any, error) {
		if name == "falla" {
			return nil, errors.New("no se pudo")
		}
		var n int
		_ = json.Unmarshal(data, &n)
		return n * 2, nil
	}, func(name string, data json.RawMessage) { events <- name + "=" + string(data) })

	ctx := t.Context()
	var got int
	if err := a.Call(ctx, "doble", 21, &got); err != nil || got != 42 {
		t.Fatalf("call = %d, %v", got, err)
	}
	if err := a.Call(ctx, "falla", nil, nil); err == nil || err.Error() != "no se pudo" {
		t.Fatalf("error not carried: %v", err)
	}
	// Both directions, many calls at once.
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var s string
			if err := b.Call(ctx, "eco", "x", &s); err != nil || s != "eco:x" {
				t.Errorf("call %d = %q, %v", i, s, err)
			}
		}()
	}
	wg.Wait()
	if err := a.Notify("raise", true); err != nil {
		t.Fatal(err)
	}
	if e := <-events; e != "raise=true" {
		t.Fatalf("event %q", e)
	}
	// A big frame (a screen capture) fits.
	big := strings.Repeat("a", 900<<10)
	if err := b.Call(ctx, "grande", big, nil); err != nil {
		t.Fatalf("900 KiB call: %v", err)
	}
	a.Close()
	if err := b.Call(ctx, "eco", "x", nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("call on a closed link = %v", err)
	}
}

func TestProxiesWithoutHelper(t *testing.T) {
	cur := &Current{}
	if _, err := (Capture{Helper: cur}).Capture(t.Context(), 320, 50); !errors.Is(err, ErrNoHelper) {
		t.Fatalf("capture without helper = %v", err)
	}
	if err := (Clipboard{Helper: cur}).WriteText(t.Context(), "x"); !errors.Is(err, ErrNoHelper) {
		t.Fatalf("clipboard without helper = %v", err)
	}
	if err := (Indicator{Helper: cur}).Show("aviso"); !errors.Is(err, ErrNoHelper) {
		t.Fatalf("indicator without helper = %v", err)
	}
	if err := (Indicator{Helper: cur}).Hide(); err != nil {
		t.Fatalf("hide without helper = %v", err)
	}
	cur.Notify(EventRaise, nil) // dropped, no panic
}

func TestProxiesThroughHelper(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	service, _ := linkPair(t, nil, func(_ context.Context, name string, data json.RawMessage) (any, error) {
		mu.Lock()
		calls = append(calls, name+" "+string(data))
		mu.Unlock()
		if name == CallScreenCapture {
			return platform.Frame{JPEG: []byte{0xff, 0xd8}, Width: 320, Height: 180}, nil
		}
		return nil, nil
	}, nil)
	cur := &Current{}
	cur.Set(service)
	f, err := (Capture{Helper: cur}).Capture(t.Context(), 320, 50)
	if err != nil || f.Width != 320 || len(f.JPEG) != 2 {
		t.Fatalf("capture = %+v, %v", f, err)
	}
	if err := (Indicator{Helper: cur}).Show("Tu pantalla está siendo vista"); err != nil {
		t.Fatal(err)
	}
	if err := (Clipboard{Helper: cur}).WriteText(t.Context(), "print(1)\n"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{`screen.capture {"maxWidth":320,"quality":50}`, `indicator.show "Tu pantalla está siendo vista"`, `clipboard.write "print(1)\n"`}
	if strings.Join(calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls %q", calls)
	}
}

// --- Supervisor ---------------------------------------------------------------------

type fakeProc struct {
	pid  int
	done chan struct{}
	once sync.Once
}

func (p *fakeProc) PID() int { return p.pid }
func (p *fakeProc) Wait(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *fakeProc) Kill() error { p.exit(); return nil }
func (p *fakeProc) exit()       { p.once.Do(func() { close(p.done) }) }
func (p *fakeProc) alive() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

type fakeSessions struct {
	mu       sync.Mutex
	active   *platform.Session
	launched []*fakeProc
	sessions []uint32
	failNext bool
	nextPID  int
}

func (f *fakeSessions) Active() (platform.Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active == nil {
		return platform.Session{}, false, nil
	}
	return *f.active, true, nil
}

func (f *fakeSessions) Launch(s platform.Session, _ string, args []string) (platform.Process, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext {
		f.failNext = false
		return nil, errors.New("CreateProcessAsUser falló")
	}
	if len(args) != 2 || args[0] != HelperFlag {
		return nil, errors.New("bad args")
	}
	f.nextPID++
	p := &fakeProc{pid: 1000 + f.nextPID, done: make(chan struct{})}
	f.launched = append(f.launched, p)
	f.sessions = append(f.sessions, s.ID)
	return p, nil
}

func (f *fakeSessions) set(s *platform.Session) {
	f.mu.Lock()
	f.active = s
	f.mu.Unlock()
}

func (f *fakeSessions) procs() ([]*fakeProc, []uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*fakeProc(nil), f.launched...), append([]uint32(nil), f.sessions...)
}

// memListener hands out the server end of net.Pipe connections; dial picks
// which PID the "peer" has.
type memListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func (l *memListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}
func (l *memListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *memListener) Addr() net.Addr { return &net.UnixAddr{Name: "mem", Net: "mem"} }

type supHarness struct {
	sessions *fakeSessions
	mu       sync.Mutex
	channels map[string]*memListener
	pids     map[net.Conn]int
	served   chan platform.Session
	endServe chan struct{} // closing it ends the channel with the helper
}

func newSupervisor(t *testing.T) (*supHarness, chan struct{}) {
	h := &supHarness{sessions: &fakeSessions{}, channels: map[string]*memListener{}, pids: map[net.Conn]int{},
		served: make(chan platform.Session, 16), endServe: make(chan struct{})}
	sup := &Supervisor{
		Sessions: h.sessions, Exe: `C:\Program Files\Aula\Cliente\aula-cliente.exe`,
		Listen: func(name, sid string) (net.Listener, error) {
			if !strings.HasPrefix(name, "aula-agente-") || sid == "" {
				return nil, errors.New("bad channel")
			}
			l := &memListener{conns: make(chan net.Conn), closed: make(chan struct{})}
			h.mu.Lock()
			h.channels[name] = l
			h.mu.Unlock()
			return l, nil
		},
		PeerPID: func(c net.Conn) (int, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.pids[c], nil
		},
		Serve: func(ctx context.Context, s platform.Session, _ net.Conn) {
			h.served <- s
			select {
			case <-ctx.Done():
			case <-h.endServe:
			}
		},
		Poll:    20 * time.Millisecond,
		Backoff: session.Backoff{Base: 10 * time.Millisecond, Max: 40 * time.Millisecond, Jitter: 0.01},
	}
	changed := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = sup.Run(ctx, changed) }()
	t.Cleanup(func() { cancel(); <-done })
	return h, changed
}

// connect dials the newest channel as the process with the given PID.
func (h *supHarness) connect(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		h.mu.Lock()
		var l *memListener
		for _, c := range h.channels {
			select {
			case <-c.closed:
			default:
				l = c
			}
		}
		h.mu.Unlock()
		if l != nil {
			server, client := net.Pipe()
			h.mu.Lock()
			h.pids[server] = pid
			h.mu.Unlock()
			select {
			case l.conns <- server:
				t.Cleanup(func() { client.Close() })
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("no open channel to connect to")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

var (
	ana  = &platform.Session{ID: 1, User: "ana", SID: "S-1-5-21-1-1001"}
	beto = &platform.Session{ID: 2, User: "beto", SID: "S-1-5-21-1-1002"}
)

func TestSupervisorFollowsTheSession(t *testing.T) {
	h, changed := newSupervisor(t)
	// Logon screen: no helper.
	time.Sleep(60 * time.Millisecond)
	if p, _ := h.sessions.procs(); len(p) != 0 {
		t.Fatalf("helper started without a user: %d", len(p))
	}

	// Ana logs on: the helper starts in her session and only it may connect.
	h.sessions.set(ana)
	changed <- struct{}{}
	waitUntil(t, "helper for ana", func() bool { p, _ := h.sessions.procs(); return len(p) == 1 })
	procs, _ := h.sessions.procs()
	h.connect(t, 4242) // another process of the student
	h.connect(t, procs[0].pid)
	select {
	case s := <-h.served:
		if s.ID != ana.ID {
			t.Fatalf("served session %d", s.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("helper's channel not served")
	}
	select {
	case <-h.served:
		t.Fatal("a process that is not the helper was served")
	default:
	}

	// The student kills the helper: it comes back after the backoff.
	procs[0].exit()
	waitUntil(t, "helper restarted", func() bool { p, _ := h.sessions.procs(); return len(p) == 2 })

	// Fast user switch to Beto: Ana's helper ends, one starts in session 2.
	h.sessions.set(beto)
	changed <- struct{}{}
	waitUntil(t, "helper moved", func() bool { p, _ := h.sessions.procs(); return len(p) == 3 })
	procs, sessions := h.sessions.procs()
	if procs[1].alive() || sessions[2] != beto.ID {
		t.Fatalf("old helper alive=%v, new session %d", procs[1].alive(), sessions[2])
	}

	// Logoff: the helper ends and none is started.
	h.sessions.set(nil)
	changed <- struct{}{}
	waitUntil(t, "helper stopped at logoff", func() bool { return !procs[2].alive() })
	time.Sleep(80 * time.Millisecond)
	if p, _ := h.sessions.procs(); len(p) != 3 {
		t.Fatalf("%d helpers after logoff, want 3", len(p))
	}
}

func TestSupervisorRetriesAFailedLaunch(t *testing.T) {
	h, changed := newSupervisor(t)
	h.sessions.mu.Lock()
	h.sessions.failNext = true
	h.sessions.mu.Unlock()
	h.sessions.set(ana)
	changed <- struct{}{}
	waitUntil(t, "helper started after a failure", func() bool { p, _ := h.sessions.procs(); return len(p) == 1 })
}

func TestSupervisorEndsTheHelperWhenItsChannelCloses(t *testing.T) {
	h, changed := newSupervisor(t)
	h.sessions.set(ana)
	changed <- struct{}{}
	waitUntil(t, "helper", func() bool { p, _ := h.sessions.procs(); return len(p) == 1 })
	procs, _ := h.sessions.procs()
	h.connect(t, procs[0].pid)
	<-h.served
	// The channel ends (the helper hung up or broke): a helper without its
	// channel is useless, so it is ended and started again.
	close(h.endServe)
	waitUntil(t, "helper ended", func() bool { return !procs[0].alive() })
	waitUntil(t, "helper started again", func() bool { p, _ := h.sessions.procs(); return len(p) == 2 })
}
