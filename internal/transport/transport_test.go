package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
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

// recordingListener keeps a copy of every byte the server receives and sends,
// as a network capture would.
type recordingListener struct {
	net.Listener
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *recordingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &recordingConn{Conn: c, l: l}, nil
}

func (l *recordingListener) bytes() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return bytes.Clone(l.buf.Bytes())
}

type recordingConn struct {
	net.Conn
	l *recordingListener
}

func (c *recordingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.l.mu.Lock()
	c.l.buf.Write(p[:n])
	c.l.mu.Unlock()
	return n, err
}

func (c *recordingConn) Write(p []byte) (int, error) {
	c.l.mu.Lock()
	c.l.buf.Write(p)
	c.l.mu.Unlock()
	return c.Conn.Write(p)
}

// sendMarker runs a server on a recording listener, sends a message containing
// marker in each direction and returns the raw bytes seen on the wire. Client
// frames are masked by WebSocket itself (RFC 6455 §5.3), so only the
// server-to-client copy can show the text when TLS is off.
func sendMarker(t *testing.T, serverTLS, clientTLS *tls.Config, scheme, marker string) []byte {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rec := &recordingListener{Listener: ln}
	ctx, cancel := context.WithCancel(context.Background())
	opts := testOptions()
	opts.TLS = serverTLS
	srv := NewServer(opts)
	served := make(chan struct{})
	go func() { defer close(served); _ = srv.Serve(ctx, rec) }()
	defer func() {
		cancel()
		go func() {
			for range srv.Events() {
			}
		}()
		<-served
	}()

	copts := testOptions()
	copts.TLS = clientTLS
	cl, err := Dial(context.Background(), scheme+"://"+ln.Addr().String()+Path, copts)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close(CloseNormal, "")
	conn := nextEvent(t, srv, EventConnect).Conn
	msg := protocol.MsgShow{Text: marker, Level: protocol.LevelInfo}
	if _, err := cl.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, srv, EventMessage); ev.Envelope.Payload.(protocol.MsgShow).Text != marker {
		t.Fatal("message altered in transit")
	}
	if _, err := conn.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if env := recv(t, cl); env.Payload.(protocol.MsgShow).Text != marker {
		t.Fatal("message altered in transit")
	}
	return rec.bytes()
}

// TestTrafficIsEncrypted is how stage 05 checks that the WebSocket really goes
// over TLS: the bytes on the wire start with a TLS handshake record and do not
// contain the message text, while the same check on plain ws:// does find it.
func TestTrafficIsEncrypted(t *testing.T) {
	cert, err := security.LoadOrCreateCertificate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const marker = "TEXTO-SECRETO-DEL-PROFESOR-42"
	var seen [32]byte
	clientTLS, err := security.ClientTLS(cert.Fingerprint(), &seen)
	if err != nil {
		t.Fatal(err)
	}

	wire := sendMarker(t, security.ServerTLS(cert), clientTLS, "wss", marker)
	if len(wire) < 3 || wire[0] != 0x16 || wire[1] != 0x03 {
		t.Fatalf("wire does not start with a TLS handshake record: % x", wire[:min(len(wire), 8)])
	}
	for _, plain := range []string{marker, "msg.show", "Upgrade: websocket", "GET /ws"} {
		if bytes.Contains(wire, []byte(plain)) {
			t.Fatalf("%q visible on the wire", plain)
		}
	}
	if seen != cert.SHA256 {
		t.Fatal("client did not pin the server certificate")
	}

	// Control: the same capture without TLS shows the text, so the check above
	// would catch plaintext.
	if plain := sendMarker(t, nil, nil, "ws", marker); !bytes.Contains(plain, []byte(marker)) {
		t.Fatal("control capture without TLS does not contain the marker; the test proves nothing")
	}
}
