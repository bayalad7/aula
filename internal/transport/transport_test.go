package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/bayalad7/aula/internal/protocol"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func testOptions() Options {
	return Options{HeartbeatInterval: 50 * time.Millisecond, IdleTimeout: 400 * time.Millisecond, Logger: quiet}
}

// startServer runs a server on a loopback port and returns its URL.
func startServer(t *testing.T, opts Options) (*Server, string, context.CancelFunc) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := NewServer(opts)
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		// Drain so shutdown never blocks on an unread event.
		go func() {
			for range srv.Events() {
			}
		}()
		if err := <-served; err != nil {
			t.Errorf("Serve: %v", err)
		}
	})
	return srv, "ws://" + ln.Addr().String() + Path, cancel
}

func nextEvent(t *testing.T, srv *Server, kind EventKind) Event {
	t.Helper()
	select {
	case ev := <-srv.Events():
		if ev.Kind != kind {
			t.Fatalf("event = %s, want %s", ev.Kind, kind)
		}
		return ev
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for %s", kind)
	}
	return Event{}
}

func recv(t *testing.T, cl *Client) protocol.Envelope {
	t.Helper()
	select {
	case env, ok := <-cl.Incoming():
		if !ok {
			t.Fatal("client connection closed")
		}
		return env
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for message")
	}
	return protocol.Envelope{}
}

func TestSendAndReceiveBothWays(t *testing.T) {
	srv, url, _ := startServer(t, testOptions())
	cl, err := Dial(context.Background(), url, testOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close(CloseNormal, "")

	conn := nextEvent(t, srv, EventConnect).Conn
	id, err := cl.Send(context.Background(), protocol.MsgAck{RefID: protocol.NewID()})
	if err != nil {
		t.Fatal(err)
	}
	ev := nextEvent(t, srv, EventMessage)
	if ev.Conn != conn || ev.Envelope.ID != id || ev.Envelope.Type != protocol.TypeMsgAck {
		t.Fatalf("unexpected event %+v", ev)
	}

	share := protocol.CodeShare{SnippetID: protocol.NewID(), Lang: "go", Content: "package main\n", Mode: protocol.ModeReplace}
	if _, err := conn.Send(context.Background(), share); err != nil {
		t.Fatal(err)
	}
	if got := recv(t, cl).Payload; got != share {
		t.Fatalf("client got %#v", got)
	}
}

func TestClientCloseEmitsDisconnect(t *testing.T) {
	srv, url, _ := startServer(t, testOptions())
	cl, err := Dial(context.Background(), url, testOptions())
	if err != nil {
		t.Fatal(err)
	}
	conn := nextEvent(t, srv, EventConnect).Conn
	cl.Close(CloseNormal, "bye")
	ev := nextEvent(t, srv, EventDisconnect)
	if ev.Conn != conn {
		t.Fatal("disconnect for another connection")
	}
	if got := websocket.CloseStatus(ev.Err); got != websocket.StatusNormalClosure {
		t.Fatalf("close status = %v, want normal", got)
	}
	<-cl.Done()
	if _, err := cl.Send(context.Background(), protocol.Heartbeat{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Send after close: %v, want ErrClosed", err)
	}
}

// rawDial opens a WebSocket without the transport layer, to send bad input.
func rawDial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ws, _, err := websocket.Dial(context.Background(), url, nil)
	if err != nil {
		t.Fatal(err)
	}
	ws.SetReadLimit(protocol.MaxMessageSize)
	t.Cleanup(func() { _ = ws.CloseNow() })
	return ws
}

func readError(t *testing.T, ws *websocket.Conn) protocol.ErrorPayload {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		_, data, err := ws.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		env, err := protocol.Decode(data)
		if err != nil {
			t.Fatalf("decode reply: %v", err)
		}
		if p, ok := env.Payload.(protocol.ErrorPayload); ok {
			return p
		}
	}
}

func TestOversizedMessageClosesWith1009(t *testing.T) {
	srv, url, _ := startServer(t, testOptions())
	ws := rawDial(t, url)
	nextEvent(t, srv, EventConnect)
	big := make([]byte, protocol.MaxMessageSize+1)
	_ = ws.Write(context.Background(), websocket.MessageText, big)
	if got := closeStatus(t, ws); got != websocket.StatusMessageTooBig {
		t.Fatalf("peer saw close status %v, want 1009", got)
	}
	nextEvent(t, srv, EventDisconnect)
}

// closeStatus reads until the connection closes and returns the close code the
// server sent.
func closeStatus(t *testing.T, ws *websocket.Conn) websocket.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		if _, _, err := ws.Read(ctx); err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

func TestUnsupportedVersionRepliesAndCloses(t *testing.T) {
	srv, url, _ := startServer(t, testOptions())
	ws := rawDial(t, url)
	nextEvent(t, srv, EventConnect)
	id := protocol.NewID()
	msg := `{"v":2,"id":"` + id + `","ts":1,"type":"heartbeat","payload":{}}`
	if err := ws.Write(context.Background(), websocket.MessageText, []byte(msg)); err != nil {
		t.Fatal(err)
	}
	if e := readError(t, ws); e.Code != protocol.CodeUnsupportedVersion || e.RefID != id {
		t.Fatalf("error reply = %+v", e)
	}
	if got := closeStatus(t, ws); got != websocket.StatusPolicyViolation {
		t.Fatalf("close status = %v, want 1008", got)
	}
	nextEvent(t, srv, EventDisconnect)
}

func TestUnknownTypeAndBinaryKeepConnection(t *testing.T) {
	srv, url, _ := startServer(t, testOptions())
	ws := rawDial(t, url)
	nextEvent(t, srv, EventConnect)
	msg := `{"v":1,"id":"` + protocol.NewID() + `","ts":1,"type":"poll.start","payload":{}}`
	if err := ws.Write(context.Background(), websocket.MessageText, []byte(msg)); err != nil {
		t.Fatal(err)
	}
	if e := readError(t, ws); e.Code != protocol.CodeUnknownType {
		t.Fatalf("error reply = %+v", e)
	}
	if err := ws.Write(context.Background(), websocket.MessageBinary, []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if e := readError(t, ws); e.Code != protocol.CodeInvalidMessage || e.RefID != "" {
		t.Fatalf("error reply = %+v", e)
	}
	// Still usable: a valid message arrives as an event.
	ok := `{"v":1,"id":"` + protocol.NewID() + `","ts":1,"type":"heartbeat","payload":{}}`
	if err := ws.Write(context.Background(), websocket.MessageText, []byte(ok)); err != nil {
		t.Fatal(err)
	}
	nextEvent(t, srv, EventMessage)
}

func TestErrorIsNeverAnsweredWithError(t *testing.T) {
	srv, url, _ := startServer(t, testOptions())
	ws := rawDial(t, url)
	nextEvent(t, srv, EventConnect)
	bad := `{"v":1,"id":"` + protocol.NewID() + `","ts":1,"type":"error","payload":{"code":"BAD CODE"}}`
	if err := ws.Write(context.Background(), websocket.MessageText, []byte(bad)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, data, err := ws.Read(ctx); err == nil {
		t.Fatalf("got reply %s, want none", data)
	}
	_ = srv
}

func TestSilentPeerIsDropped(t *testing.T) {
	srv, url, _ := startServer(t, testOptions())
	ws := rawDial(t, url) // never sends anything
	start := time.Now()
	nextEvent(t, srv, EventConnect)
	nextEvent(t, srv, EventDisconnect)
	if d := time.Since(start); d < testOptions().IdleTimeout {
		t.Fatalf("dropped after %v, before the idle timeout", d)
	}
	_ = ws
}

func TestHeartbeatKeepsConnectionAlive(t *testing.T) {
	srv, url, _ := startServer(t, testOptions())
	cl, err := Dial(context.Background(), url, testOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close(CloseNormal, "")
	conn := nextEvent(t, srv, EventConnect).Conn
	cl.StartHeartbeat()
	conn.StartHeartbeat()
	deadline := time.After(3 * testOptions().IdleTimeout)
	for {
		select {
		case ev := <-srv.Events():
			if ev.Kind == EventDisconnect {
				t.Fatalf("connection dropped despite heartbeats: %v", ev.Err)
			}
		case env := <-cl.Incoming():
			if env.Type != protocol.TypeHeartbeat {
				t.Fatalf("unexpected %s", env.Type)
			}
		case <-deadline:
			return
		}
	}
}

func TestShutdownClosesClientsAndEvents(t *testing.T) {
	srv, url, cancel := startServer(t, testOptions())
	cl, err := Dial(context.Background(), url, testOptions())
	if err != nil {
		t.Fatal(err)
	}
	nextEvent(t, srv, EventConnect)
	cancel()
	nextEvent(t, srv, EventDisconnect)
	select {
	case _, ok := <-srv.Events():
		if ok {
			t.Fatal("unexpected event after disconnect")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Events not closed after shutdown")
	}
	select {
	case <-cl.Done():
		if got := websocket.CloseStatus(cl.Err()); got != websocket.StatusGoingAway {
			t.Fatalf("client close status = %v, want 1001", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client not closed")
	}
}

func TestDialFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	_, err = Dial(context.Background(), "ws://"+addr+Path, Options{DialTimeout: time.Second, Logger: quiet})
	if err == nil || !strings.Contains(err.Error(), "dial") {
		t.Fatalf("Dial to closed port: %v", err)
	}
}
