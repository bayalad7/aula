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
	"github.com/bayalad7/aula/internal/classend"
	"github.com/bayalad7/aula/internal/classtimer"
	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/collect"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/openapp"
	"github.com/bayalad7/aula/internal/participation"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/policy"
	"github.com/bayalad7/aula/internal/poll"
	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/remote"
	"github.com/bayalad7/aula/internal/screen"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/snapshot"
	"github.com/bayalad7/aula/internal/transport"
)

// maxNotices bounds the notices kept on screen.
const maxNotices = 10

// App is the student agent behind the Wails window. Its exported methods are
// the bindings the frontend calls.
type App struct {
	cfg       Config
	cfgErr    error
	pub       ed25519.PublicKey
	hello     protocol.HelloInfo
	label     string // computer.label of this session (§5.26); guarded by mu
	board     *codedist.Board
	recv      *codedist.Receiver
	screen    *screen.Streamer        // screen thumbnails; inert until enableScreen
	freeze    *screen.Freezer         // the pause window (§5.9); inert until enableFreeze
	power     *power.Countdown        // power.*; inert until enablePower
	viewer    *broadcast.Viewer       // the teacher's screen; declared after ReportMedia
	present   *broadcast.Presenter    // this screen shown to the class (§5.24); inert until enablePresent
	badge     *broadcast.IndicatorMux // the native badge, shared by the screen stream and a presentation
	control   *remote.Agent           // remote control; inert until enableRemote
	snap      *snapshot.Taker         // still screenshots; inert until enableScreen
	desk      *participation.Desk     // the student's hand and messages to the teacher (§5.18)
	countdown *classtimer.Countdown   // the exercise timer (§5.19)
	ballot    *poll.Ballot            // the polls of the teacher (§5.23)
	files     *filepush.Receiver      // file.push; inert until enableFiles (§5.20)
	inbox     *filepush.Inbox         // what arrived, for the window
	collect   *collect.Responder      // file.collect; inert until enableCollect (§5.21)
	opener    *openapp.Opener         // open.*; inert until enableOpen (§5.22)
	work      platform.WorkFolder     // «Abrir carpeta»; nil without a work folder
	clip      platform.Clipboard      // nil when the system has none
	clipEr    error
	log       *slog.Logger

	// UI hooks, set at startup (replaced in tests).
	emit   func(event string, data any)
	raise  func() // bring the window to the front: power, restrictions, broadcast (window.go)
	notify func() // code or a notice arrived: tell without taking the focus (window.go)
	// compactHook and trayText act on this process's window (window.go);
	// nil until it exists.
	compactHook func(on bool) error
	trayText    func(text string)
	quitWindow  func() // before runtime.Quit: let the window close (window.go)
	// hold brings the window forward and keeps it up while the teacher
	// broadcasts (rule 12), in mode ("" is v0.4's maximized window); with on
	// false it puts the window back as it was (window.go).
	hold  func(on bool, mode protocol.BroadcastMode)
	onTop func(on bool) // keep the window above the others (power notice)

	// connOpts are the connection's timings; zero is §3.4's (tests shorten
	// them).
	connOpts transport.Options

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
	// endRestrictions removes the class restrictions with their journal when
	// the teacher ends the class (§5.25.1); nil in window mode, which has none.
	endRestrictions func() error

	// reload re-reads the configuration files; nil when the configuration
	// came from command-line flags, which pin it. Checked every reloadEvery,
	// so a config.json written by the installer while the agent runs is
	// picked up without restarting it.
	reload      func() (Config, ed25519.PublicKey, error)
	reloadEvery time.Duration

	mu         sync.Mutex
	status     Status
	notices    []codedist.Notice
	diag       string            // why the agent is not connected (session.Agent.OnDiagnostic)
	cause      session.Cause     // the same, classified (session.Agent.OnCause)
	viewed     bool              // the console is receiving this screen (docs/privacidad.md rule 6)
	restrict   policy.State      // class restrictions in force (rules 20-21)
	watching   bool              // this screen shows the teacher's broadcast (rule 12)
	presenting bool              // this screen is shown to the class (rules 75-82)
	paused     bool              // the teacher paused this screen (rules 9-10)
	powerV     PowerView         // power notice in force (rules 26-28)
	ctrl       remote.State      // remote control in force (rules 36-41)
	cl         *transport.Client // the connection with the console; nil while not connected
	opened     []openapp.Opened  // what the teacher opened, for the corner notices (rule 66)
	endCard    *classend.Card    // «La clase terminó», until dismissed or new code arrives (§5.25)

	timerMu sync.Mutex
	timerT  *time.Timer // the next tick of the countdown

	cancel context.CancelFunc
	done   chan struct{}
}

// NewApp builds the agent. cfgErr is shown in the banner instead of
// connecting; clipErr explains why copying is unavailable.
func NewApp(cfg Config, pub ed25519.PublicKey, cfgErr error, hello protocol.HelloInfo,
	clip platform.Clipboard, clipErr error, log *slog.Logger) *App {
	a := &App{
		cfg: cfg, pub: pub, cfgErr: cfgErr, hello: hello,
		board: codedist.NewBoard(0), desk: participation.NewDesk(), countdown: classtimer.NewCountdown(nil), ballot: poll.NewBallot(nil), clip: clip, clipEr: clipErr, log: log,
		emit: func(string, any) {}, raise: func() {}, notify: func() {}, hold: func(bool, protocol.BroadcastMode) {}, onTop: func(bool) {},
		mediaReady: make(chan struct{}), restart: make(chan struct{}, 1),
	}
	a.recv = &codedist.Receiver{Board: a.board, OnBoard: a.onBoard, OnNotice: a.onNotice}
	a.screen = &screen.Streamer{Logger: log, OnViewed: a.onViewed}
	a.freeze = &screen.Freezer{Who: a.pauseWho, OnPaused: a.onPaused, Logger: log}
	a.viewer = &broadcast.Viewer{Logger: log, OnView: a.onBroadcastView, OnSignal: a.onBroadcastSignal}
	a.badge = &broadcast.IndicatorMux{}
	a.present = &broadcast.Presenter{OnView: a.onPresent, Logger: log}
	a.power = &power.Countdown{OnChange: a.onPower, Logger: log}
	a.control = &remote.Agent{Video: a.screen, OnState: a.onControl, Logger: log}
	a.snap = &snapshot.Taker{Indicator: a.screen, Identity: a.identity, OnTaken: a.onSnapshot, Logger: log}
	a.files = &filepush.Receiver{OnFile: a.onFile, Logger: log}
	a.inbox = &filepush.Inbox{}
	a.collect = &collect.Responder{OnChange: a.onCollect, Logger: log}
	a.opener = &openapp.Opener{Who: a.who, OnOpened: a.onOpened, Logger: log}
	if cfgErr != nil {
		a.status = Status{State: StateUnconfigured, Channel: cfg.Channel, Detail: cfgErr.Error()}
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
			a.diag, a.cause = "", session.CauseNone
			a.mu.Unlock()
			if err != nil {
				a.setStatus(Status{State: StateUnconfigured, Channel: cfg.Channel, Detail: err.Error()})
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
		MACs:             agentMACs,
		Source:           a.source(),
		OnMessage:        a.onMessage,
		OnConnect:        a.onConnect,
		OnState:          a.onState,
		OnDiagnostic:     a.onDiagnostic,
		OnCause:          a.onCause,
		Conn:             a.connOpts,
		Logger:           a.log,
	}
	actx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	channel := a.cfg.Channel
	go func() {
		defer close(done)
		if err := agent.Run(actx); err != nil {
			a.log.Error("agent stopped", "err", err)
			a.setStatus(Status{State: StateUnconfigured, Channel: channel, Detail: err.Error()})
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
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.hello
}

// capabilities is what this computer lets the teacher do, as the window shows
// it in «Qué puede hacer Aula en este equipo» (nota de interfaz 13): the
// capabilities of the hello, never empty (null) for the window.
func (a *App) capabilities() []string {
	out := []string{}
	for _, c := range a.currentHello().Capabilities {
		out = append(out, string(c))
	}
	return out
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
	a.control.Stop()
	a.screen.Stop()
	a.freeze.Stop() // nobody stays paused when the agent ends (rule 10)
	a.viewer.Stop()
	a.present.Stop()
	a.snap.Wait() // its connection is closed: it drops the image
	a.files.Wait()
	a.collect.Wait() // its connection is closed: the round is abandoned
	a.opener.Wait()
	a.stopTimer()
}

// enableScreen lets the console see this screen in thumbnails and take a
// still screenshot of it, with the always-visible indicator, and declares
// screen.thumb and screen.snapshot in hello. Call it before start; without
// both pieces the agent declares neither.
func (a *App) enableScreen(c platform.ScreenCapture, i platform.ScreenIndicator) {
	a.setBadge(i)
	a.screen.Capture = c
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), a.screen.Capabilities()...)
	if a.screen.Available() {
		a.snap.Capture = c
		a.hello.Capabilities = append(a.hello.Capabilities, a.snap.Capabilities()...)
	}
}

// setBadge shares the native badge between the screen stream and a
// presentation (whose text wins while it is live).
func (a *App) setBadge(i platform.ScreenIndicator) {
	if i == nil {
		return
	}
	a.badge.Indicator = i
	a.screen.Indicator = a.badge.Stream()
	a.present.Indicator = a.badge.Presentation()
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
		a.mu.Lock()
		a.cl = nil   // a raised hand waits, up, for the next connection
		a.label = "" // the label is only for the session (§5.26.1)
		a.mu.Unlock()
		// The streams belong to the connection: no console, no capture and
		// no broadcast view, and no restrictions.
		a.control.Stop() // first: it releases the keys and gives the stream back
		a.screen.Stop()
		a.freeze.Stop() // nobody stays paused without the class (rule 10)
		a.viewer.Stop()
		a.present.Stop()  // the band and the badge go; nothing is captured any more
		a.files.Reset()   // a file half received is dropped, never written
		a.collect.Reset() // a round half sent is abandoned; what was collected stays listed
		if a.onLeave != nil {
			a.onLeave()
		}
	}
	a.mu.Lock()
	if st.State != StateConnected {
		st.Detail, st.Cause = a.diag, string(a.cause)
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

// onCause keeps why the agent is not connected, classified, for the window
// (CauseNone once connected).
func (a *App) onCause(c session.Cause) {
	a.mu.Lock()
	a.cause = c
	st := a.status
	changed := st.State != StateConnected && st.State != StateUnconfigured && st.Cause != string(c)
	if changed {
		st.Cause = string(c)
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
	if a.handleParticipation(env) || a.handleTimer(ctx, cl, env) || a.handlePoll(ctx, cl, env) || a.handleClassEnd(ctx, cl, env) ||
		a.handleComputerLabel(env) {
		return
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
		reply, ok = a.files.Handle(ctx, filepush.Sender(send), env)
	}
	if !ok {
		reply, ok = a.collect.Handle(ctx, collect.Sender(send), env)
	}
	if !ok {
		reply, ok = a.opener.Handle(ctx, openapp.Sender(send), env)
	}
	if !ok {
		// Before the control: a screenshot does not end or pause it (§5.16).
		reply, ok = a.snap.Handle(ctx, snapshot.Sender(send), env)
	}
	if !ok {
		reply, ok = a.control.Handle(ctx, send, env)
	}
	if !ok {
		reply, ok = a.freeze.Handle(ctx, env)
	}
	if !ok {
		reply, ok = a.screen.Handle(ctx, send, env)
	}
	if !ok {
		reply, ok = a.viewer.Handle(ctx, send, env)
	}
	if !ok {
		reply, ok = a.present.Handle(ctx, send, env)
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
		// New code: the next class began, the «La clase terminó» card goes.
		a.mu.Lock()
		had := a.endCard != nil
		a.endCard = nil
		a.mu.Unlock()
		if had {
			a.emit(EventClassEnd, (*classend.Card)(nil))
		}
		a.notify() // never raise: the student keeps the keyboard (prompt 19)
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
	a.notify()
}

func (a *App) setStatus(s Status) {
	a.mu.Lock()
	s.Viewed = a.viewed && s.State == StateConnected
	s.Watching = a.watching && s.State == StateConnected
	s.Presenting = a.presenting && s.State == StateConnected
	s.Restricted = a.restrict.Active() && s.State == StateConnected
	s.Control = controlState(a.ctrl, s.State)
	s.Paused = pausedState(a.paused, s.State)
	a.status = s
	a.mu.Unlock()
	a.emit(EventStatus, s)
}

// statusFor builds the identity strip's state. The window words it (rule 1:
// the student always sees whose class this machine is connected to).
func statusFor(state, channel, teacher string) Status {
	return Status{State: state, Channel: channel, Teacher: teacher}
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
	caps, computer := a.capabilities(), a.computer()
	a.mu.Lock()
	defer a.mu.Unlock()
	snippets := a.board.Snippets()
	if snippets == nil {
		snippets = []codedist.Snippet{}
	}
	return View{Status: a.status, Snippets: snippets, Notices: append([]codedist.Notice{}, a.notices...),
		Broadcast: a.broadcastView(bc, a.status), Restrictions: a.restrict, Power: a.powerV,
		Participation: a.desk.View(), Timer: a.countdown.View(), Files: a.filesView(a.inbox.List()),
		Collections: a.collectionsView(), Opened: Opened{List: append([]openapp.Opened{}, a.opened...)},
		Polls: a.ballot.View(), Present: a.present.Current(), Capabilities: caps, ClassEnd: a.endCard,
		Computer: computer}
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

// SetCompact shrinks the window to a strip in a corner of the screen, or
// restores it (prompt 19). It acts on this process's window, also in the
// service's helper: it is never forwarded.
func (a *App) SetCompact(on bool) error {
	if a.compactHook == nil {
		return nil
	}
	return a.compactHook(on)
}

// ReportWindow receives the window's title, as the page wrote it, for the
// notification area icon's text. Local to this process, like SetCompact.
func (a *App) ReportWindow(title string) {
	if a.trayText != nil {
		a.trayText(title)
	}
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
