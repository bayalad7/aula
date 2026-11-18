package session

import (
	"io"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

const testChannel = "1"

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
	r.Connect("3", "3", "", hello("PC09"))

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

// Audit 2026-11-17, I-2: two connected sessions that claim the same computer
// are both marked; a disconnected one, another channel or another computer
// are not.
func TestRegistryMarksDuplicates(t *testing.T) {
	r, _ := newTestRegistry()
	r.Connect("a", testChannel, "10.0.0.7:5000", hello("LAB3-PC07"))
	if s, _ := r.Get("a"); s.DuplicateOf != nil {
		t.Fatalf("alone: DuplicateOf = %v", s.DuplicateOf)
	}
	other := hello("lab3-pc07") // the same, without case
	other.Username = "ALUMNO"
	r.Connect("b", testChannel, "10.0.0.90:6000", other)
	r.Connect("c", "3", "10.0.0.8:5000", hello("LAB3-PC07")) // another channel
	byMAC := hello("PC-CLON")
	byMAC.MACs = []string{"00:1a:2b:3c:4d:5e"}
	r.Connect("d", testChannel, "10.0.0.9:5000", byMAC)
	sameMAC := hello("PC-OTRO")
	sameMAC.MACs = []string{"00:1A:2B:3C:4D:5E"}
	r.Connect("e", testChannel, "10.0.0.10:5000", sameMAC)
	got := map[string][]string{}
	for _, s := range r.List(testChannel) {
		got[s.ID] = s.DuplicateOf
	}
	want := map[string][]string{
		"a": {"10.0.0.90:6000"}, "b": {"10.0.0.7:5000"},
		"d": {"10.0.0.10:5000"}, "e": {"10.0.0.9:5000"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DuplicateOf = %v, want %v", got, want)
	}
	if s, _ := r.Get("c"); s.DuplicateOf != nil {
		t.Fatalf("another channel marked: %v", s.DuplicateOf)
	}
	r.Disconnect("b")
	if s, _ := r.Get("a"); s.DuplicateOf != nil {
		t.Fatalf("after the other left: DuplicateOf = %v", s.DuplicateOf)
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

// The clock of a computer is the median skew of its last five heartbeats
// (docs/protocol.md §5.27.6): one late packet does not move it.
func TestRegistryClockSkewFromHeartbeats(t *testing.T) {
	r, clk := newTestRegistry()
	r.Connect("a", testChannel, "10.0.0.2:5000", hello("PC01"))
	if s, _ := r.Get("a"); s.ClockSamples != 0 || s.ClockSkew != 0 {
		t.Fatalf("no heartbeat yet: %+v", s)
	}
	ahead := 4 * time.Minute
	for i, late := range []time.Duration{0, 10 * time.Millisecond, 5 * time.Second, 20 * time.Millisecond, 0, 0} {
		// The agent stamps its (fast) clock; the packet arrives late.
		ts := clk.now().Add(ahead).UnixMilli()
		clk.add(late)
		if !r.RecordClock("a", ts) {
			t.Fatalf("sample %d refused", i)
		}
		clk.add(5*time.Second - late)
	}
	s, _ := r.Get("a")
	if s.ClockSamples != 5 {
		t.Fatalf("samples = %d, want 5", s.ClockSamples)
	}
	if d := s.ClockSkew - ahead; d < -20*time.Millisecond || d > 0 {
		t.Fatalf("skew = %v, want about %v", s.ClockSkew, ahead)
	}
	if r.RecordClock("nobody", 1) || r.RecordClock("a", 0) {
		t.Fatal("an unknown session or a missing ts must be refused")
	}
}
