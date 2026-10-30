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

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/policy"
	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/screen"
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
	screen *screen.Streamer   // screen thumbnails; inert until enableScreen
	power  *power.Countdown   // power.*; inert until enablePower
	viewer *broadcast.Viewer  // the teacher's screen; declared after ReportMedia
	clip   platform.Clipboard // nil when the system has none
	clipEr error
	log    *slog.Logger

	// UI hooks, set at startup (replaced in tests).
	emit     func(event string, data any)
	raise    func()        // bring the window to the front
	maximize func(on bool) // maximize (broadcast) or restore the window
	onTop    func(on bool) // keep the window above the others (power notice)

	// The frontend reports whether it can play the broadcast; start waits up
	// to mediaWait for it (0: do not wait) before declaring the capability.
	mediaReady chan struct{}
	mediaOnce  sync.Once
	mediaWait  time.Duration

	// Service mode (D-016): helloFor gives the hello of each connection (the
	// session's user and what its helper can do), and a value on restart
	// reconnects with a new hello; beforeRestart runs with the agent stopped.
	helloFor      func() protocol.HelloInfo
	beforeRestart func()
	restart       chan struct{}
	// Helper mode: the window's bindings go to the agent in the service, and
	// ReportMedia goes to onMedia.
	remote  *agentsvc.Link
	onMedia func(webrtc, h264 bool)
	// Service mode: extra handles messages before the others (the class
	// restrictions) and onLeave runs when the connection with the console
	// ends (they end with it, docs/privacidad.md rule 24).
	extra   func(env protocol.Envelope) (protocol.Payload, bool)
	onLeave func()

	// reload re-reads the configuration files; nil when the configuration
	// came from command-line flags, which pin it. Checked every reloadEvery,
	// so a config.json written by the installer while the agent runs is
	// picked up without restarting it.
	reload      func() (Config, ed25519.PublicKey, error)
	reloadEvery time.Duration

	mu       sync.Mutex
	status   Status
	notices  []codedist.Notice
	diag     string       // why the agent is not connected (session.Agent.OnDiagnostic)
	viewed   bool         // the console is receiving this screen (docs/privacidad.md rule 6)
	restrict policy.State // class restrictions in force (rules 20-21)
	watching bool         // this screen shows the teacher's broadcast (rule 12)
	powerV   PowerView    // power notice in force (rules 26-28)
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewApp builds the agent. cfgErr is shown in the banner instead of
// connecting; clipErr explains why copying is unavailable.
func NewApp(cfg Config, pub ed25519.PublicKey, cfgErr error, hello protocol.HelloInfo,
	clip platform.Clipboard, clipErr error, log *slog.Logger) *App {
	a := &App{
		cfg: cfg, pub: pub, cfgErr: cfgErr, hello: hello,
		board: codedist.NewBoard(0), clip: clip, clipEr: clipErr, log: log,
		emit: func(string, any) {}, raise: func() {}, maximize: func(bool) {}, onTop: func(bool) {},
		mediaReady: make(chan struct{}), restart: make(chan struct{}, 1),
	}
	a.recv = &codedist.Receiver{Board: a.board, OnBoard: a.onBoard, OnNotice: a.onNotice}
	a.screen = &screen.Streamer{Logger: log, OnViewed: a.onViewed}
	a.viewer = &broadcast.Viewer{Logger: log, OnView: a.onBroadcastView, OnSignal: a.onBroadcastSignal}
	a.power = &power.Countdown{OnChange: a.onPower, Logger: log}
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
		a.awaitMedia(ctx)
		a.supervise(ctx)
	}()
}

// supervise keeps one agent running with the current configuration and
// replaces it when the configuration changes. The old agent is fully stopped
// before a.cfg is touched, so the agent goroutine never sees it change.
func (a *App) supervise(ctx context.Context) {
	stopAgent := a.runAgent(ctx)
	defer func() { stopAgent() }()
	var tick <-chan time.Time
	if a.reload != nil {
		every := a.reloadEvery
		if every <= 0 {
			every = DefaultReloadEvery
		}
		t := time.NewTicker(every)
		defer t.Stop()
		tick = t.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.restart:
			a.log.Info("helper changed, reconnecting with a new hello")
			stopAgent()
			if a.beforeRestart != nil {
				a.beforeRestart()
			}
			stopAgent = a.runAgent(ctx)
		case <-tick:
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
		Hello:            a.currentHello(),
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

// currentHello is the hello of the next connection.
func (a *App) currentHello() protocol.HelloInfo {
	if a.helloFor != nil {
		return a.helloFor()
	}
	return a.hello
}

// requestRestart asks supervise to reconnect with a new hello.
func (a *App) requestRestart() {
	select {
	case a.restart <- struct{}{}:
	default: // one pending is enough
	}
}

// forward sends a binding of the helper's window to the agent in the
// service and decodes its result into out (if not nil).
func (a *App) forward(method string, out any, args ...any) error {
	b, err := agentsvc.NewBinding(method, args...)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.remote.Call(ctx, agentsvc.CallBinding, b, out); err != nil {
		return errors.New(err.Error()) // the agent's own (Spanish) message
	}
	return nil
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
	a.screen.Stop()
	a.viewer.Stop()
}

// enableScreen lets the console see this screen in thumbnails, with the
// always-visible indicator, and declares screen.thumb in hello. Call it
// before start; without both pieces the agent does not declare it.
func (a *App) enableScreen(c platform.ScreenCapture, i platform.ScreenIndicator) {
	a.screen.Capture, a.screen.Indicator = c, i
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), a.screen.Capabilities()...)
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
	if s != session.AgentConnected {
		// The streams belong to the connection: no console, no capture and
		// no broadcast view, and no restrictions.
		a.screen.Stop()
		a.viewer.Stop()
		if a.onLeave != nil {
			a.onLeave()
		}
	}
	a.mu.Lock()
	if st.State != StateConnected {
		st.Detail = a.diag
	}
	a.mu.Unlock()
	a.setStatus(st)
}

// onViewed changes the banner while the console receives this screen.
func (a *App) onViewed(viewed bool) {
	a.mu.Lock()
	a.viewed = viewed
	st := a.status
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
	send := func(ctx context.Context, p protocol.Payload) error {
		_, err := cl.Send(ctx, p)
		return err
	}
	var reply protocol.Payload
	ok := false
	if a.extra != nil {
		reply, ok = a.extra(env)
	}
	if !ok {
		// Only after auth.ok: the session rejects everything earlier.
		reply, ok = a.power.Handle(ctx, power.Sender(send), env)
	}
	if !ok {
		reply, ok = a.screen.Handle(ctx, send, env)
	}
	if !ok {
		reply, ok = a.viewer.Handle(ctx, send, env)
	}
	if ok {
		if reply != nil {
			if err := send(ctx, reply); err != nil {
				a.log.Debug("cannot send reply", "type", reply.MessageType(), "err", err)
			}
		}
		return
	}
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
	s.Viewed = a.viewed && s.State == StateConnected
	s.Watching = a.watching && s.State == StateConnected
	s.Restricted = a.restrict.Active() && s.State == StateConnected
	if s.Viewed {
		s.Banner = fmt.Sprintf("Aula — tu pantalla está siendo vista por %s (canal %s)", s.Teacher, s.Channel)
	} else if s.Watching {
		s.Banner = fmt.Sprintf("Aula — viendo la pantalla de %s (canal %s)", s.Teacher, s.Channel)
	} else if s.State == StateConnected {
		s.Banner = statusFor(s.State, s.Channel, s.Teacher).Banner
	}
	if s.Restricted {
		s.Banner += " · restricciones activas"
	}
	if pending := powerPending(a.powerV.Notice); pending != "" && s.State != StateUnconfigured {
		s.Banner += " · " + pending
	}
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
	if a.remote != nil {
		var v View
		if err := a.forward("View", &v); err != nil {
			a.log.Warn("view from the service", "err", err)
		}
		return v
	}
	bc := a.viewer.Current() // before a.mu: the viewer calls into the app with its lock held
	a.mu.Lock()
	defer a.mu.Unlock()
	snippets := a.board.Snippets()
	if snippets == nil {
		snippets = []codedist.Snippet{}
	}
	return View{Status: a.status, Snippets: snippets, Notices: append([]codedist.Notice{}, a.notices...),
		Broadcast: a.broadcastView(bc, a.status), Restrictions: a.restrict, Power: a.powerV}
}

// Copy puts a snippet on the system clipboard. The text comes from the Go
// copy of the snippet, not from the page, so what is pasted is exactly what
// the teacher sent (indentation and line breaks included).
func (a *App) Copy(snippetID string) error {
	if a.remote != nil {
		return a.forward("Copy", nil, snippetID)
	}
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
	if a.remote != nil {
		_ = a.forward("DismissNotice", nil, id)
		return
	}
	a.mu.Lock()
	a.notices = slices.DeleteFunc(a.notices, func(n codedist.Notice) bool { return n.ID == id })
	list := slices.Clone(a.notices)
	a.mu.Unlock()
	a.emit(EventNotices, Notices{Notices: list})
}

// setRestrictions shows the class restrictions (docs/privacidad.md rules
// 20-21): the window comes forward when they start, and the banner says so.
func (a *App) setRestrictions(r policy.State) {
	a.mu.Lock()
	was := a.restrict.Active()
	a.restrict = r
	st := a.status
	a.mu.Unlock()
	a.setStatus(st)
	a.emit(EventRestrictions, r)
	if r.Active() && !was {
		a.raise()
	}
}

// appClosed tells the student which app was closed and why (rule 22).
func (a *App) appClosed(name string) {
	a.emit(EventAppClosed, AppClosed{App: name})
}
