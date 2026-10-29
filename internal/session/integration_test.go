package session

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/transport"
)

// Integration tests: transport.Server + Console + Registry against Agents on
// loopback, over TLS with the real handshake.

// keys is a console's identity and TLS certificate, saved in dir.
type keys struct {
	dir  string
	id   *security.Identity
	cert security.Certificate
}

func newKeys(t *testing.T) keys {
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
	return keys{dir: dir, id: id, cert: cert}
}

type teacher struct {
	reg     *Registry
	console *Console
	addr    *net.TCPAddr
	keys    keys
	stop    func()
}

func connOpts() transport.Options {
	return transport.Options{HeartbeatInterval: 50 * time.Millisecond, IdleTimeout: 600 * time.Millisecond, Logger: quiet}
}

// startTeacher wires transport.Server (TLS), Console and Registry on loopback.
func startTeacher(t *testing.T, addr string, k keys, log *slog.Logger) *teacher {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	reg := NewRegistry(600 * time.Millisecond)
	opts := connOpts()
	opts.TLS = security.ServerTLS(k.cert)
	opts.Logger = log
	srv := transport.NewServer(opts)
	console := &Console{
		Channel: testChannel, Registry: reg, Identity: k.id, CertSHA256: k.cert.SHA256,
		HelloTimeout: 500 * time.Millisecond, Logger: log,
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = srv.Serve(ctx, ln) }()
	go func() {
		defer wg.Done()
		if err := console.Run(ctx, srv.Events()); err != nil {
			t.Errorf("console: %v", err)
		}
	}()
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); wg.Wait() }) }
	t.Cleanup(stop)
	return &teacher{reg: reg, console: console, addr: ln.Addr().(*net.TCPAddr), keys: k, stop: stop}
}

// source announces tc as mDNS would, with its real certificate fingerprint.
func (tc *teacher) source() StaticSource {
	return sourceFor(tc.addr, tc.keys.cert.Fingerprint())
}

func sourceFor(addr *net.TCPAddr, fingerprint string) StaticSource {
	return StaticSource{{
		Instance: "test", Channel: testChannel, Name: "Prueba",
		Addrs: []net.IP{addr.IP}, Port: addr.Port, CertFingerprint: fingerprint,
	}}
}

type agentOpts struct {
	pub       ed25519.PublicKey
	log       *slog.Logger
	onMessage func(context.Context, *transport.Client, protocol.Envelope)
	onState   func(AgentState, discovery.Console)
	caps      []protocol.Capability // nil: code.view and msg.show
}

func startAgent(t *testing.T, host string, src ConsoleSource, o agentOpts) context.CancelFunc {
	t.Helper()
	if o.log == nil {
		o.log = quiet
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Agent{
		Channel:          testChannel,
		ConsolePublicKey: o.pub,
		HandshakeTimeout: time.Second,
		Hello:            helloWith(host, o.caps),
		Source:           src,
		Backoff:          Backoff{Base: 20 * time.Millisecond, Max: 100 * time.Millisecond},
		Conn:             connOpts(),
		OnMessage:        o.onMessage,
		OnState:          o.onState,
		Logger:           o.log,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := a.Run(ctx); err != nil {
			t.Errorf("agent: %v", err)
		}
	}()
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); <-done }) }
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

// syncBuffer collects log output from several goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func captureLogs() (*slog.Logger, *syncBuffer) {
	buf := &syncBuffer{}
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})), buf
}

// dialAuthenticated connects like an agent and completes auth, so tests can
// then misbehave in a precise way.
func dialAuthenticated(t *testing.T, tc *teacher, opts transport.Options) *transport.Client {
	t.Helper()
	var seen [32]byte
	tlsConf, err := security.ClientTLS(tc.keys.cert.Fingerprint(), &seen)
	if err != nil {
		t.Fatal(err)
	}
	opts.TLS = tlsConf
	cl, err := transport.Dial(context.Background(), "wss://"+tc.addr.String()+transport.Path, opts)
	if err != nil {
		t.Fatal(err)
	}
	nonce := security.NewNonce()
	cid, err := cl.Send(context.Background(), protocol.AuthChallenge{Nonce: nonce})
	if err != nil {
		t.Fatal(err)
	}
	env := <-cl.Incoming()
	proof, ok := env.Payload.(protocol.AuthProof)
	if !ok || proof.RefID != cid || !security.VerifyProof(tc.keys.id.Public(), nonce, testChannel, seen, proof.Signature) {
		t.Fatalf("bad proof: %+v", env)
	}
	if _, err := cl.Send(context.Background(), protocol.AuthOK{RefID: env.ID}); err != nil {
		t.Fatal(err)
	}
	return cl
}

func drainIncoming(cl *transport.Client) {
	go func() {
		for range cl.Incoming() {
		}
	}()
}

// --- Session lifecycle --------------------------------------------------------

func TestTwoAgentsAppearAndOneLeaves(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	pub := tc.keys.id.Public()
	startAgent(t, "PC01", tc.source(), agentOpts{pub: pub})
	stop2 := startAgent(t, "PC02", tc.source(), agentOpts{pub: pub})

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
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	// Authenticates and says hello, then nothing: no heartbeats, socket open.
	cl := dialAuthenticated(t, tc, transport.Options{IdleTimeout: time.Minute, Logger: quiet})
	defer cl.Close(transport.CloseNormal, "")
	drainIncoming(cl)
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

func TestFirstMessageAfterAuthMustBeHello(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	cl := dialAuthenticated(t, tc, connOpts())
	if _, err := cl.Send(context.Background(), protocol.Heartbeat{}); err != nil {
		t.Fatal(err)
	}
	var gotErr bool
	for env := range cl.Incoming() {
		if e, ok := env.Payload.(protocol.ErrorPayload); ok && e.Code == protocol.CodeInvalidMessage {
			gotErr = true
		}
	}
	if !gotErr {
		t.Fatal("no invalid_message error before close")
	}
	if n := len(tc.reg.List(testChannel)); n != 0 {
		t.Fatalf("registry has %d sessions", n)
	}
}

func TestHandshakeTimeout(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	var seen [32]byte
	tlsConf, _ := security.ClientTLS(tc.keys.cert.Fingerprint(), &seen)
	opts := connOpts()
	opts.TLS = tlsConf
	cl, err := transport.Dial(context.Background(), "wss://"+tc.addr.String()+transport.Path, opts)
	if err != nil {
		t.Fatal(err)
	}
	drainIncoming(cl) // connected over TLS, but never sends auth.challenge
	select {
	case <-cl.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("connection without handshake not closed")
	}
}

func TestAgentReconnectsWhenConsoleComesBack(t *testing.T) {
	k := newKeys(t)
	// Reserve a port, then start the agent before the console exists.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	ln.Close()

	var mu sync.Mutex
	var seen []AgentState
	stop := startAgent(t, "PC04", sourceFor(tcpAddr, k.cert.Fingerprint()), agentOpts{
		pub:     k.id.Public(),
		onState: func(s AgentState, _ discovery.Console) { mu.Lock(); seen = append(seen, s); mu.Unlock() },
	})
	time.Sleep(300 * time.Millisecond) // a few failed attempts

	tc := startTeacher(t, tcpAddr.String(), k, quiet)
	waitFor(t, tc.reg, "PC04 connected", func(l []Session) bool { return states(l)["PC04"] == StateConnected })

	tc.stop() // the console goes away and comes back with the same keys
	tc2 := startTeacher(t, tcpAddr.String(), k, quiet)
	waitFor(t, tc2.reg, "PC04 back", func(l []Session) bool { return states(l)["PC04"] == StateConnected })

	stop()
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

// --- Authentication -----------------------------------------------------------

// neverConnects starts an agent and checks it does not reach the session.
func neverConnects(t *testing.T, tc *teacher, src ConsoleSource, pub ed25519.PublicKey) string {
	t.Helper()
	log, buf := captureLogs()
	var mu sync.Mutex
	connected := false
	stop := startAgent(t, "PC66", src, agentOpts{pub: pub, log: log,
		onState: func(s AgentState, _ discovery.Console) {
			if s == AgentConnected {
				mu.Lock()
				connected = true
				mu.Unlock()
			}
		}})
	time.Sleep(time.Second)
	stop()
	mu.Lock()
	defer mu.Unlock()
	if connected {
		t.Fatal("agent connected to a console it must reject")
	}
	if n := len(tc.reg.List(testChannel)); n != 0 {
		t.Fatalf("console registered %d sessions", n)
	}
	return buf.String()
}

func TestAgentWithCorrectKeyAuthenticates(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	startAgent(t, "PC10", tc.source(), agentOpts{pub: tc.keys.id.Public()})
	waitFor(t, tc.reg, "PC10 connected", func(l []Session) bool { return states(l)["PC10"] == StateConnected })
}

func TestAgentWithAnotherKeyRejectsConsole(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	other, err := security.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	logs := neverConnects(t, tc, tc.source(), other.Public())
	if !strings.Contains(logs, "console rejected") || !strings.Contains(logs, "signature does not verify") {
		t.Fatalf("rejection not logged:\n%s", logs)
	}
}

func TestFingerprintMismatchRejected(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	decoy := newKeys(t) // announcement says decoy's certificate, console presents its own
	logs := neverConnects(t, tc, sourceFor(tc.addr, decoy.cert.Fingerprint()), tc.keys.id.Public())
	if !strings.Contains(logs, "does not match its announcement") {
		t.Fatalf("pinning failure not logged:\n%s", logs)
	}
}

func TestDirectConnectionWithoutFingerprint(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	startAgent(t, "PC11", sourceFor(tc.addr, ""), agentOpts{pub: tc.keys.id.Public()})
	waitFor(t, tc.reg, "PC11 connected", func(l []Session) bool { return states(l)["PC11"] == StateConnected })
}

// A relay with its own certificate sits between agent and the real console and
// forwards every message. Without a pinned fingerprint TLS succeeds, but the
// console signs its own certificate hash, not the relay's, so the agent must
// still reject it (docs/protocol.md §10.2).
func TestRelayWithOwnCertificateDetected(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	relayKeys := newKeys(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ropts := connOpts()
	ropts.TLS = security.ServerTLS(relayKeys.cert)
	relay := transport.NewServer(ropts)
	go func() { _ = relay.Serve(ctx, ln) }()
	// A faithful relay: it forwards payloads unchanged and translates ref_id
	// between the two connections, so correlation checks pass and only the
	// signature binding can catch it.
	type link struct {
		mu            sync.Mutex
		up            *transport.Client
		downChallenge string // agent's auth.challenge id
		upProof       string // console's auth.proof id
		downProofSent string // id of the proof as sent to the agent
	}
	go func() {
		links := map[*transport.Conn]*link{}
		for ev := range relay.Events() {
			switch ev.Kind {
			case transport.EventConnect:
				var seen [32]byte
				tlsConf, _ := security.ClientTLS(tc.keys.cert.Fingerprint(), &seen)
				o := connOpts()
				o.TLS = tlsConf
				up, err := transport.Dial(ctx, "wss://"+tc.addr.String()+transport.Path, o)
				if err != nil {
					ev.Conn.Close(transport.CloseGoingAway, "")
					continue
				}
				l := &link{up: up}
				links[ev.Conn] = l
				down := ev.Conn
				go func() { // console → agent
					for env := range up.Incoming() {
						p := env.Payload
						l.mu.Lock()
						if proof, ok := p.(protocol.AuthProof); ok {
							proof.RefID = l.downChallenge
							p = proof
							l.upProof = env.ID
						}
						l.mu.Unlock()
						id, _ := down.Send(ctx, p)
						if _, ok := p.(protocol.AuthProof); ok {
							l.mu.Lock()
							l.downProofSent = id
							l.mu.Unlock()
						}
					}
				}()
			case transport.EventMessage: // agent → console
				l := links[ev.Conn]
				if l == nil {
					continue
				}
				p := ev.Envelope.Payload
				l.mu.Lock()
				switch m := p.(type) {
				case protocol.AuthChallenge:
					l.downChallenge = ev.Envelope.ID
				case protocol.AuthOK:
					m.RefID = l.upProof
					p = m
				case protocol.ErrorPayload:
					if m.RefID == l.downProofSent {
						m.RefID = l.upProof
						p = m
					}
				}
				l.mu.Unlock()
				_, _ = l.up.Send(ctx, p)
			case transport.EventDisconnect:
				if l := links[ev.Conn]; l != nil {
					l.up.Close(transport.CloseNormal, "")
				}
			}
		}
	}()
	logs := neverConnects(t, tc, sourceFor(ln.Addr().(*net.TCPAddr), ""), tc.keys.id.Public())
	if !strings.Contains(logs, "signature does not verify") {
		t.Fatalf("relay not rejected by the signature check:\n%s", logs)
	}
}

// A console that sends code.share before auth.ok: the agent must answer
// unauthorized, close, and never hand the message to OnMessage.
func TestCodeShareBeforeAuthOKRejected(t *testing.T) {
	k := newKeys(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts := connOpts()
	opts.TLS = security.ServerTLS(k.cert)
	srv := transport.NewServer(opts)
	go func() { _ = srv.Serve(ctx, ln) }()

	var mu sync.Mutex
	delivered := 0
	startAgent(t, "PC12", sourceFor(ln.Addr().(*net.TCPAddr), k.cert.Fingerprint()), agentOpts{
		pub:       k.id.Public(),
		onMessage: func(context.Context, *transport.Client, protocol.Envelope) { mu.Lock(); delivered++; mu.Unlock() },
	})

	gotUnauthorized := false
	timeout := time.After(5 * time.Second)
loop:
	for {
		select {
		case ev := <-srv.Events():
			if ev.Kind == transport.EventDisconnect && gotUnauthorized {
				break loop
			}
			if ev.Kind != transport.EventMessage {
				continue
			}
			switch p := ev.Envelope.Payload.(type) {
			case protocol.AuthChallenge:
				share := protocol.CodeShare{SnippetID: protocol.NewID(), Lang: "go", Content: "x", Mode: protocol.ModeReplace}
				_, _ = ev.Conn.Send(ctx, share)
			case protocol.ErrorPayload:
				gotUnauthorized = p.Code == protocol.CodeUnauthorized
			}
		case <-timeout:
			t.Fatalf("agent did not reject code.share before auth (unauthorized=%v)", gotUnauthorized)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if delivered != 0 {
		t.Fatalf("OnMessage received %d messages before auth.ok", delivered)
	}
	cancel()
	go func() {
		for range srv.Events() {
		}
	}()
}

// The console mirror image: anything before auth.ok gets unauthorized + 1008.
func TestConsoleRejectsMessagesBeforeAuth(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	var seen [32]byte
	tlsConf, _ := security.ClientTLS(tc.keys.cert.Fingerprint(), &seen)
	opts := connOpts()
	opts.TLS = tlsConf
	cl, err := transport.Dial(context.Background(), "wss://"+tc.addr.String()+transport.Path, opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cl.Send(context.Background(), hello("PC13")); err != nil { // hello without auth
		t.Fatal(err)
	}
	var code protocol.ErrorCode
	for env := range cl.Incoming() {
		if e, ok := env.Payload.(protocol.ErrorPayload); ok {
			code = e.Code
		}
	}
	if code != protocol.CodeUnauthorized {
		t.Fatalf("error code = %q, want unauthorized", code)
	}
	if n := len(tc.reg.List(testChannel)); n != 0 {
		t.Fatalf("registry has %d sessions", n)
	}
}

// --- The private key never reaches the logs --------------------------------

func TestPrivateKeyNeverLogged(t *testing.T) {
	log, buf := captureLogs()
	k := newKeys(t)
	tc := startTeacher(t, "127.0.0.1:0", k, log)
	startAgent(t, "PC20", tc.source(), agentOpts{pub: k.id.Public(), log: log})
	waitFor(t, tc.reg, "PC20 connected", func(l []Session) bool { return states(l)["PC20"] == StateConnected })
	other, _ := security.GenerateIdentity()
	neverConnects(t, &teacher{reg: NewRegistry(0), keys: k}, tc.source(), other.Public()) // failing path too

	// Accidental logging or printing of the identity itself.
	log.Info("identity", "id", k.id)
	log.Debug(fmt.Sprintf("%v %+v %#v %s %x %q", k.id, k.id, k.id, k.id, k.id, k.id))
	t.Logf("identity prints as %v", k.id)

	priv := readPrivateKey(t, k.dir)
	logs := buf.String()
	for name, secret := range secretEncodings(priv) {
		if strings.Contains(logs, secret) {
			t.Fatalf("logs contain the private key (%s)", name)
		}
	}
	if !strings.Contains(logs, security.KeyFingerprint(k.id.Public())) {
		t.Fatal("identity was not logged by its public fingerprint")
	}
}

func readPrivateKey(t *testing.T, dir string) ed25519.PrivateKey {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, security.IdentityFile))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(data)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return key.(ed25519.PrivateKey)
}

// secretEncodings lists the ways the private key could show up in text.
func secretEncodings(priv ed25519.PrivateKey) map[string]string {
	seed := priv.Seed()
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	pemBody := base64.StdEncoding.EncodeToString(der)
	return map[string]string{
		"seed base64":     base64.StdEncoding.EncodeToString(seed)[:40],
		"seed raw base64": base64.RawURLEncoding.EncodeToString(seed)[:40],
		"seed hex":        hex.EncodeToString(seed),
		"seed HEX":        strings.ToUpper(hex.EncodeToString(seed)),
		"key base64":      base64.StdEncoding.EncodeToString(priv)[:40],
		"key hex":         hex.EncodeToString(priv[:32]),
		"PEM body":        pemBody[:40],
		"seed bytes":      fmt.Sprint([]byte(seed))[:60],
	}
}

// --- Code distribution ---------------------------------------------------------

func helloWith(host string, caps []protocol.Capability) protocol.HelloInfo {
	h := hello(host)
	if caps != nil {
		h.Capabilities = caps
	}
	return h
}

// receiverAgent answers like the student app: codedist.Receiver applies the
// message and its reply (the ack) is sent back.
func receiverAgent(board *codedist.Board) func(context.Context, *transport.Client, protocol.Envelope) {
	r := &codedist.Receiver{Board: board}
	return func(ctx context.Context, cl *transport.Client, env protocol.Envelope) {
		if reply := r.Handle(env); reply != nil {
			_, _ = cl.Send(ctx, reply)
		}
	}
}

func TestCodeShareIsAckedAndRecorded(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	board := codedist.NewBoard(0)
	startAgent(t, "PC30", tc.source(), agentOpts{pub: tc.keys.id.Public(), onMessage: receiverAgent(board)})
	list := waitFor(t, tc.reg, "PC30 connected", func(l []Session) bool { return states(l)["PC30"] == StateConnected })
	id := list[0].ID

	share := protocol.CodeShare{
		SnippetID: protocol.NewID(), Exercise: "3", Title: "Factorial", Lang: "python",
		Content: "def f(n):\n    return 1 if n <= 1 else n * f(n - 1)\n", Mode: protocol.ModeReplace,
	}
	if _, err := tc.console.Send(context.Background(), id, share); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tc.reg, "code.ack recorded", func(l []Session) bool {
		return len(l) == 1 && l[0].LastCodeAck == share.SnippetID && !l[0].LastCodeAckAt.IsZero()
	})
	got, ok := board.Get(share.SnippetID)
	if !ok || got.Content != share.Content || got.Exercise != "3" {
		t.Fatalf("agent board = %+v", board.Snippets())
	}

	// code.clear is acked with an empty snippet_id.
	sent, failed := tc.console.Broadcast(context.Background(), protocol.CodeClear{})
	if len(sent) != 1 || len(failed) != 0 {
		t.Fatalf("Broadcast: sent %v, failed %v", sent, failed)
	}
	waitFor(t, tc.reg, "clear acked", func(l []Session) bool { return l[0].LastCodeAck == "" })
	if len(board.Snippets()) != 0 {
		t.Fatal("board not cleared")
	}
}

func TestSendRefusesMissingCapability(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	startAgent(t, "PC31", tc.source(), agentOpts{pub: tc.keys.id.Public(), caps: []protocol.Capability{protocol.CapMsgShow}})
	list := waitFor(t, tc.reg, "PC31 connected", func(l []Session) bool { return states(l)["PC31"] == StateConnected })
	share := protocol.CodeShare{SnippetID: protocol.NewID(), Lang: "go", Content: "x", Mode: protocol.ModeReplace}
	if _, err := tc.console.Send(context.Background(), list[0].ID, share); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("Send = %v, want ErrUnsupportedCapability", err)
	}
	if _, err := tc.console.Send(context.Background(), "no-such-session", protocol.CodeClear{}); !errors.Is(err, ErrUnknownSession) {
		t.Fatalf("Send to unknown = %v", err)
	}
}

// --- Diagnostics: why the agent is not connected -------------------------------

// diagAgent runs an agent until it reports a diagnostic containing want.
func diagAgent(t *testing.T, src ConsoleSource, pub ed25519.PublicKey, want string) string {
	t.Helper()
	got := make(chan string, 16)
	ctx, cancel := context.WithCancel(context.Background())
	a := &Agent{
		Channel: testChannel, ConsolePublicKey: pub, Hello: hello("PC40"), Source: src,
		HandshakeTimeout: time.Second, SearchHint: 200 * time.Millisecond,
		Backoff: Backoff{Base: 20 * time.Millisecond, Max: 100 * time.Millisecond},
		Conn:    connOpts(), Logger: quiet,
		OnDiagnostic: func(msg string) {
			select {
			case got <- msg:
			default:
			}
		},
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = a.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case msg := <-got:
			if strings.Contains(msg, want) {
				return msg
			}
		case <-deadline:
			t.Fatalf("no diagnostic containing %q", want)
		}
	}
}

func TestDiagnosticWrongCentreKey(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	other, _ := security.GenerateIdentity()
	msg := diagAgent(t, tc.source(), other.Public(), "no es la de este centro")
	if !strings.Contains(msg, "Prueba") || !strings.Contains(msg, "canal "+testChannel) {
		t.Fatalf("diagnostic lacks console name or channel: %q", msg)
	}
}

func TestDiagnosticUnreachableConsole(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	ln.Close() // nobody listens there now
	k := newKeys(t)
	msg := diagAgent(t, sourceFor(addr, k.cert.Fingerprint()), k.id.Public(), "no se pudo conectar")
	if !strings.Contains(msg, addr.String()) {
		t.Fatalf("diagnostic lacks the address: %q", msg)
	}
}

func TestDiagnosticNoConsoleInChannel(t *testing.T) {
	k := newKeys(t)
	diagAgent(t, StaticSource{}, k.id.Public(), "No se encontró ninguna consola del canal "+testChannel)
}

func TestDiagnosticClearedOnConnect(t *testing.T) {
	tc := startTeacher(t, "127.0.0.1:0", newKeys(t), quiet)
	msg := diagAgent(t, tc.source(), tc.keys.id.Public(), "")
	if msg != "" {
		t.Fatalf("connected agent reported %q", msg)
	}
}
