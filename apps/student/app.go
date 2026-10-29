package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

// maxNotices bounds the notices kept on screen.
const maxNotices = 10

// App is the student agent behind the Wails window. Its exported methods are
// the bindings the frontend calls.
type App struct {
	cfg    Config
	cfgErr error
	pub    ed25519.PublicKey
	hello  protocol.HelloInfo
	board  *codedist.Board
	recv   *codedist.Receiver
	clip   platform.Clipboard // nil when the system has none
	clipEr error
	log    *slog.Logger

	// UI hooks, set at startup (replaced in tests).
	emit  func(event string, data any)
	raise func() // bring the window to the front

	// reload re-reads the configuration files; nil when the configuration
	// came from command-line flags, which pin it. Checked every reloadEvery,
	// so a config.json written by the installer while the agent runs is
	// picked up without restarting it.
	reload      func() (Config, ed25519.PublicKey, error)
	reloadEvery time.Duration

	mu      sync.Mutex
	status  Status
	notices []codedist.Notice
	diag    string // why the agent is not connected (session.Agent.OnDiagnostic)
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewApp builds the agent. cfgErr is shown in the banner instead of
// connecting; clipErr explains why copying is unavailable.
func NewApp(cfg Config, pub ed25519.PublicKey, cfgErr error, hello protocol.HelloInfo,
	clip platform.Clipboard, clipErr error, log *slog.Logger) *App {
	a := &App{
		cfg: cfg, pub: pub, cfgErr: cfgErr, hello: hello,
		board: codedist.NewBoard(0), clip: clip, clipEr: clipErr, log: log,
		emit: func(string, any) {}, raise: func() {},
	}
	a.recv = &codedist.Receiver{Board: a.board, OnBoard: a.onBoard, OnNotice: a.onNotice}
	if cfgErr != nil {
		a.status = Status{State: StateUnconfigured, Channel: cfg.Channel, Banner: "Aula — sin configurar", Detail: cfgErr.Error()}
	} else {
		a.status = statusFor(StateSearching, cfg.Channel, "")
	}
	return a
}

// DefaultReloadEvery is how often the agent re-reads its configuration.
const DefaultReloadEvery = 3 * time.Second

// start runs the agent until stop is called. Without a valid configuration it
// only shows the banner, and keeps checking the configuration files.
func (a *App) start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel, a.done = cancel, make(chan struct{})
	go func() {
		defer close(a.done)
		a.supervise(ctx)
	}()
}

// supervise keeps one agent running with the current configuration and
// replaces it when the configuration changes. The old agent is fully stopped
// before a.cfg is touched, so the agent goroutine never sees it change.
func (a *App) supervise(ctx context.Context) {
	stopAgent := a.runAgent(ctx)
	defer func() { stopAgent() }()
	if a.reload == nil {
		<-ctx.Done()
		return
	}
	every := a.reloadEvery
	if every <= 0 {
		every = DefaultReloadEvery
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cfg, pub, err := a.reload()
			if cfg == a.cfg && errText(err) == errText(a.cfgErr) {
				continue
			}
			a.log.Info("configuration changed, restarting the agent", "channel", cfg.Channel, "valid", err == nil)
			stopAgent()
			a.cfg, a.pub, a.cfgErr = cfg, pub, err
			a.mu.Lock()
			a.diag = ""
			a.mu.Unlock()
			if err != nil {
				a.setStatus(Status{State: StateUnconfigured, Channel: cfg.Channel, Banner: "Aula — sin configurar", Detail: err.Error()})
			} else {
				a.setStatus(statusFor(StateSearching, cfg.Channel, ""))
			}
			stopAgent = a.runAgent(ctx)
		}
	}
}

// runAgent starts an agent for the current configuration, if valid, and
// returns a function that stops it and waits for it to finish.
func (a *App) runAgent(ctx context.Context) (stop func()) {
	if a.cfgErr != nil {
		return func() {}
	}
	agent := &session.Agent{
		Channel:          a.cfg.Channel,
		ConsolePublicKey: a.pub,
		Hello:            a.hello,
		Source:           a.source(),
		OnMessage:        a.onMessage,
		OnState:          a.onState,
		OnDiagnostic:     a.onDiagnostic,
		Logger:           a.log,
	}
	actx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	channel := a.cfg.Channel
	go func() {
		defer close(done)
		if err := agent.Run(actx); err != nil {
			a.log.Error("agent stopped", "err", err)
			a.setStatus(Status{State: StateUnconfigured, Channel: channel, Banner: "Aula — sin configurar", Detail: err.Error()})
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *App) stop() {
	if a.cancel != nil {
		a.cancel()
		<-a.done
	}
}

func (a *App) source() session.ConsoleSource {
	if a.cfg.Direct == "" {
		return &discovery.Browser{Channel: a.cfg.Channel, Logger: a.log}
	}
	host, portStr, err := net.SplitHostPort(a.cfg.Direct)
	port, perr := strconv.Atoi(portStr)
	ip := net.ParseIP(host)
	if err != nil || perr != nil || ip == nil {
		a.log.Warn("invalid direct address, using mDNS", "direct", a.cfg.Direct)
		return &discovery.Browser{Channel: a.cfg.Channel, Logger: a.log}
	}
	// No announced fingerprint: the console is still authenticated by its
	// signature (docs/protocol.md §10.2).
	return session.StaticSource{{Instance: "directo", Channel: a.cfg.Channel, Name: a.cfg.Direct, Addrs: []net.IP{ip}, Port: port}}
}

// --- Agent callbacks (agent goroutine) -------------------------------------------

func (a *App) onState(s session.AgentState, c discovery.Console) {
	var st Status
	switch s {
	case session.AgentConnected:
		st = statusFor(StateConnected, a.cfg.Channel, c.Name)
	case session.AgentDisconnected:
		st = statusFor(StateDisconnected, a.cfg.Channel, c.Name)
	default:
		st = statusFor(StateSearching, a.cfg.Channel, "")
	}
	a.mu.Lock()
	if st.State != StateConnected {
		st.Detail = a.diag
	}
	a.mu.Unlock()
	a.setStatus(st)
}

// onDiagnostic shows under the banner why the agent is not connected; "" when
// it connects. The same text goes to the log, for the administrator.
func (a *App) onDiagnostic(msg string) {
	if msg != "" {
		a.log.Warn("agent not connected", "reason", msg)
	}
	a.mu.Lock()
	a.diag = msg
	st := a.status
	changed := st.State != StateConnected && st.State != StateUnconfigured && st.Detail != msg
	if changed {
		st.Detail = msg
		a.status = st
	}
	a.mu.Unlock()
	if changed {
		a.emit(EventStatus, st)
	}
}

func (a *App) onMessage(ctx context.Context, cl *transport.Client, env protocol.Envelope) {
	a.log.Debug("message from console", "type", env.Type)
	if reply := a.recv.Handle(env); reply != nil {
		if _, err := cl.Send(ctx, reply); err != nil {
			a.log.Debug("cannot send reply", "type", reply.MessageType(), "err", err)
		}
	}
}

func (a *App) onBoard(snippets []codedist.Snippet) {
	if snippets == nil {
		snippets = []codedist.Snippet{}
	}
	a.emit(EventBoard, Board{Snippets: snippets})
	if len(snippets) > 0 {
		a.raise()
	}
}

func (a *App) onNotice(n codedist.Notice) {
	a.mu.Lock()
	a.notices = append(a.notices, n)
	if len(a.notices) > maxNotices {
		a.notices = slices.Clone(a.notices[len(a.notices)-maxNotices:])
	}
	list := slices.Clone(a.notices)
	a.mu.Unlock()
	a.emit(EventNotices, Notices{Notices: list})
	a.raise()
}

func (a *App) setStatus(s Status) {
	a.mu.Lock()
	a.status = s
	a.mu.Unlock()
	a.emit(EventStatus, s)
}

// statusFor builds the identity banner. The wording is fixed by the privacy
// rules: the student always sees whose class this machine is connected to.
func statusFor(state, channel, teacher string) Status {
	s := Status{State: state, Channel: channel, Teacher: teacher}
	switch state {
	case StateConnected:
		s.Banner = fmt.Sprintf("Aula — conectado a la clase de %s (canal %s)", teacher, channel)
	case StateDisconnected:
		s.Banner = fmt.Sprintf("Aula — sin conexión con la clase de %s (canal %s); reintentando", teacher, channel)
	default:
		s.Banner = fmt.Sprintf("Aula — buscando la clase del canal %s", channel)
	}
	return s
}

// --- Bindings (called by the frontend) -------------------------------------------

// View returns the whole state, for the first render.
func (a *App) View() View {
	a.mu.Lock()
	defer a.mu.Unlock()
	snippets := a.board.Snippets()
	if snippets == nil {
		snippets = []codedist.Snippet{}
	}
	return View{Status: a.status, Snippets: snippets, Notices: append([]codedist.Notice{}, a.notices...)}
}

// Copy puts a snippet on the system clipboard. The text comes from the Go
// copy of the snippet, not from the page, so what is pasted is exactly what
// the teacher sent (indentation and line breaks included).
func (a *App) Copy(snippetID string) error {
	s, ok := a.board.Get(snippetID)
	if !ok {
		return errors.New("ese fragmento ya no está en pantalla")
	}
	if a.clip == nil {
		return fmt.Errorf("el portapapeles no está disponible en este equipo: %v", a.clipEr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.clip.WriteText(ctx, s.Content); err != nil {
		a.log.Warn("copy failed", "snippet", snippetID, "err", err)
		return fmt.Errorf("no se pudo copiar: %v", err)
	}
	a.log.Info("snippet copied", "snippet", snippetID, "bytes", len(s.Content))
	return nil
}

// DismissNotice hides a notice.
func (a *App) DismissNotice(id string) {
	a.mu.Lock()
	a.notices = slices.DeleteFunc(a.notices, func(n codedist.Notice) bool { return n.ID == id })
	list := slices.Clone(a.notices)
	a.mu.Unlock()
	a.emit(EventNotices, Notices{Notices: list})
}
