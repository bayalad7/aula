package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/policy"
	"github.com/bayalad7/aula/internal/protocol"
)

// The agent as a Windows service (D-016): the service keeps the connection
// with the console; a helper in the student's session shows the window and
// touches the screen and the clipboard (agentsvc).

// Service names. Tests install another name, never this one.
const (
	ServiceName        = "AulaAgente"
	serviceDisplayName = "Aula — agente del aula"
	serviceDescription = "Conecta este equipo con la consola del profesor de su canal. " +
		"Muestra en la sesión del alumno quién da la clase y cuándo se ve su pantalla."
)

// Exit codes of "servicio estado", for scripts and GPO.
const (
	exitRunning      = 0
	exitFailure      = 1
	exitUsage        = 2
	exitStopped      = 3
	exitNotInstalled = 4
)

// runServiceCLI runs "aula-cliente servicio instalar|desinstalar|iniciar|
// detener|estado|ejecutar [-nombre N]". It needs an administrator except for
// estado.
func runServiceCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "uso: aula-cliente servicio instalar|desinstalar|iniciar|detener|estado [-nombre N]")
		return exitUsage
	}
	fs := flag.NewFlagSet("servicio "+args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	name := fs.String("nombre", ServiceName, "nombre del servicio (las pruebas usan otro)")
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}
	ctl := platform.NewServiceControl(*name)
	fail := func(what string, err error) int {
		fmt.Fprintf(errOut, "%s: %v\n", what, err)
		if errors.Is(err, platform.ErrNotSupported) {
			fmt.Fprintln(errOut, "El agente como servicio solo existe en Windows por ahora.")
		} else if isAccessDenied(err) {
			fmt.Fprintln(errOut, "Ejecuta la orden como administrador.")
		}
		return exitFailure
	}
	switch args[0] {
	case "instalar":
		exe, err := os.Executable()
		if err != nil {
			return fail("no se encontró el ejecutable", err)
		}
		if err := ctl.Install(platform.ServiceConfig{Name: *name, DisplayName: serviceDisplayName,
			Description: serviceDescription, Exe: exe, Args: []string{"servicio", "ejecutar", "-nombre", *name}}); err != nil {
			return fail("no se pudo instalar el servicio", err)
		}
		fmt.Fprintf(out, "Servicio %s instalado: arranca con el equipo y se reinicia si falla.\n", *name)
	case "desinstalar":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if st, err := ctl.Status(); err == nil && st.State != platform.ServiceNotInstalled {
			if err := ctl.Stop(ctx); err != nil {
				return fail("no se pudo detener el servicio", err)
			}
		}
		// A web filter left by a crash is undone before the service goes
		// (docs/privacidad.md rule 24).
		if m := restoreOnlyManager(); m != nil {
			if err := m.Restore(); err != nil {
				fmt.Fprintf(errOut, "aviso: no se pudo deshacer del todo un filtro web anterior: %v\n", err)
			}
		}
		if err := ctl.Uninstall(); err != nil {
			return fail("no se pudo desinstalar el servicio", err)
		}
		fmt.Fprintf(out, "Servicio %s desinstalado.\n", *name)
	case "iniciar":
		if err := ctl.Start(); err != nil {
			return fail("no se pudo iniciar el servicio", err)
		}
		fmt.Fprintf(out, "Servicio %s iniciado.\n", *name)
	case "detener":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := ctl.Stop(ctx); err != nil {
			return fail("no se pudo detener el servicio", err)
		}
		fmt.Fprintf(out, "Servicio %s detenido.\n", *name)
	case "estado":
		st, err := ctl.Status()
		if err != nil {
			return fail("no se pudo consultar el servicio", err)
		}
		fmt.Fprintln(out, describeStatus(*name, st))
		switch st.State {
		case platform.ServiceRunning:
			return exitRunning
		case platform.ServiceNotInstalled:
			return exitNotInstalled
		default:
			return exitStopped
		}
	case "ejecutar":
		if is, err := platform.IsService(); err != nil || !is {
			fmt.Fprintln(errOut, "\"servicio ejecutar\" lo usa el administrador de servicios de Windows; para instalarlo usa \"servicio instalar\".")
			return exitUsage
		}
		if err := platform.RunAsService(*name, runService); err != nil {
			return fail("el servicio terminó con error", err)
		}
	default:
		fmt.Fprintf(errOut, "orden desconocida %q\n", args[0])
		return exitUsage
	}
	return exitRunning
}

func describeStatus(name string, st platform.ServiceStatus) string {
	s := fmt.Sprintf("Servicio %s: %s", name, st.State)
	if st.State == platform.ServiceNotInstalled {
		return s
	}
	if st.PID != 0 {
		s += fmt.Sprintf(" (PID %d)", st.PID)
	}
	if st.Auto {
		s += "; arranca con el equipo"
	} else {
		s += "; arranque manual"
	}
	if st.RestartOnFailure {
		s += "; se reinicia si falla"
	}
	return s
}

// isAccessDenied reports a missing administrator right (ERROR_ACCESS_DENIED
// on Windows maps to fs.ErrPermission).
func isAccessDenied(err error) bool { return errors.Is(err, fs.ErrPermission) }

// serviceRunning reports whether the agent service is running, so a second
// agent is not started in the user's session.
func serviceRunning() bool {
	st, err := platform.NewServiceControl(ServiceName).Status()
	return err == nil && st.State == platform.ServiceRunning
}

// runService is the agent in the service: the connection with the console,
// the supervisor of the helper and the channel with it. It returns when the
// service manager stops the service.
func runService(ctx context.Context, sessionChanged <-chan struct{}) error {
	log := openServiceLog()
	log.Info("aula-cliente service starting", "version", version)
	sessions, err := platform.NewSessionManager()
	if err != nil {
		return fmt.Errorf("sessions: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	cfg, pub, cfgErr := loadConfig(nil)
	if cfgErr != nil {
		log.Warn("agent not configured", "err", cfgErr)
	}
	sa := newServiceAgent(cfg, pub, cfgErr, log)
	sa.policy = newPolicyManager(exe, sa, log)
	if p, err := platform.NewPower(); err == nil {
		sa.app.power.Power = p // also without a helper (docs/privacidad.md rule 29)
	} else {
		log.Info("power actions not available", "err", err)
	}
	// A web filter left by a crash or a power cut is undone before
	// connecting (D-017).
	if err := sa.policy.Restore(); err != nil {
		log.Warn("restrictions of a previous run not fully restored", "err", err)
	}
	if sa.policy.Apps != nil {
		go sa.policy.Apps.Run(ctx)
	}
	sa.app.extra = sa.policy.Handle
	sa.app.onLeave = func() { _ = sa.policy.Clear("disconnected") }
	defer func() { _ = sa.policy.Clear("service stopping") }()
	sa.app.reload = func() (Config, ed25519.PublicKey, error) { return loadConfig(nil) }
	sa.app.start(ctx)
	defer sa.app.stop()

	sup := &agentsvc.Supervisor{
		Sessions: sessions, Exe: exe,
		Listen: platform.ListenLocal, PeerPID: platform.PeerPID,
		Serve:  sa.serve,
		Logger: log,
	}
	err = sup.Run(ctx, sessionChanged)
	log.Info("aula-cliente service stopping")
	return err
}

// newPolicyManager builds the class restrictions with the pieces this OS has;
// without them the agent does not declare the capabilities.
func newPolicyManager(exe string, sa *serviceAgent, log *slog.Logger) *policy.Manager {
	m := &policy.Manager{
		Journal:  policyJournal(),
		OnChange: sa.app.setRestrictions,
		Logger:   log,
	}
	if procs, err := platform.NewProcessControl(); err == nil {
		m.Apps = &policy.AppEnforcer{
			Procs: procs,
			Protect: policy.Protection{SystemRoot: os.Getenv("SystemRoot"),
				Keep: []string{filepath.Base(exe), "msedgewebview2", "explorer"}},
			Session:  sa.session,
			OnClosed: sa.app.appClosed,
			Logger:   log,
		}
	}
	dnsR, derr := platform.NewDNSRedirect()
	doh, berr := platform.NewBrowserDoH()
	if derr == nil && berr == nil {
		m.DNS, m.DoH = dnsR, doh
	}
	return m
}

// policyJournal is C:\ProgramData\Aula\alumno\politica-activa.json.
func policyJournal() policy.Journal {
	return policy.Journal{Path: filepath.Join(filepath.Dir(platform.AgentMachineConfigPath()), policy.JournalName)}
}

// restoreOnlyManager can only undo a web filter (uninstall); nil where the OS
// has none.
func restoreOnlyManager() *policy.Manager {
	dnsR, derr := platform.NewDNSRedirect()
	doh, berr := platform.NewBrowserDoH()
	if derr != nil || berr != nil {
		return nil
	}
	return &policy.Manager{DNS: dnsR, DoH: doh, Journal: policyJournal()}
}

// serviceAgent is the App of the service plus the state of its helper.
type serviceAgent struct {
	app    *App
	helper *agentsvc.Current
	base   protocol.HelloInfo
	policy *policy.Manager // the class restrictions

	mu    sync.Mutex
	sess  platform.Session
	ready *agentsvc.Ready // nil without a helper
}

func newServiceAgent(cfg Config, pub ed25519.PublicKey, cfgErr error, log *slog.Logger) *serviceAgent {
	sa := &serviceAgent{helper: &agentsvc.Current{}, base: localHello()}
	sa.base.Username = "" // the service runs as SYSTEM: the user is the session's
	app := NewApp(cfg, pub, cfgErr, sa.base, agentsvc.Clipboard{Helper: sa.helper}, nil, log)
	app.screen.Capture = agentsvc.Capture{Helper: sa.helper}
	app.screen.Indicator = agentsvc.Indicator{Helper: sa.helper}
	app.emit = func(event string, data any) {
		sa.helper.Notify(agentsvc.EventEmit, emitted{Event: event, Data: data})
	}
	app.raise = func() { sa.helper.Notify(agentsvc.EventRaise, nil) }
	app.maximize = func(on bool) { sa.helper.Notify(agentsvc.EventMaximize, on) }
	app.onTop = func(on bool) { sa.helper.Notify(agentsvc.EventOnTop, on) }
	app.power.Session = sa.session
	app.power.InSession = agentsvc.Power{Helper: sa.helper}
	app.helloFor = sa.hello
	app.beforeRestart = sa.apply
	sa.app = app
	return sa
}

// emitted is an App event forwarded to the helper's window.
type emitted struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// hello is the hello of the next connection: the session's user and the
// capabilities its helper can honour; without a helper only power (no window,
// no screen, nobody's work to lose: docs/privacidad.md rules 17 and 29).
func (sa *serviceAgent) hello() protocol.HelloInfo {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	h := sa.base
	h.Capabilities = nil
	if sa.app.power.Supported() {
		h.Capabilities = append(h.Capabilities, protocol.CapPower)
	}
	if sa.ready == nil {
		return h
	}
	h.Username = sa.sess.User
	h.Capabilities = append(h.Capabilities, protocol.CapCodeView, protocol.CapMsgShow)
	if sa.ready.Capture {
		h.Capabilities = append(h.Capabilities, protocol.CapScreenThumb)
	}
	if sa.ready.Broadcast {
		h.Capabilities = append(h.Capabilities, protocol.CapScreenBroadcast)
	}
	if sa.policy != nil {
		// Only with a helper: it shows the notice (docs/privacidad.md rule 20).
		h.Capabilities = append(h.Capabilities, sa.policy.Capabilities()...)
	}
	return h
}

// session is the student's session while a helper is ready in it.
func (sa *serviceAgent) session() (uint32, bool) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	return sa.sess.ID, sa.ready != nil
}

// apply runs with the agent stopped, before it reconnects.
func (sa *serviceAgent) apply() {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	sa.app.viewer.Supported = sa.ready != nil && sa.ready.Broadcast
}

// serve runs the channel with one helper until it ends.
func (sa *serviceAgent) serve(ctx context.Context, s platform.Session, conn net.Conn) {
	var link *agentsvc.Link
	link = agentsvc.NewLink(conn, sa.binding, func(name string, data json.RawMessage) {
		if name != agentsvc.EventReady {
			return
		}
		var r agentsvc.Ready
		if err := json.Unmarshal(data, &r); err != nil {
			return
		}
		sa.app.log.Info("helper ready", "session", s.ID, "user", s.User, "capture", r.Capture, "broadcast", r.Broadcast)
		sa.mu.Lock()
		sa.sess, sa.ready = s, &r
		sa.mu.Unlock()
		sa.helper.Set(link)
		sa.app.requestRestart()
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = link.Run(ctx)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		link.Close()
		<-done
	}
	sa.helper.Set(nil)
	sa.mu.Lock()
	had := sa.ready != nil
	sa.ready = nil
	sa.mu.Unlock()
	if had {
		sa.app.log.Info("helper gone", "session", s.ID)
		// Nothing can be shown any more: the screen stream and the broadcast
		// end, and the agent reconnects without capabilities.
		sa.app.screen.Stop()
		sa.app.viewer.Stop()
		if sa.policy != nil {
			_ = sa.policy.Clear("helper gone")
		}
		sa.app.requestRestart()
	}
}

// binding answers a call of the helper's window with the App's method.
func (sa *serviceAgent) binding(_ context.Context, name string, data json.RawMessage) (any, error) {
	if name != agentsvc.CallBinding {
		return nil, fmt.Errorf("%s: unknown call", name)
	}
	var b agentsvc.Binding
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	a := sa.app
	var (
		id, s1, s2    string
		attempt, line int
	)
	switch b.Method {
	case "View":
		return a.View(), nil
	case "Copy":
		if err := b.Arg(0, &id); err != nil {
			return nil, err
		}
		return nil, a.Copy(id)
	case "DismissNotice":
		if err := b.Arg(0, &id); err != nil {
			return nil, err
		}
		a.DismissNotice(id)
		return nil, nil
	case "BroadcastAnswer":
		if err := errors.Join(b.Arg(0, &id), b.Arg(1, &attempt), b.Arg(2, &s1)); err != nil {
			return nil, err
		}
		return nil, a.BroadcastAnswer(id, attempt, s1)
	case "BroadcastCandidate":
		if err := errors.Join(b.Arg(0, &id), b.Arg(1, &attempt), b.Arg(2, &s1), b.Arg(3, &s2), b.Arg(4, &line)); err != nil {
			return nil, err
		}
		return nil, a.BroadcastCandidate(id, attempt, s1, s2, line)
	case "BroadcastFailed":
		if err := errors.Join(b.Arg(0, &id), b.Arg(1, &attempt)); err != nil {
			return nil, err
		}
		return nil, a.BroadcastFailed(id, attempt)
	case "PowerPostpone":
		return nil, a.PowerPostpone()
	case "DismissPower":
		a.DismissPower()
		return nil, nil
	}
	return nil, fmt.Errorf("%s: unknown method", b.Method)
}

// openServiceLog logs to C:\ProgramData\Aula\alumno\logs\aula-agente.log: the
// service runs as SYSTEM, which has no user profile to log into.
func openServiceLog() *slog.Logger {
	f, _, err := platform.OpenLogFileIn(platform.AgentServiceLogDir(), "aula-agente.log")
	if err != nil {
		return slog.Default()
	}
	return slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
