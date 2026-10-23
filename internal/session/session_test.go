package session

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

const testChannel = "AULA_101"

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func hello(host string) protocol.HelloInfo {
	return protocol.HelloInfo{
		Hostname:     host,
		Username:     "alumno",
		OS:           protocol.OSWindows,
		OSVersion:    "10.0.19045",
		AgentVersion: "0.1.0",
		Capabilities: []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow},
	}
}

// --- Registry ---------------------------------------------------------------

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newTestRegistry() (*Registry, *fakeClock) {
	clk := &fakeClock{t: time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)}
	r := NewRegistry(15 * time.Second)
	r.now = clk.now
	return r, clk
}

func TestRegistrySweepAfterTimeout(t *testing.T) {
	r, clk := newTestRegistry()
	r.Connect("a", testChannel, "10.0.0.2:5000", hello("PC01"))
	r.Connect("b", testChannel, "10.0.0.3:5000", hello("PC02"))

	clk.add(10 * time.Second)
	r.Touch("a")
	clk.add(6 * time.Second) // b silent for 16 s, a for 6 s
	if stale := r.Sweep(); len(stale) != 1 || stale[0] != "b" {
		t.Fatalf("Sweep = %v, want [b]", stale)
	}
	if s, _ := r.Get("b"); s.State != StateDisconnected {
		t.Fatalf("b state = %s", s.State)
	}
	if s, _ := r.Get("a"); s.State != StateConnected {
		t.Fatalf("a state = %s", s.State)
	}
	if r.Touch("b") {
		t.Fatal("Touch revived a disconnected session")
	}
	if len(r.Sweep()) != 0 {
		t.Fatal("second Sweep reported sessions again")
	}
}

func TestRegistryListByChannelAndReconnect(t *testing.T) {
	r, _ := newTestRegistry()
	r.Connect("1", testChannel, "", hello("PC02"))
	r.Connect("2", testChannel, "", hello("PC01"))
	r.Connect("3", "CURSO_PYTHON_A", "", hello("PC09"))

	list := r.List(testChannel)
	if len(list) != 2 || list[0].Hostname != "PC01" || list[1].Hostname != "PC02" {
		t.Fatalf("List = %+v", list)
	}
	if !list[0].Can(protocol.CapCodeView) || list[0].Can(protocol.CapScreenFreeze) {
		t.Fatal("capabilities not kept")
	}

	// PC02 drops and comes back: one entry, the new connection.
	if !r.Disconnect("1") || r.Disconnect("1") {
		t.Fatal("Disconnect must report the change once")
	}
	r.Connect("4", testChannel, "", hello("PC02"))
	list = r.List(testChannel)
	if len(list) != 2 || list[1].ID != "4" || list[1].State != StateConnected {
		t.Fatalf("after reconnect List = %+v", list)
	}
}

func TestRegistrySnapshotsAreCopies(t *testing.T) {
	r, _ := newTestRegistry()
	s := r.Connect("a", testChannel, "", hello("PC01"))
	s.Capabilities[0] = "tampered"
	if got, _ := r.Get("a"); got.Capabilities[0] != protocol.CapCodeView {
		t.Fatal("snapshot shares memory with the registry")
	}
}

func TestRegistryConcurrentUse(t *testing.T) {
	r, _ := newTestRegistry()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := string(rune('a' + i))
			r.Connect(id, testChannel, "", hello("PC"+id))
			for range 100 {
				r.Touch(id)
				r.List(testChannel)
				r.Sweep()
			}
			r.Disconnect(id)
		}()
	}
	wg.Wait()
	if n := len(r.List(testChannel)); n != 8 {
		t.Fatalf("got %d sessions", n)
	}
}

// --- Backoff ----------------------------------------------------------------

func TestBackoffSequence(t *testing.T) {
	b := Backoff{Base: time.Second, Max: 30 * time.Second}
	want := []time.Duration{1, 2, 4, 8, 16, 30, 30}
	for n, w := range want {
		if got := b.Delay(n); got != w*time.Second {
			t.Errorf("Delay(%d) = %v, want %v", n, got, w*time.Second)
		}
	}
}

func TestBackoffJitter(t *testing.T) {
	low := Backoff{Base: time.Second, Max: 30 * time.Second, Jitter: 0.2, rand: func() float64 { return 0 }}
	high := low
	high.rand = func() float64 { return 0.999999 }
	if got := low.Delay(3); got != 6400*time.Millisecond {
		t.Errorf("min jitter Delay(3) = %v, want 6.4s", got)
	}
	if got := high.Delay(3); got < 9590*time.Millisecond || got > 9600*time.Millisecond {
		t.Errorf("max jitter Delay(3) = %v, want ~9.6s", got)
	}
	var zero Backoff
	for range 100 {
		if d := zero.Delay(0); d < 800*time.Millisecond || d > 1200*time.Millisecond {
			t.Fatalf("zero-value Delay(0) = %v, want 1s ±20%%", d)
		}
	}
}

// --- Integration: Server + Console + Registry + Agents on loopback -----------

type teacher struct {
	reg  *Registry
	addr *net.TCPAddr
	stop func()
}

func connOpts() transport.Options {
	return transport.Options{HeartbeatInterval: 50 * time.Millisecond, IdleTimeout: 600 * time.Millisecond, Logger: quiet}
}

// startTeacher wires transport.Server, Console and Registry on 127.0.0.1.
func startTeacher(t *testing.T, addr string) *teacher {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	reg := NewRegistry(600 * time.Millisecond)
	srv := transport.NewServer(connOpts())
	console := &Console{Channel: testChannel, Registry: reg, HelloTimeout: 500 * time.Millisecond, Logger: quiet}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = srv.Serve(ctx, ln) }()
	go func() { defer wg.Done(); _ = console.Run(ctx, srv.Events()) }()
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); wg.Wait() }) }
	t.Cleanup(stop)
	return &teacher{reg: reg, addr: ln.Addr().(*net.TCPAddr), stop: stop}
}

func (tc *teacher) source() StaticSource {
	return StaticSource{{
		Instance: "test", Channel: testChannel, Name: "Prueba",
		Addrs: []net.IP{tc.addr.IP}, Port: tc.addr.Port,
		CertFingerprint: discovery.PlaceholderFingerprint,
	}}
}

func startAgent(t *testing.T, host string, src ConsoleSource) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	a := &Agent{
		Hello:   hello(host),
		Source:  src,
		Backoff: Backoff{Base: 20 * time.Millisecond, Max: 100 * time.Millisecond},
		Conn:    connOpts(),
		Logger:  quiet,
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = a.Run(ctx) }()
	stop := func() { cancel(); <-done }
	t.Cleanup(stop)
	return stop
}

// waitFor polls the registry until cond holds.
func waitFor(t *testing.T, reg *Registry, what string, cond func([]Session) bool) []Session {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if list := reg.List(testChannel); cond(list) {
			return list
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s; registry: %+v", what, reg.List(testChannel))
	return nil
}

func states(list []Session) map[string]State {
	m := map[string]State{}
	for _, s := range list {
		m[s.Hostname] = s.State
	}
	return m
}

func TestTwoAgentsAppearAndOneLeaves(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0")
	startAgent(t, "PC01", tc.source())
	stop2 := startAgent(t, "PC02", tc.source())

	waitFor(t, tc.reg, "two connected agents", func(l []Session) bool {
		s := states(l)
		return len(l) == 2 && s["PC01"] == StateConnected && s["PC02"] == StateConnected
	})

	stop2()
	waitFor(t, tc.reg, "PC02 disconnected", func(l []Session) bool {
		s := states(l)
		return s["PC01"] == StateConnected && s["PC02"] == StateDisconnected
	})

	// PC01 keeps sending heartbeats: still connected well past the timeout.
	time.Sleep(3 * 600 * time.Millisecond)
	if s := states(tc.reg.List(testChannel)); s["PC01"] != StateConnected {
		t.Fatalf("PC01 dropped despite heartbeats: %v", s)
	}
}

func TestFrozenAgentMarkedDisconnectedAfterTimeout(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0")
	// A client that sends hello and then nothing: no heartbeats, socket open.
	cl, err := transport.Dial(context.Background(), "ws://"+tc.addr.String()+transport.Path,
		transport.Options{IdleTimeout: time.Minute, Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close(transport.CloseNormal, "")
	go func() {
		for range cl.Incoming() {
		}
	}()
	if _, err := cl.Send(context.Background(), hello("PC03")); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	waitFor(t, tc.reg, "PC03 connected", func(l []Session) bool { return states(l)["PC03"] == StateConnected })
	waitFor(t, tc.reg, "PC03 disconnected", func(l []Session) bool { return states(l)["PC03"] == StateDisconnected })
	if d := time.Since(start); d < 600*time.Millisecond {
		t.Fatalf("marked disconnected after %v, before the 600ms timeout", d)
	}
}

func TestFirstMessageMustBeHello(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0")
	cl, err := transport.Dial(context.Background(), "ws://"+tc.addr.String()+transport.Path, connOpts())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cl.Send(context.Background(), protocol.Heartbeat{}); err != nil {
		t.Fatal(err)
	}
	var gotErr bool
	for env := range cl.Incoming() {
		if e, ok := env.Payload.(protocol.ErrorPayload); ok && e.Code == protocol.CodeInvalidMessage {
			gotErr = true
		}
	}
	<-cl.Done()
	if !gotErr {
		t.Fatal("no invalid_message error before close")
	}
	if n := len(tc.reg.List(testChannel)); n != 0 {
		t.Fatalf("registry has %d sessions", n)
	}
}

func TestHandshakeTimeout(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0")
	cl, err := transport.Dial(context.Background(), "ws://"+tc.addr.String()+transport.Path, connOpts())
	if err != nil {
		t.Fatal(err)
	}
	cl.StartHeartbeat() // alive, but never says hello
	go func() {
		for range cl.Incoming() {
		}
	}()
	select {
	case <-cl.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("connection without hello not closed")
	}
}

func TestAgentReconnectsWhenConsoleComesBack(t *testing.T) {
	// Reserve a port, then start the agent before the console exists.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	tcpAddr := ln.Addr().(*net.TCPAddr)
	src := StaticSource{{Instance: "test", Channel: testChannel, Addrs: []net.IP{tcpAddr.IP}, Port: tcpAddr.Port,
		CertFingerprint: discovery.PlaceholderFingerprint}}

	var mu sync.Mutex
	var seen []AgentState
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &Agent{
		Hello: hello("PC04"), Source: src, Conn: connOpts(), Logger: quiet,
		Backoff: Backoff{Base: 20 * time.Millisecond, Max: 100 * time.Millisecond},
		OnState: func(s AgentState, _ discovery.Console) { mu.Lock(); seen = append(seen, s); mu.Unlock() },
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = a.Run(ctx) }()
	time.Sleep(300 * time.Millisecond) // a few failed attempts

	tc := startTeacher(t, addr)
	waitFor(t, tc.reg, "PC04 connected", func(l []Session) bool { return states(l)["PC04"] == StateConnected })

	tc.stop() // console goes away…
	tc2 := startTeacher(t, addr)
	waitFor(t, tc2.reg, "PC04 back", func(l []Session) bool { return states(l)["PC04"] == StateConnected })

	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	var connected int
	for _, s := range seen {
		if s == AgentConnected {
			connected++
		}
	}
	if connected < 2 {
		t.Fatalf("agent states %v, want two connections", seen)
	}
}
