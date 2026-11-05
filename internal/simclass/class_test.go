package simclass

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/participation"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/snapshot"
	"github.com/bayalad7/aula/internal/transport"
)

const testChannel = "7"

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// console is a real console (TLS, handshake, registry) on loopback that
// records what the students send back.
type console struct {
	*session.Console
	reg  *session.Registry
	info Console

	mu     sync.Mutex
	acks   map[string]bool // ref_id of code.ack and msg.ack
	frames map[string]int  // session ID → frames received
	snaps  *snapshot.Store // screenshots asked to the students
	hands  []protocol.HandRaise
	msgs   []protocol.Envelope // msg.student
}

func startConsole(t *testing.T) *console {
	t.Helper()
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
	c := &console{reg: session.NewRegistry(0), acks: map[string]bool{}, frames: map[string]int{}, snaps: &snapshot.Store{}}
	c.Console = &session.Console{
		Channel: testChannel, Registry: c.reg, Identity: id, CertSHA256: cert.SHA256, Logger: quiet,
		OnMessage: func(_ context.Context, conn *transport.Conn, env protocol.Envelope) {
			c.mu.Lock()
			defer c.mu.Unlock()
			switch p := env.Payload.(type) {
			case protocol.CodeAck:
				c.acks[p.RefID] = true
			case protocol.MsgAck:
				c.acks[p.RefID] = true
			case protocol.ScreenThumbFrame:
				c.frames[conn.ID()]++
			case protocol.ScreenSnapshotResponse:
				c.snaps.Response(conn.ID(), p)
			case protocol.HandRaise:
				c.hands = append(c.hands, p)
			case protocol.MsgStudent:
				c.msgs = append(c.msgs, env)
			}
		},
	}
	srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(cert), Logger: quiet})
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = srv.Serve(ctx, ln) }()
	go func() { defer wg.Done(); _ = c.Run(ctx, srv.Events()) }()
	t.Cleanup(func() { cancel(); wg.Wait() })
	c.info = Console{
		Addr: ln.Addr().(*net.TCPAddr), Channel: testChannel, Name: "Prueba",
		PublicKey: id.Public(), CertFingerprint: cert.Fingerprint(),
	}
	return c
}

func (c *console) acked(ref string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.acks[ref]
}

func (c *console) framesOf(id string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.frames[id]
}

func (c *console) connected() []session.Session {
	var out []session.Session
	for _, s := range c.reg.List(testChannel) {
		if s.State == session.StateConnected {
			out = append(out, s)
		}
	}
	return out
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func newClass(t *testing.T, c *console) *Class {
	t.Helper()
	cl, err := New(c.info, "0.6.0-dev", nil, quiet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cl.Close)
	return cl
}

func sessionOf(t *testing.T, c *console, name string) session.Session {
	t.Helper()
	for _, s := range c.connected() {
		if s.Hostname == name {
			return s
		}
	}
	t.Fatalf("%s is not connected", name)
	return session.Session{}
}

func TestStudentsJoinShowCodeAndAcknowledge(t *testing.T) {
	c := startConsole(t)
	cl := newClass(t, c)
	a, err := cl.Add(protocol.OSWindows, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cl.Add(protocol.OSLinux, []protocol.Capability{protocol.CapCodeView}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "two simulated sessions", func() bool { return len(c.connected()) == 2 })

	s := sessionOf(t, c, a.Name)
	if s.AgentVersion != "0.6.0-dev+sim" || s.Username != "Alumno de prueba" || s.OSVersion != "simulado" {
		t.Errorf("hello = %+v", s)
	}
	sent, failed := c.Broadcast(context.Background(), protocol.CodeShare{
		SnippetID: protocol.NewID(), Exercise: "1", Title: "Hola", Lang: "python", Content: "print(1)\nprint(2)\n", Mode: protocol.ModeReplace,
	})
	if len(sent) != 2 || len(failed) != 0 {
		t.Fatalf("sent %d, failed %v", len(sent), failed)
	}
	for _, ref := range sent {
		eventually(t, "code.ack", func() bool { return c.acked(ref) })
	}
	got := cl.Students()[0]
	if !got.Connected || len(got.Snippets) != 1 || got.Snippets[0].Lines != 2 || got.Snippets[0].Title != "Hola" {
		t.Errorf("student = %+v", got)
	}
}

func TestFaultNoAckShowsButDoesNotAcknowledge(t *testing.T) {
	c := startConsole(t)
	cl := newClass(t, c)
	a, _ := cl.Add(protocol.OSWindows, nil)
	eventually(t, "connected", func() bool { return len(c.connected()) == 1 })
	if err := cl.SetFault(a.ID, FaultNoAck, 0); err != nil {
		t.Fatal(err)
	}
	ref, err := c.Send(context.Background(), sessionOf(t, c, a.Name).ID, protocol.MsgShow{Text: "Hola", Level: protocol.LevelInfo})
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "notice shown", func() bool { return cl.Students()[0].Notices == 1 })
	time.Sleep(300 * time.Millisecond)
	if c.acked(ref) {
		t.Error("a student with the noack fault acknowledged")
	}
}

func TestFaultSlowAcknowledgesLate(t *testing.T) {
	c := startConsole(t)
	cl := newClass(t, c)
	a, _ := cl.Add(protocol.OSWindows, nil)
	eventually(t, "connected", func() bool { return len(c.connected()) == 1 })
	if err := cl.SetFault(a.ID, FaultSlow, 800*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d := cl.Students()[0].DelayMs; d != 800 {
		t.Errorf("delay = %d ms", d)
	}
	start := time.Now()
	ref, _ := c.Send(context.Background(), sessionOf(t, c, a.Name).ID, protocol.CodeClear{})
	eventually(t, "late ack", func() bool { return c.acked(ref) })
	if el := time.Since(start); el < 700*time.Millisecond {
		t.Errorf("ack after %v, want about 800 ms", el)
	}
	if len(c.connected()) != 1 {
		t.Error("a slow student must stay connected")
	}
}

func TestFaultDisconnectedAndBack(t *testing.T) {
	c := startConsole(t)
	cl := newClass(t, c)
	a, _ := cl.Add(protocol.OSWindows, nil)
	eventually(t, "connected", func() bool { return len(c.connected()) == 1 })
	if err := cl.SetFault(a.ID, FaultDisconnected, 0); err != nil {
		t.Fatal(err)
	}
	eventually(t, "disconnected", func() bool { return len(c.connected()) == 0 && !cl.Students()[0].Connected })
	if err := cl.SetFault(a.ID, FaultNone, 0); err != nil {
		t.Fatal(err)
	}
	eventually(t, "back", func() bool { return len(c.connected()) == 1 && cl.Students()[0].Connected })
}

func TestScreenThumbnailsShowTheViewedNotice(t *testing.T) {
	c := startConsole(t)
	cl := newClass(t, c)
	a, _ := cl.Add(protocol.OSWindows, nil)
	eventually(t, "connected", func() bool { return len(c.connected()) == 1 })
	s := sessionOf(t, c, a.Name)
	if !s.Can(protocol.CapScreenThumb) {
		t.Fatal("screen.thumb not declared")
	}
	if _, err := c.Send(context.Background(), s.ID, protocol.ScreenThumbStart{IntervalMS: 500, Quality: 50, MaxWidth: 320}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "frames", func() bool { return c.framesOf(s.ID) >= 1 })
	if !cl.Students()[0].Viewed {
		t.Error("the student does not show that its screen is viewed")
	}
	if _, err := c.Send(context.Background(), s.ID, protocol.ScreenThumbStop{}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "notice removed", func() bool { return !cl.Students()[0].Viewed })
}

// A simulated student answers a screenshot with its drawn screen, which says
// it is simulated, and counts it.
func TestScreenshotOfASimulatedStudent(t *testing.T) {
	c := startConsole(t)
	cl := newClass(t, c)
	a, _ := cl.Add(protocol.OSWindows, nil)
	eventually(t, "connected", func() bool { return len(c.connected()) == 1 })
	s := sessionOf(t, c, a.Name)
	if !s.Can(protocol.CapScreenSnapshot) {
		t.Fatal("screen.snapshot not declared")
	}
	req, snap, err := c.snaps.Request(s.ID, s.Hostname, s.Username, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Send(context.Background(), s.ID, req); err != nil {
		t.Fatal(err)
	}
	eventually(t, "screenshot", func() bool { got, _ := c.snaps.Get(snap.ID); return got.Status == snapshot.StatusReady })
	got, _ := c.snaps.Get(snap.ID)
	if got.Hostname != a.Name || got.Width != canvasW*scale || got.Height != canvasH*scale {
		t.Fatalf("snapshot = %+v", got)
	}
	eventually(t, "counted", func() bool { return cl.Students()[0].Snapshots == 1 })
}

func TestRemoveAndClose(t *testing.T) {
	c := startConsole(t)
	cl := newClass(t, c)
	a, _ := cl.Add(protocol.OSWindows, nil)
	if _, err := cl.Add(protocol.OSDarwin, nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "two connected", func() bool { return len(c.connected()) == 2 })
	if err := cl.Remove(a.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "one left", func() bool { return len(c.connected()) == 1 })
	if err := cl.Remove(a.ID); !errors.Is(err, ErrUnknownStudent) {
		t.Errorf("remove twice: %v", err)
	}
	cl.Close()
	eventually(t, "none left", func() bool { return len(c.connected()) == 0 })
	if _, err := cl.Add(protocol.OSWindows, nil); !errors.Is(err, ErrClosed) {
		t.Errorf("add after close: %v", err)
	}
}

func TestOnlySafeCapabilitiesAndFaults(t *testing.T) {
	c := startConsole(t)
	cl := newClass(t, c)
	for _, k := range []protocol.Capability{protocol.CapPower, protocol.CapAppBlock, protocol.CapWebFilter, protocol.CapScreenBroadcast, protocol.CapRemote} {
		if _, err := cl.Add(protocol.OSWindows, []protocol.Capability{k}); !errors.Is(err, ErrBadCapability) {
			t.Errorf("capability %s: %v", k, err)
		}
	}
	a, _ := cl.Add(protocol.OSWindows, nil)
	if err := cl.SetFault(a.ID, Fault("crash"), 0); !errors.Is(err, ErrBadFault) {
		t.Errorf("unknown fault: %v", err)
	}
	if _, err := New(Console{}, "0.6.0", nil, nil); err == nil {
		t.Error("New accepted an empty console")
	}
}

// A simulated student raises its hand and writes when the teacher makes it;
// the console's ack and «Bajar la mano» reach it like a real one.
func TestSimulatedStudentRaisesItsHandAndWrites(t *testing.T) {
	c := startConsole(t)
	cl := newClass(t, c)
	a, _ := cl.Add(protocol.OSWindows, nil)
	only, _ := cl.Add(protocol.OSWindows, []protocol.Capability{protocol.CapCodeView})
	eventually(t, "connected", func() bool { return len(c.connected()) == 2 })
	if err := cl.Raise(only.ID, true); !errors.Is(err, ErrBadCapability) {
		t.Errorf("raise without the capability: %v", err)
	}
	if err := cl.Raise(a.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := cl.Write(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	var hand protocol.HandRaise
	var msg protocol.Envelope
	eventually(t, "hand and message", func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		if len(c.hands) == 1 && len(c.msgs) == 1 {
			hand, msg = c.hands[0], c.msgs[0]
			return true
		}
		return false
	})
	if text := msg.Payload.(protocol.MsgStudent).Text; text != SampleMessages[0] {
		t.Errorf("sample text %q", text)
	}
	if got := cl.Students()[0]; got.Hand != participation.HandUp || got.Sent != 1 {
		t.Errorf("student = %+v", got)
	}
	s := sessionOf(t, c, a.Name)
	if _, err := c.Send(context.Background(), s.ID, protocol.HandLower{HandID: hand.HandID, By: protocol.HandByTeacher}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "lowered", func() bool { return cl.Students()[0].Hand == participation.HandLoweredByTeacher })
	if err := cl.Write("nobody", "x"); !errors.Is(err, ErrUnknownStudent) {
		t.Errorf("unknown student: %v", err)
	}
}
