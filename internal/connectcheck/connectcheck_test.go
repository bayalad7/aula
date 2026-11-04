package connectcheck

import (
	"context"
	"io"
	"log/slog"
	"net"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// console is a real console (TLS server, handshake, registry) on 127.0.0.1.
type console struct {
	id   *security.Identity
	cert security.Certificate
	port int
	reg  *session.Registry
}

func startConsole(t *testing.T, channel string) console {
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
	reg := session.NewRegistry(0)
	srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(cert), Logger: quiet})
	con := &session.Console{Channel: channel, Registry: reg, Identity: id, CertSHA256: cert.SHA256, Logger: quiet}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() { _ = srv.Serve(ctx, ln) })
	wg.Go(func() { _ = con.Run(ctx, srv.Events()) })
	t.Cleanup(func() { cancel(); wg.Wait() })
	return console{id: id, cert: cert, port: ln.Addr().(*net.TCPAddr).Port, reg: reg}
}

func (c console) announcement(channel string) discovery.Console {
	return discovery.Console{Instance: "aula-test", Channel: channel, Name: "Prueba",
		Addrs: []net.IP{net.IPv4(127, 0, 0, 1)}, Port: c.port, CertFingerprint: c.cert.Fingerprint()}
}

// fakeNet is a NetworkInspector with a fixed answer.
type fakeNet struct {
	st  platform.NetworkState
	err error
}

func (f fakeNet) Inspect(context.Context, string, int) (platform.NetworkState, error) {
	return f.st, f.err
}

var privateAllowed = platform.NetworkState{
	Networks: []platform.Network{{Name: "Laboratorio", Category: platform.NetworkPrivate}},
	Firewall: []platform.FirewallProfile{{Name: platform.NetworkPrivate, Enabled: true}},
	Rules:    []platform.FirewallRule{{Name: "Aula Consola", Allow: true, Profiles: []string{platform.NetworkPrivate, platform.NetworkDomain}}},
}

// agentCause runs a real agent and returns the first cause it reports.
func agentCause(t *testing.T, channel string, pub []byte, src session.ConsoleSource) session.Cause {
	t.Helper()
	got := make(chan session.Cause, 16)
	ctx, cancel := context.WithCancel(context.Background())
	agent := &session.Agent{
		Channel: channel, ConsolePublicKey: pub, Source: src, Logger: quiet,
		Hello: protocol.HelloInfo{Hostname: "PC01", OS: protocol.OSWindows, AgentVersion: "0.1.0",
			Capabilities: []protocol.Capability{protocol.CapCodeView}},
		Backoff:    session.Backoff{Base: 20 * time.Millisecond, Max: 50 * time.Millisecond},
		SearchHint: 50 * time.Millisecond,
		OnCause: func(c session.Cause) {
			select {
			case got <- c:
			default:
			}
		},
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = agent.Run(ctx) }()
	defer func() { cancel(); <-done }()
	select {
	case c := <-got:
		return c
	case <-time.After(10 * time.Second):
		t.Fatal("the agent reported no cause")
		return ""
	}
}

// Fase 7.3: for each induced failure, the checker reports the same cause as
// a real agent facing it.
func TestCheckerAgreesWithTheAgent(t *testing.T) {
	const channel = "7"
	con := startConsole(t, channel)
	other := startConsole(t, channel) // another certificate and key
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := closed.Addr().(*net.TCPAddr).Port
	_ = closed.Close()

	good := con.announcement(channel)
	wrongCert := good
	wrongCert.CertFingerprint = other.cert.Fingerprint()
	closedAnn := good
	closedAnn.Port = closedPort

	cases := []struct {
		name string
		src  session.StaticSource
		pub  []byte
		port int
		want session.Cause
	}{
		{"todo bien", session.StaticSource{good}, con.id.Public(), con.port, session.CauseNone},
		{"sin anuncio", session.StaticSource{}, con.id.Public(), con.port, session.CauseNoConsole},
		{"otra huella", session.StaticSource{wrongCert}, con.id.Public(), con.port, session.CauseCertMismatch},
		{"otra clave", session.StaticSource{good}, other.id.Public(), con.port, session.CauseWrongKey},
		{"puerto cerrado", session.StaticSource{closedAnn}, con.id.Public(), closedPort, session.CauseUnreachable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Checker{Channel: channel, PublicKey: tc.pub, Fingerprint: con.cert.Fingerprint(), Port: tc.port,
				Source: tc.src, Net: fakeNet{st: privateAllowed}, Loopback: true,
				BrowseFor: 500 * time.Millisecond, DialTimeout: time.Second, Logger: quiet}
			r := c.Run(context.Background(), nil)
			checkOutcomes(t, r)
			if !r.Done || r.Cause != tc.want {
				t.Fatalf("checker cause = %q, want %q; steps %+v", r.Cause, tc.want, r.Steps)
			}
			if got := agentCause(t, channel, tc.pub, tc.src); got != tc.want {
				t.Fatalf("agent cause = %q, want %q", got, tc.want)
			}
		})
	}
	// The probe closes before hello: no student was registered by the checker.
	// (The agents of "todo bien" did register; that console is the other one.)
	if n := len(other.reg.List(channel)); n != 0 {
		t.Fatalf("%d sessions on the console nobody should join", n)
	}
}

func TestProbeLeavesNoStudent(t *testing.T) {
	con := startConsole(t, "3")
	c := &Checker{Channel: "3", PublicKey: con.id.Public(), Fingerprint: con.cert.Fingerprint(), Port: con.port,
		Source: session.StaticSource{con.announcement("3")}, Loopback: true, BrowseFor: 300 * time.Millisecond, Logger: quiet}
	if r := c.Run(context.Background(), nil); r.Steps[2].Status != StatusOK {
		t.Fatalf("secure step = %+v", r.Steps[2])
	}
	time.Sleep(100 * time.Millisecond)
	if n := len(con.reg.List("3")); n != 0 {
		t.Fatalf("the probe left %d sessions", n)
	}
}

// The app never accepts loopback: the aula simulada listens on 127.0.0.1 and
// is not announced, and a student cannot use that address anyway.
func TestLoopbackIsNotAStudentPath(t *testing.T) {
	con := startConsole(t, "7")
	c := &Checker{Channel: "7", PublicKey: con.id.Public(), Fingerprint: con.cert.Fingerprint(), Port: con.port,
		LocalAddrs: []net.IP{net.IPv4(127, 0, 0, 1)},
		Source:     session.StaticSource{con.announcement("7")}, BrowseFor: 300 * time.Millisecond, Logger: quiet}
	r := c.Run(context.Background(), nil)
	checkOutcomes(t, r)
	if r.Steps[1].Status != StatusFail || r.Steps[1].Reason != ReasonNoAddress || r.Steps[2].Reason != ReasonSkipped {
		t.Fatalf("steps = %+v", r.Steps)
	}
}

func TestOtherConsolesOnTheChannel(t *testing.T) {
	con := startConsole(t, "7")
	alt := con.announcement("7")
	alt.Instance, alt.Name, alt.Port = "otra", "Sala alterna", con.port+1
	c := &Checker{Channel: "7", PublicKey: con.id.Public(), Fingerprint: con.cert.Fingerprint(), Port: con.port,
		Source: session.StaticSource{alt, con.announcement("7")}, Loopback: true, BrowseFor: 300 * time.Millisecond, Logger: quiet}
	r := c.Run(context.Background(), nil)
	checkOutcomes(t, r)
	if s := r.Steps[0]; s.Status != StatusWarn || s.Reason != ReasonOtherConsoles || !reflect.DeepEqual(s.Others, []string{"Sala alterna"}) {
		t.Fatalf("announce = %+v", s)
	}
	if r.Cause != session.CauseNone {
		t.Fatalf("cause = %q", r.Cause)
	}
}

func TestProgress(t *testing.T) {
	var seen []Report
	c := &Checker{Channel: "7", Source: session.StaticSource{}, BrowseFor: 100 * time.Millisecond, Logger: quiet}
	final := c.Run(context.Background(), func(r Report) { seen = append(seen, r) })
	if len(seen) != 9 || seen[0].Steps[0].Status != StatusRunning || !seen[8].Done {
		t.Fatalf("%d progress reports", len(seen))
	}
	if final.Steps[3].Reason != ReasonNotSupported {
		t.Fatalf("without an inspector the network step is %+v", final.Steps[3])
	}
}

func TestEvaluate(t *testing.T) {
	fw := []platform.FirewallProfile{
		{Name: platform.NetworkDomain, Enabled: true}, {Name: platform.NetworkPrivate, Enabled: true},
		{Name: platform.NetworkPublic, Enabled: true},
	}
	rule := platform.FirewallRule{Name: "Aula Consola", Allow: true, Profiles: []string{platform.NetworkPrivate, platform.NetworkDomain}}
	nets := func(cat string) []platform.Network { return []platform.Network{{Name: "Red", Category: cat}} }
	cases := []struct {
		name   string
		st     platform.NetworkState
		status Status
		cause  session.Cause
		reason string
	}{
		{"privada con regla", platform.NetworkState{Networks: nets(platform.NetworkPrivate), Firewall: fw, Rules: []platform.FirewallRule{rule}}, StatusOK, "", ""},
		{"pública sin regla", platform.NetworkState{Networks: nets(platform.NetworkPublic), Firewall: fw, Rules: []platform.FirewallRule{rule}}, StatusFail, session.CauseNoConsole, ReasonPublicBlocked},
		{"pública permitida", platform.NetworkState{Networks: nets(platform.NetworkPublic), Firewall: fw, Rules: []platform.FirewallRule{{Allow: true}}}, StatusWarn, "", ReasonPublic},
		{"privada sin regla", platform.NetworkState{Networks: nets(platform.NetworkPrivate), Firewall: fw}, StatusFail, session.CauseUnreachable, ReasonBlocked},
		{"regla que bloquea", platform.NetworkState{Networks: nets(platform.NetworkDomain), Firewall: fw,
			Rules: []platform.FirewallRule{rule, {Name: "bloqueo"}}}, StatusFail, session.CauseUnreachable, ReasonBlocked},
		{"firewall apagado", platform.NetworkState{Networks: nets(platform.NetworkPrivate),
			Firewall: []platform.FirewallProfile{{Name: platform.NetworkPrivate}}}, StatusOK, "", ""},
		{"sin red", platform.NetworkState{Firewall: fw}, StatusWarn, "", ReasonNoNetwork},
	}
	for _, tc := range cases {
		s := Evaluate(tc.st, 47830)
		checkOutcomes(t, Report{Steps: []Step{s}})
		if s.Status != tc.status || s.Cause != tc.cause || s.Reason != tc.reason {
			t.Errorf("%s: got %+v", tc.name, s)
		}
	}
}

func TestNetworkStepUnknown(t *testing.T) {
	for _, tc := range []struct {
		err    error
		reason string
	}{{platform.ErrNotSupported, ReasonNotSupported}, {io.ErrUnexpectedEOF, ReasonInspectError}} {
		c := &Checker{Net: fakeNet{err: tc.err}, Logger: quiet}
		s := c.network(context.Background())
		checkOutcomes(t, Report{Steps: []Step{s}})
		if s.Status != StatusUnknown || s.Reason != tc.reason {
			t.Errorf("%v: %+v", tc.err, s)
		}
	}
}

// checkOutcomes fails if a finished step ends with an outcome the console
// has no text for.
func checkOutcomes(t *testing.T, r Report) {
	t.Helper()
	for _, s := range r.Steps {
		if !slices.Contains(Outcomes[s.ID], s.Outcome()) {
			t.Errorf("step %s ended with %q, not listed in Outcomes", s.ID, s.Outcome())
		}
	}
}
