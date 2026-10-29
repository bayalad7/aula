package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/policy"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

// liveConsole is a real console on loopback whose registry the test reads.
type liveConsole struct {
	addr, key string
	reg       *session.Registry
	console   *session.Console
}

func startLiveConsole(t *testing.T) liveConsole {
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
	ctx, cancel := context.WithCancel(context.Background())
	reg := session.NewRegistry(0)
	srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(cert), Logger: quiet})
	console := &session.Console{Channel: "1", Registry: reg, Identity: id, CertSHA256: cert.SHA256, Logger: quiet}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = srv.Serve(ctx, ln) }()
	go func() { defer wg.Done(); _ = console.Run(ctx, srv.Events()) }()
	t.Cleanup(func() { cancel(); wg.Wait() })
	return liveConsole{addr: net.JoinHostPort("127.0.0.1", strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)),
		key: security.EncodePublicKey(id.Public()), reg: reg, console: console}
}

// waitAgent waits until the console has a connected agent matching cond.
func (c liveConsole) waitAgent(t *testing.T, what string, cond func(session.Session) bool) session.Session {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, s := range c.reg.List("1") {
			if s.State == session.StateConnected && cond(s) {
				return s
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; agents: %+v", what, c.reg.List("1"))
	return session.Session{}
}

// fakeHelper is the helper's end of the channel: it records what the service
// asks of the student's screen and clipboard.
type fakeHelper struct {
	link *agentsvc.Link
	mu   sync.Mutex
	clip []string
	evs  []string
}

func newFakeHelper(t *testing.T, conn net.Conn) *fakeHelper {
	h := &fakeHelper{}
	h.link = agentsvc.NewLink(conn, func(_ context.Context, name string, data json.RawMessage) (any, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		switch name {
		case agentsvc.CallClipboardWrite:
			var s string
			_ = json.Unmarshal(data, &s)
			h.clip = append(h.clip, s)
			return nil, nil
		case agentsvc.CallIndicatorShow, agentsvc.CallIndicatorHide:
			return nil, nil
		}
		return nil, fmt.Errorf("%s: unexpected", name)
	}, func(name string, data json.RawMessage) {
		h.mu.Lock()
		h.evs = append(h.evs, name)
		h.mu.Unlock()
	})
	ctx, cancel := context.WithCancel(context.Background())
	go h.link.Run(ctx)
	t.Cleanup(func() { cancel(); h.link.Close() })
	return h
}

func (h *fakeHelper) binding(t *testing.T, out any, method string, args ...any) error {
	t.Helper()
	b, err := agentsvc.NewBinding(method, args...)
	if err != nil {
		t.Fatal(err)
	}
	return h.link.Call(t.Context(), agentsvc.CallBinding, b, out)
}

func capsOf(s session.Session) []string {
	var out []string
	for _, c := range s.Capabilities {
		out = append(out, string(c))
	}
	slices.Sort(out)
	return out
}

// The agent in the service shows up without capabilities while there is no
// helper, reconnects with the session's user and the helper's capabilities
// when one is ready, reaches the student's clipboard only through it, and
// drops the capabilities when it goes.
func TestServiceAgentFollowsItsHelper(t *testing.T) {
	c := startLiveConsole(t)
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: c.addr}
	pub, err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
	sa := newServiceAgent(cfg, pub, nil, quiet)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sa.app.start(ctx)
	defer sa.app.stop()

	s := c.waitAgent(t, "agent without helper", func(session.Session) bool { return true })
	if len(s.Capabilities) != 0 || s.Username != "" {
		t.Fatalf("without a helper the agent declared %v as %q", s.Capabilities, s.Username)
	}

	// A helper starts in Ana's session and reports what it can do.
	svcEnd, helperEnd := net.Pipe()
	ana := platform.Session{ID: 1, User: "ana", SID: "S-1-5-21-1-1001"}
	served := make(chan struct{})
	go func() { defer close(served); sa.serve(ctx, ana, svcEnd) }()
	h := newFakeHelper(t, helperEnd)
	if err := h.link.Notify(agentsvc.EventReady, agentsvc.Ready{Capture: true, Clipboard: true, Broadcast: true}); err != nil {
		t.Fatal(err)
	}
	s = c.waitAgent(t, "agent with helper", func(s session.Session) bool { return len(s.Capabilities) > 0 })
	want := []string{"code.view", "msg.show", "screen.broadcast", "screen.thumb"}
	if !slices.Equal(capsOf(s), want) || s.Username != "ana" {
		t.Fatalf("with a helper: %v as %q, want %v as ana", capsOf(s), s.Username, want)
	}

	// The teacher shares code; the window copies it: the text goes from the
	// service to the helper's clipboard.
	share := protocol.CodeShare{SnippetID: protocol.NewID(), Title: "t", Lang: "python", Content: "print(1)\n", Mode: protocol.ModeReplace}
	if _, err := c.console.Send(ctx, s.ID, share); err != nil {
		t.Fatal(err)
	}
	var v View
	deadline := time.Now().Add(5 * time.Second)
	for len(v.Snippets) == 0 && time.Now().Before(deadline) {
		if err := h.binding(t, &v, "View"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(v.Snippets) != 1 || v.Status.State != StateConnected {
		t.Fatalf("view through the helper: %+v", v)
	}
	if err := h.binding(t, nil, "Copy", v.Snippets[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := h.binding(t, nil, "Copy", "no-existe"); err == nil || !strings.Contains(err.Error(), "ya no está") {
		t.Fatalf("copy of a missing snippet: %v", err)
	}
	h.mu.Lock()
	clip := slices.Clone(h.clip)
	sawEmit := slices.Contains(h.evs, agentsvc.EventEmit)
	h.mu.Unlock()
	if len(clip) != 1 || clip[0] != "print(1)\n" || !sawEmit {
		t.Fatalf("clipboard %q, events seen %v", clip, sawEmit)
	}

	// The helper goes (logoff, or the student killed it): no capabilities.
	h.link.Close()
	<-served
	s = c.waitAgent(t, "agent after the helper left", func(s session.Session) bool { return len(s.Capabilities) == 0 })
	if s.Username != "" {
		t.Fatalf("user %q kept after the helper left", s.Username)
	}
}

// In the helper, the window's bindings go to the service.
func TestHelperForwardsBindings(t *testing.T) {
	svcEnd, helperEnd := net.Pipe()
	service := agentsvc.NewLink(svcEnd, func(_ context.Context, name string, data json.RawMessage) (any, error) {
		var b agentsvc.Binding
		_ = json.Unmarshal(data, &b)
		switch b.Method {
		case "View":
			return View{Status: Status{State: StateConnected, Banner: "Aula — conectado a la clase de Prof (canal 1)"}}, nil
		case "Copy":
			return nil, errors.New("ese fragmento ya no está en pantalla")
		}
		return nil, nil
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.Run(ctx)
	a, _ := newTestApp(nil)
	a.remote = agentsvc.NewLink(helperEnd, nil, nil)
	go a.remote.Run(ctx)
	var media []bool
	a.onMedia = func(w, h bool) { media = append(media, w, h) }

	if v := a.View(); v.Status.State != StateConnected {
		t.Fatalf("forwarded view %+v", v)
	}
	if err := a.Copy("x"); err == nil || err.Error() != "ese fragmento ya no está en pantalla" {
		t.Fatalf("forwarded copy error %v", err)
	}
	a.ReportMedia(true, false)
	if !slices.Equal(media, []bool{true, false}) {
		t.Fatalf("media report %v", media)
	}
}

func TestServiceCommands(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runServiceCLI(nil, &out, &errOut); code != exitUsage {
		t.Fatalf("no command: %d", code)
	}
	if code := runServiceCLI([]string{"bailar"}, &out, &errOut); code != exitUsage {
		t.Fatalf("unknown command: %d", code)
	}
	// "ejecutar" outside the service manager refuses to run.
	if code := runServiceCLI([]string{"ejecutar"}, &out, &errOut); code != exitUsage {
		t.Fatalf("ejecutar from a terminal: %d", code)
	}
	out.Reset()
	code := runServiceCLI([]string{"estado", "-nombre", "AulaNoExiste" + strconv.Itoa(os.Getpid())}, &out, &errOut)
	if runtime.GOOS == "windows" {
		if code != exitNotInstalled || !strings.Contains(out.String(), "no instalado") {
			t.Fatalf("estado of a missing service: %d %q", code, out.String())
		}
	} else if code != exitFailure {
		t.Fatalf("estado on %s: %d", runtime.GOOS, code)
	}
	st := describeStatus("AulaAgente", platform.ServiceStatus{State: platform.ServiceRunning, PID: 42, Auto: true, RestartOnFailure: true})
	if st != "Servicio AulaAgente: en ejecución (PID 42); arranca con el equipo; se reinicia si falla" {
		t.Fatalf("describe %q", st)
	}
}

type fakeProcs struct {
	mu     sync.Mutex
	procs  []platform.ProcessInfo
	killed []string
}

func (f *fakeProcs) List(uint32) ([]platform.ProcessInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.procs), nil
}

func (f *fakeProcs) Kill(pid int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, p := range f.procs {
		if p.PID == pid {
			f.killed = append(f.killed, p.Name)
			f.procs = slices.Delete(f.procs, i, i+1)
		}
	}
	return nil
}

// The class restrictions reach the agent in the service only with a helper,
// the student is told before anything is closed, and they end with the
// helper.
func TestServiceAgentAppliesAndEndsRestrictions(t *testing.T) {
	c := startLiveConsole(t)
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: c.addr}
	pub, _ := cfg.Validate()
	sa := newServiceAgent(cfg, pub, nil, quiet)
	fp := &fakeProcs{procs: []platform.ProcessInfo{{PID: 7, Name: "steam.exe", Path: `C:\Games\steam.exe`}}}
	sa.policy = &policy.Manager{OnChange: sa.app.setRestrictions,
		Apps: &policy.AppEnforcer{Procs: fp, Session: sa.session, OnClosed: sa.app.appClosed}}
	sa.app.extra = sa.policy.Handle
	sa.app.onLeave = func() { _ = sa.policy.Clear("disconnected") }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sa.app.start(ctx)
	defer sa.app.stop()

	s := c.waitAgent(t, "agent without helper", func(session.Session) bool { return true })
	if s.Can(protocol.CapAppBlock) {
		t.Fatal("app.block declared without a helper to show the notice")
	}
	svcEnd, helperEnd := net.Pipe()
	served := make(chan struct{})
	go func() {
		defer close(served)
		sa.serve(ctx, platform.Session{ID: 1, User: "ana", SID: "S-1-5-21-1-1001"}, svcEnd)
	}()
	h := newFakeHelper(t, helperEnd)
	_ = h.link.Notify(agentsvc.EventReady, agentsvc.Ready{Capture: true, Clipboard: true})
	s = c.waitAgent(t, "agent with app.block", func(s session.Session) bool { return s.Can(protocol.CapAppBlock) })
	if s.Can(protocol.CapWebFilter) {
		t.Fatal("web.filter declared without its platform pieces")
	}

	set := protocol.AppBlockSet{PolicyID: protocol.NewID(), Mode: protocol.PolicyDeny, Apps: []string{"steam"}}
	if _, err := c.console.Send(ctx, s.ID, set); err != nil {
		t.Fatal(err)
	}
	var v View
	deadline := time.Now().Add(5 * time.Second)
	for (v.Restrictions.Apps == nil || !v.Status.Restricted) && time.Now().Before(deadline) {
		_ = h.binding(t, &v, "View")
		time.Sleep(20 * time.Millisecond)
	}
	if v.Restrictions.Apps == nil || v.Restrictions.Apps.Items[0] != "steam" || !strings.HasSuffix(v.Status.Banner, "· restricciones activas") {
		t.Fatalf("student's view %+v", v)
	}
	sa.policy.Apps.Enforce()
	fp.mu.Lock()
	killed := slices.Clone(fp.killed)
	fp.mu.Unlock()
	if !slices.Equal(killed, []string{"steam.exe"}) {
		t.Fatalf("closed %v", killed)
	}

	// The helper goes: the restrictions end with it.
	h.link.Close()
	<-served
	if st := sa.policy.State(); st.Active() {
		t.Fatalf("restrictions left after the helper left: %+v", st)
	}
	if v := sa.app.View(); v.Restrictions.Active() || v.Status.Restricted {
		t.Fatalf("notice left: %+v", v.Restrictions)
	}
}
