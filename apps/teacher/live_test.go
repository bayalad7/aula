package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
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
	view, err := app.StartClass("AULA_TEST", "Prueba")
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
			Channel: "AULA_TEST", ConsolePublicKey: pub,
			Hello: protocol.HelloInfo{Hostname: host, OS: protocol.OSWindows, AgentVersion: "0.1.0",
				Capabilities: []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow}},
			Source:  session.StaticSource{{Instance: "x", Channel: "AULA_TEST", Addrs: []net.IP{net.IPv4(127, 0, 0, 1)}, Port: view.Class.Port}},
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
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}
