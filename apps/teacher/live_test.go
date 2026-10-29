package main

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

// TestLiveClassTwoAgentsAck runs the real stack: the App opens a class (TLS,
// console, registry), two authenticated agents join, the teacher emits and
// both acks are counted, as in the stage 07 acceptance test.
func TestLiveClassTwoAgentsAck(t *testing.T) {
	rec := &recorder{}
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	app.port = 0 // any free port
	app.emit = rec.emit
	view, err := app.StartClass("7", "Prueba")
	if err != nil {
		t.Fatal(err)
	}
	defer app.EndClass()
	pub, err := security.ParsePublicKey(view.Class.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	boards := map[string]*codedist.Board{}
	for _, host := range []string{"PC01", "PC02"} {
		board := codedist.NewBoard(0)
		boards[host] = board
		recv := &codedist.Receiver{Board: board}
		ctx, cancel := context.WithCancel(context.Background())
		agent := &session.Agent{
			Channel: "7", ConsolePublicKey: pub,
			Hello: protocol.HelloInfo{Hostname: host, OS: protocol.OSWindows, AgentVersion: "0.1.0",
				Capabilities: []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow}},
			Source:  session.StaticSource{{Instance: "x", Channel: "7", Addrs: []net.IP{net.IPv4(127, 0, 0, 1)}, Port: view.Class.Port}},
			Backoff: session.Backoff{Base: 20 * time.Millisecond, Max: 100 * time.Millisecond},
			Logger:  quiet,
			OnMessage: func(ctx context.Context, cl *transport.Client, env protocol.Envelope) {
				if reply := recv.Handle(env); reply != nil {
					_, _ = cl.Send(ctx, reply)
				}
			},
		}
		done := make(chan struct{})
		go func() { defer close(done); _ = agent.Run(ctx) }()
		t.Cleanup(func() { cancel(); <-done })
	}

	waitUntil(t, "two students connected", func() bool { return app.View().Students.CanCode == 2 })
	if s, ok := rec.last(EventStudents).(Students); !ok || s.Connected != 2 {
		t.Fatalf("students event = %#v", rec.last(EventStudents))
	}

	d, err := app.Emit(EmitRequest{Title: "Factorial", Lang: "python", Content: pySnippet, Mode: "replace"})
	if err != nil || d.Sent != 2 {
		t.Fatalf("Emit = %+v, %v", d, err)
	}
	waitUntil(t, "two code.acks", func() bool {
		for _, x := range app.View().Deliveries {
			if x.ID == d.ID {
				return x.Acked == 2
			}
		}
		return false
	})
	for host, b := range boards {
		if got := b.Snippets(); len(got) != 1 || got[0].Content != pySnippet || got[0].Exercise != "1" {
			t.Fatalf("%s board = %+v", host, got)
		}
	}

	n, err := app.SendNotice("Guarden su trabajo", "warning")
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "two msg.acks", func() bool { return app.View().Deliveries[0].ID == n.ID && app.View().Deliveries[0].Acked == 2 })
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// startAgents launches n authenticated agents against the class on port,
// each answering like the real app. It returns how many code.share each got.
func startAgents(t *testing.T, n, port int, pub ed25519.PublicKey) []*codedist.Board {
	t.Helper()
	return startAgentsOn(t, n, "7", "SIM", session.StaticSource{localConsole(port, "7")}, pub)
}

// localConsole describes a console on loopback for a StaticSource.
func localConsole(port int, channel string) discovery.Console {
	return discovery.Console{Instance: "c" + strconv.Itoa(port), Channel: channel, Name: "Canal " + channel,
		Addrs: []net.IP{net.IPv4(127, 0, 0, 1)}, Port: port}
}

// startAgentsOn launches n agents of channel that know the given consoles.
func startAgentsOn(t *testing.T, n int, channel, prefix string, src session.StaticSource, pub ed25519.PublicKey) []*codedist.Board {
	t.Helper()
	boards := make([]*codedist.Board, n)
	for i := range n {
		board := codedist.NewBoard(0)
		boards[i] = board
		recv := &codedist.Receiver{Board: board}
		ctx, cancel := context.WithCancel(context.Background())
		agent := &session.Agent{
			Channel: channel, ConsolePublicKey: pub,
			Hello: protocol.HelloInfo{Hostname: fmt.Sprintf("%s-%02d", prefix, i+1), OS: protocol.OSWindows, AgentVersion: "0.1.0",
				Capabilities: []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow}},
			Source:  src,
			Backoff: session.Backoff{Base: 50 * time.Millisecond, Max: 300 * time.Millisecond},
			Logger:  quiet,
			OnMessage: func(ctx context.Context, cl *transport.Client, env protocol.Envelope) {
				if reply := recv.Handle(env); reply != nil {
					_, _ = cl.Send(ctx, reply)
				}
			},
		}
		done := make(chan struct{})
		go func() { defer close(done); _ = agent.Run(ctx) }()
		t.Cleanup(func() { cancel(); <-done })
	}
	return boards
}

// Hito 1 robustness: 12 simultaneous agents all acknowledge, and after the
// console restarts they come back on their own, with nothing reinstalled.
func TestTwelveAgentsAndConsoleRestart(t *testing.T) {
	const n = 12
	dir := t.TempDir()
	app := NewApp(dir, nil, nil, &fakeHotkeys{}, quiet)
	app.port = 0
	view, err := app.StartClass("7", "Prueba de carga")
	if err != nil {
		t.Fatal(err)
	}
	port := view.Class.Port
	pub, _ := security.ParsePublicKey(view.Class.PublicKey)
	boards := startAgents(t, n, port, pub)

	waitUntil(t, "12 students", func() bool { return app.View().Students.CanCode == n })
	emitAndWaitAll := func(title string) {
		t.Helper()
		d, err := app.Emit(EmitRequest{Title: title, Lang: "python", Content: pySnippet, Mode: "replace"})
		if err != nil || d.Sent != n {
			t.Fatalf("Emit %s = %+v, %v", title, d, err)
		}
		waitUntil(t, title+" acked by all", func() bool {
			for _, x := range app.View().Deliveries {
				if x.ID == d.ID {
					return x.Acked == n
				}
			}
			return false
		})
	}
	emitAndWaitAll("antes del reinicio")

	// Restart the console: same directory (same key and certificate), same port.
	app.EndClass()
	app.port = port
	if _, err := app.StartClass("7", "Prueba de carga"); err != nil {
		t.Fatal(err)
	}
	defer app.EndClass()
	waitUntil(t, "12 students back", func() bool { return app.View().Students.CanCode == n })
	emitAndWaitAll("después del reinicio")

	for i, b := range boards {
		if got := b.Snippets(); len(got) != 1 || got[0].Title != "después del reinicio" {
			t.Fatalf("agent %d board = %+v", i+1, got)
		}
	}
}

// Two rooms on the same network, as in a LanSchool setup: the main room on
// channel 1 and the alternate room on channel 2, both consoles signing with
// the same site key. Students of channel 1 are told about both consoles (the
// channel-2 one first) and must still end up only in the channel-1 console:
// the proof is bound to the channel, so the other console's does not verify.
func TestTwoRoomsShareTheSiteKey(t *testing.T) {
	site := t.TempDir()
	t.Cleanup(func() { unprotect(site) })
	if _, err := security.CreateSiteKey(site, false); err != nil {
		t.Fatal(err)
	}
	open := func(channel, name string) (*App, View) {
		t.Helper()
		app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
		app.siteKeyDir, app.port = site, 0
		view, err := app.StartClass(channel, name)
		if err != nil {
			t.Fatal(err)
		}
		if view.Class.KeySource != keyCentre {
			t.Fatalf("console %s uses key %q, want the site key", channel, view.Class.KeySource)
		}
		t.Cleanup(app.EndClass)
		return app, view
	}
	main, mainView := open("1", "Sala principal")
	alt, altView := open("2", "Sala alterna")
	if mainView.Class.PublicKey != altView.Class.PublicKey {
		t.Fatal("the two consoles do not share the site key")
	}
	pub, _ := security.ParsePublicKey(mainView.Class.PublicKey)

	both := session.StaticSource{localConsole(altView.Class.Port, "2"), localConsole(mainView.Class.Port, "1")}
	startAgentsOn(t, 3, "1", "PRINCIPAL", both, pub)
	startAgentsOn(t, 2, "2", "ALTERNA", session.StaticSource{localConsole(mainView.Class.Port, "1"), localConsole(altView.Class.Port, "2")}, pub)

	waitUntil(t, "3 in the main room", func() bool { return main.View().Students.Connected == 3 })
	waitUntil(t, "2 in the alternate room", func() bool { return alt.View().Students.Connected == 2 })
	for _, s := range main.View().Students.Students {
		if !strings.HasPrefix(s.Hostname, "PRINCIPAL-") {
			t.Fatalf("%s showed up in the main room", s.Hostname)
		}
	}
	for _, s := range alt.View().Students.Students {
		if !strings.HasPrefix(s.Hostname, "ALTERNA-") {
			t.Fatalf("%s showed up in the alternate room", s.Hostname)
		}
	}
}
