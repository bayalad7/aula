package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/classend"
	"github.com/bayalad7/aula/internal/classtimer"
	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/collect"
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
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/simclass"
	"github.com/bayalad7/aula/internal/snapshot"
)

var errNoClass = codedf(codeNoClass, "primero abre una clase")

// version is the console's SemVer. Releases set it with
// -ldflags "-X main.version=x.y.z".
var version = "0.1.0-dev"

// App is the teacher console behind the Wails window. Its exported methods
// are the bindings the frontend calls; all state lives here.
type App struct {
	dir        string // user settings, certificate and the personal identity
	siteKeyDir string // the site key (clave del centro), platform.SiteKeyDir()
	port       int
	// defaultChannel and defaultName open the class on start when autoStart
	// is set (the real app; tests open classes themselves).
	defaultChannel string
	defaultName    string
	autoStart      bool
	log            *slog.Logger
	clip           platform.Clipboard // nil when the system has none
	clipErr        error
	hotkeys        platform.Hotkeys
	start          startClassFunc
	now            func() time.Time

	emit  func(event string, data any) // set at startup; replaced in tests
	raise func()                       // bring the window forward (second launch); set at startup

	mu         sync.Mutex
	ctx        context.Context
	class      classroom
	info       ClassInfo
	startErr   string // why the automatic start failed, shown on the setup screen
	history    *codedist.History
	deliveries *codedist.Deliveries
	payloads   map[string]protocol.Payload // what each recent delivery sent, for Resend
	lastLang   string
	hotkey     HotkeyStatus
	wall       *screen.Wall // screens view: who streams and their last frame

	// Broadcast of the teacher's screen. bcMu orders start and stop;
	// exclude hides the console from captures (replaced in tests).
	bc               *broadcast.Broadcaster
	bcMu             sync.Mutex
	exclude          func(bool) error
	broadcastUDPPort int       // ws port + 1 (docs/protocol.md §5.12)
	bcClass          classroom // the class the broadcast goes to; guarded by mu
	// A student's screen shown to the class (§5.24): one at a time, and
	// never with the teacher's broadcast (same view, same port).
	proj      *broadcast.Projector
	presClass classroom // the class the presentation goes to; guarded by mu

	restr  *policy.ClassPolicy  // class restrictions: who has what (D-017)
	power  *power.Tracker       // power orders and each agent's answer (§5.15)
	wake   wakeState            // known computers and who is being turned on (§5.15.2)
	pauses *screen.Pauses       // paused screens and each agent's answer (§5.9)
	opens  *openapp.Tracker     // pages and apps opened, and each agent's answer (§5.22)
	desk   *remote.Desk         // remote control: one computer at a time (§5.16)
	snaps  *snapshot.Store      // screenshots of students, in memory until saved (§5.17)
	board  *participation.Board // raised hands and students' messages, in memory (§5.18)
	timer  *classtimer.Class    // the exercise timer of the class (§5.19)
	polls  *poll.Class          // the live polls of the class (§5.23)
	// files and pusher send the teacher's files (§5.20); chooseFiles opens
	// the system dialog (replaced in tests: never a window on the desktop).
	files       fileState
	pusher      *filepush.Pusher
	chooseFiles func(title string) ([]string, error)
	// collector and coll collect the students' work (§5.21); chooseFolder
	// opens the system dialog of «Guardar como…» (replaced in tests).
	collector    *collect.Collector
	coll         collectState
	chooseFolder func(title string) (string, error)
	// timerEnd tells the window when the timer reaches zero; guarded by mu.
	timerEnd *time.Timer
	// quizPoll and quizTimerID are the open quiz and the timer that closes
	// it at zero (§5.23.1); guarded by mu.
	quizPoll, quizTimerID string
	// documents is the teacher's Documents folder and openFolder shows a
	// folder in Explorer (replaced in tests: never a window on the desktop).
	documents  func() (string, error)
	openFolder func(string) error
	sim        *simclass.Class // the aula simulada's students; nil while closed (guarded by mu)

	agentErrs agentErrors // the last error each student reported

	// ends is «Terminar la clase» (§5.25): the steps, each student's answer
	// and who is owed a class.end when they reconnect. endMu makes sending a
	// class.end and recording it atomic with respect to its ack.
	ends           *classend.Class
	endMu          sync.Mutex
	endCollectWait time.Duration // 0: classend.CollectWait (tests shorten it)

	checks *connectChecks // the connection checker (CheckConnection)

	// sendMu makes a broadcast and the registration of its envelope IDs atomic
	// with respect to acks, so a fast agent's ack is never counted as unknown.
	sendMu sync.Mutex
}

// NewApp builds the console. dir holds the identity and settings.
func NewApp(dir string, clip platform.Clipboard, clipErr error, hotkeys platform.Hotkeys, log *slog.Logger) *App {
	m, s := loadMachineConfig(), loadSettings(dir)
	a := &App{
		dir: dir, siteKeyDir: siteKeyDir(), port: serverPort(m, s),
		defaultChannel: defaultChannel(m, s), defaultName: s.ConsoleName,
		log: log, clip: clip, clipErr: clipErr, hotkeys: hotkeys,
		start: startLiveClass, now: time.Now, emit: func(string, any) {}, raise: func() {},
		history: &codedist.History{}, deliveries: &codedist.Deliveries{},
		hotkey:  HotkeyStatus{Combo: platform.DefaultCaptureHotkey.String()},
		wall:    screen.NewWall(),
		exclude: platform.ExcludeFromCapture,
	}
	a.broadcastUDPPort = a.port + 1
	a.restr = policy.NewClassPolicy()
	a.power = &power.Tracker{Now: func() time.Time { return a.now() }}
	a.wake = newWakeState(dir, log)
	a.pauses = &screen.Pauses{}
	a.opens = &openapp.Tracker{}
	a.desk = &remote.Desk{Now: func() time.Time { return a.now() }}
	a.snaps = &snapshot.Store{Now: func() time.Time { return a.now() }, Logger: log,
		OnChange: func(l []snapshot.Snapshot) { a.emit(EventSnapshots, Snapshots{Snapshots: l}) }}
	a.board = participation.NewBoard()
	a.timer = classtimer.NewClass(nil)
	a.polls = poll.NewClass(nil)
	a.pusher = &filepush.Pusher{Send: a.sendToSession, OnOutcome: a.onFileOutcome, Logger: log}
	a.chooseFiles = a.openFilesDialog
	a.collector = &collect.Collector{Send: a.sendToSession, Root: a.collectRoot, Logger: log,
		OnChange: func(string) { a.emitCollections() }}
	a.chooseFolder = a.openFolderDialog
	a.documents, a.openFolder = platform.DocumentsDir, platform.OpenFolder
	a.bc = a.newBroadcaster(broadcastKbps(m))
	a.proj = a.newProjector(broadcastKbps(m))
	a.checks = newConnectChecks(log)
	a.ends = classend.NewClass(func() time.Time { return a.now() })
	return a
}

// startup registers the capture hotkey for the lifetime of ctx.
func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
	err := a.hotkeys.Register(ctx, platform.DefaultCaptureHotkey, func() { _, _ = a.capture("hotkey") })
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case err == nil:
		a.hotkey.Active = true
	case errors.Is(err, platform.ErrHotkeyInUse):
		a.hotkey.Reason = hotkeyInUse
	case errors.Is(err, platform.ErrNotSupported):
		a.hotkey.Reason = hotkeyNotSupported
	default:
		a.hotkey.Error = err.Error()
	}
	if err != nil {
		a.log.Warn("capture hotkey unavailable", "err", err)
	}
	if a.autoStart {
		// Like LanSchool: open on the default channel right away.
		go a.startDefault()
	}
}

// startDefault opens the class on the default channel. A failure (port in
// use, no key…) is shown on the setup screen, where the teacher can retry.
func (a *App) startDefault() {
	if _, err := a.StartClass(a.defaultChannel, a.defaultName); err != nil {
		a.log.Warn("automatic start failed", "channel", a.defaultChannel, "err", err)
		a.mu.Lock()
		a.startErr = err.Error()
		a.mu.Unlock()
		a.emit(EventClass, a.View())
		return
	}
	a.emit(EventClass, a.View())
}

func (a *App) shutdown(context.Context) { a.EndClass() }

// --- Class lifecycle ----------------------------------------------------------

// Setup returns what the channel selection screen shows.
func (a *App) Setup() Setup {
	s := loadSettings(a.dir)
	name := s.ConsoleName
	if name == "" {
		name = a.defaultName
	}
	a.mu.Lock()
	startErr := a.startErr
	a.mu.Unlock()
	out := Setup{RecentChannels: s.RecentChannels, ConsoleName: name, Version: version,
		DefaultChannel: a.defaultChannel, StartError: startErr}
	if out.RecentChannels == nil {
		out.RecentChannels = []string{}
	}
	if id, source, err := consoleKey(a.siteKeyDir, a.dir, false); err == nil {
		out.PublicKey = security.EncodePublicKey(id.Public())
		out.Fingerprint = security.KeyFingerprint(id.Public())
		out.KeySource = source
	}
	return out
}

// StartClass opens a class on channel (a number from 1 to 16000), announced
// as consoleName. It also changes channel: the open class is closed first.
func (a *App) StartClass(channel, consoleName string) (View, error) {
	channel, err := protocol.ParseChannel(channel)
	if err != nil {
		return View{}, coded(codeInvalidChannel, err)
	}
	consoleName = strings.TrimSpace(consoleName)
	if consoleName == "" || len(consoleName) > 63 {
		return View{}, codedf(codeInvalidName, "escribe tu nombre como lo verán los alumnos (hasta 63 caracteres)")
	}
	a.EndClass()

	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	class, info, err := a.start(ctx, classConfig{Channel: channel, ConsoleName: consoleName, Dir: a.dir,
		SiteKeyDir: a.siteKeyDir, Port: a.port}, classHooks{onAck: a.onAck, onChange: a.onChange, onFrame: a.onFrame,
		onBroadcast: a.onBroadcastMessage, onPolicyAck: a.onPolicyAck, onPower: a.onPowerMessage, onPause: a.onPauseMessage,
		onRemote: a.onRemoteMessage, onSnapshot: a.onSnapshotMessage, onClassroom: a.onClassroomMessage,
		onTimer: a.onTimerAck, onPoll: a.onPollMessage, onFile: a.onFileMessage, onOpen: a.onOpenMessage, onClassEnd: a.onClassEndAck,
		onError: a.onAgentError}, a.log)
	if err != nil {
		return View{}, err
	}
	a.mu.Lock()
	// The history and the deliveries are the teacher's, not the channel's:
	// they survive a change of channel or name (docs/ux/historial/consola-analisis.md, C2).
	a.class, a.info, a.startErr = class, info, ""
	a.wake.channel.Store(info.Channel)
	a.mu.Unlock()
	a.power.Reset()  // orders of another class are not followed
	a.pauses.Reset() // nor its pauses
	a.opens.Clear()  // nor its openings
	a.emit(EventOpens, a.OpenOrders())
	a.resetTimer() // nor its timer
	a.resetPolls() // nor its polls

	s := loadSettings(a.dir)
	s.remember(channel, consoleName)
	if err := saveSettings(a.dir, s); err != nil {
		a.log.Warn("cannot save settings", "err", err)
	}
	a.log.Info("class started", "channel", channel)
	return a.View(), nil
}

// EndClass closes the class; agents see the console go away and keep
// retrying until a class on their channel opens again.
func (a *App) EndClass() {
	a.mu.Lock()
	class := a.class
	a.class = nil
	a.mu.Unlock()
	if class != nil {
		a.stopSimulationOf(class)
		a.StopBroadcast() // agents also stop when the connection closes
		a.StopPresent()
		a.endControl(class)
		a.snaps.Reset() // unsaved screenshots never outlive the class (rule 45)
		a.board.Reset() // nor hands and messages (rule 51)
		a.emit(EventParticipation, a.participation(nil, nil))
		a.stopFiles()       // what was being sent stops with the class
		a.stopCollections() // and the collections are forgotten; their folders stay
		a.resetTimer()
		a.resetPolls()
		a.sendRestrictions(class, a.restr.Reset())
		a.sendScreen(class, a.wall.Close())
		a.ends.Reset() // nobody is owed a class.end on another channel
		class.Stop()
	}
}

// View returns the whole state.
func (a *App) View() View {
	a.mu.Lock()
	defer a.mu.Unlock()
	v := View{Active: a.class != nil, Class: a.info, Hotkey: a.hotkey,
		History: a.history.Entries(), Deliveries: a.deliveries.Recent()}
	if v.History == nil {
		v.History = []codedist.HistoryEntry{}
	}
	if a.class != nil {
		v.Students = a.studentsIn(a.class)
	} else {
		v.Students = studentsOf(nil)
	}
	v.Restrictions = a.Restrictions()
	v.Broadcast = a.stateOf(a.bc.Status())
	v.Present = presentStateOf(a.proj.Status())
	v.Remote = a.desk.Current()
	v.Snapshots = a.snaps.List()
	v.Participation = a.participation(a.class, nil)
	if a.class != nil {
		v.Timer = a.timer.Snapshot(a.class.Sessions(), simSessionsOf(a.class))
		v.Polls = a.polls.Snapshot(a.class.Sessions(), simSessionsOf(a.class))
		v.Pauses = a.pauses.Summary()
	} else {
		v.Timer = a.timer.Snapshot(nil, nil)
		v.Polls = a.polls.Snapshot(nil, nil)
	}
	return v
}

// --- Emitting -------------------------------------------------------------------

// Emit sends the code panel to every student, or only to req.Targets (session
// IDs, real or simulated): "replace" leaves it alone on their screens,
// "append" adds it below. Each emission is a new history entry.
func (a *App) Emit(req EmitRequest) (codedist.Delivery, error) {
	mode := protocol.CodeMode(req.Mode)
	if mode != protocol.ModeReplace && mode != protocol.ModeAppend {
		return codedist.Delivery{}, codedf(codeInvalidMode, "modo %q desconocido", req.Mode)
	}
	content := normalizeNewlines(req.Content)
	if strings.TrimSpace(content) == "" {
		return codedist.Delivery{}, codedf(codeEmptyCode, "el editor está vacío")
	}
	title := strings.TrimSpace(req.Title)
	if utf8.RuneCountInString(title) > protocol.MaxTitle {
		return codedist.Delivery{}, codedf(codeTitleTooLong, "el título pasa de %d caracteres", protocol.MaxTitle)
	}
	lang := req.Lang
	if lang == "" {
		lang = "plaintext"
	}
	// Validate before recording it in the history.
	probe := protocol.CodeShare{SnippetID: protocol.NewID(), Title: title, Lang: lang, Content: content, Mode: mode}
	if err := probe.Validate(); err != nil {
		return codedist.Delivery{}, friendly(err)
	}
	if a.currentClass() == nil {
		return codedist.Delivery{}, errNoClass
	}
	a.mu.Lock()
	entry := a.history.Add(title, lang, content, a.now())
	a.lastLang = lang
	a.mu.Unlock()
	a.emitHistory()
	return a.broadcastTo("code", entryLabel(entry), entry.Share(mode), req.Targets)
}

// Reemit sends a snippet from the history again, replacing the students'
// screens, with its original number.
func (a *App) Reemit(snippetID string) (codedist.Delivery, error) {
	if a.currentClass() == nil {
		return codedist.Delivery{}, errNoClass
	}
	a.mu.Lock()
	entry, ok := a.history.Touch(snippetID, a.now())
	a.mu.Unlock()
	if !ok {
		return codedist.Delivery{}, codedf(codeSnippetGone, "ese fragmento ya no está en el historial")
	}
	a.emitHistory()
	return a.broadcast("code", entryLabel(entry)+" (reemitido)", entry.Share(protocol.ModeReplace))
}

// ClearScreens empties the code shown on every student screen.
func (a *App) ClearScreens() (codedist.Delivery, error) {
	return a.broadcast("clear", "Limpiar pantallas", protocol.CodeClear{})
}

// SendNotice shows a notice to every student.
func (a *App) SendNotice(text, level string) (codedist.Delivery, error) {
	return a.SendNoticeTo(text, level, nil)
}

// SendNoticeTo shows a notice to the given sessions (real or simulated), or
// to every student if targets is empty.
func (a *App) SendNoticeTo(text, level string, targets []string) (codedist.Delivery, error) {
	text = strings.TrimSpace(text)
	p := protocol.MsgShow{Text: text, Level: protocol.MsgLevel(level)}
	if err := p.Validate(); err != nil {
		return codedist.Delivery{}, friendly(err)
	}
	label := "Aviso: " + text
	if utf8.RuneCountInString(label) > 60 {
		label = string([]rune(label)[:59]) + "…"
	}
	return a.broadcastTo("notice", label, p, targets)
}

// Resend sends a delivery again to the students that have not acknowledged
// it and are still connected ("Reenviar a los que faltan"). Their late acks
// of the first send no longer count; the new ones do, once.
func (a *App) Resend(deliveryID string) (codedist.Delivery, error) {
	class := a.currentClass()
	if class == nil {
		return codedist.Delivery{}, errNoClass
	}
	a.mu.Lock()
	d, ok := a.deliveries.Get(deliveryID)
	p := a.payloads[deliveryID]
	a.mu.Unlock()
	if ok && d.Kind == "file" {
		return a.resendFiles(class, d)
	}
	if !ok || p == nil {
		return codedist.Delivery{}, codedf(codeDeliveryGone, "ese envío ya no está entre los recientes")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a.sendMu.Lock()
	sent := map[string]string{}
	again := slices.Clone(d.Pending) // who did not confirm, and who could not do it
	for _, r := range d.Rejected {
		again = append(again, r.Session)
	}
	for _, sessionID := range again {
		if envID, err := class.Send(ctx, sessionID, p); err == nil {
			sent[sessionID] = envID
		}
	}
	a.mu.Lock()
	d, _ = a.deliveries.Resent(deliveryID, sent)
	a.mu.Unlock()
	a.sendMu.Unlock()
	a.log.Info("resend", "delivery", deliveryID, "type", p.MessageType(), "sent", len(sent))
	a.emitDeliveries()
	return d, nil
}

func (a *App) broadcast(kind, label string, p protocol.Payload) (codedist.Delivery, error) {
	return a.broadcastTo(kind, label, p, nil)
}

// broadcastTo sends p to every student, or only to targets (session IDs), and
// tracks the acks: real students in Sent/Acked, simulated ones apart.
func (a *App) broadcastTo(kind, label string, p protocol.Payload, targets []string) (codedist.Delivery, error) {
	class := a.currentClass()
	if class == nil {
		return codedist.Delivery{}, errNoClass
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	a.sendMu.Lock()
	var sent, simSent map[string]string
	var failed map[string]error
	if len(targets) == 0 {
		sent, failed = class.Broadcast(ctx, p)
		simSent = map[string]string{}
		if host, ok := class.(simulationHost); ok {
			simSent, _ = host.SimBroadcast(ctx, p)
		}
	} else {
		sent, simSent, failed = sendToTargets(ctx, class, p, targets)
	}
	var noCap int
	for _, err := range failed {
		if errors.Is(err, session.ErrUnsupportedCapability) {
			noCap++
		}
	}
	a.mu.Lock()
	d := a.deliveries.Start(kind, label, sent, len(failed)-noCap, noCap, a.now())
	if len(simSent) > 0 {
		d, _ = a.deliveries.AddSimulated(d.ID, simSent)
	}
	a.rememberPayload(d.ID, p)
	a.mu.Unlock()
	a.sendMu.Unlock()

	a.log.Info("broadcast", "type", p.MessageType(), "sent", d.Sent, "failed", d.Failed, "no_capability", d.NoCapability,
		"simulated_sent", d.SimSent)
	a.emitDeliveries()
	return d, nil
}

// --- Screens view (docs/protocol.md §5.6) -----------------------------------------

// OpenScreens opens the screens view: every connected student whose agent
// supports it starts sending thumbnails, with its "screen viewed" indicator.
func (a *App) OpenScreens() error {
	if a.currentClass() == nil {
		return errNoClass
	}
	a.wall.Open()
	a.log.Info("screens view opened")
	a.syncScreens()
	return nil
}

// CloseScreens closes the view: every stream stops and the frames are dropped.
func (a *App) CloseScreens() {
	if class := a.currentClass(); class != nil {
		a.sendScreen(class, a.wall.Close())
	} else {
		a.wall.Close()
	}
	a.log.Info("screens view closed")
}

// ViewScreen shows one student in detail (larger frames); "" goes back to
// the grid of thumbnails.
func (a *App) ViewScreen(sessionID string) {
	a.wall.SetDetail(sessionID)
	a.syncScreens()
}

// syncScreens starts the streams the open view needs: new students, and the
// change between thumbnail and detail.
func (a *App) syncScreens() {
	class := a.currentClass()
	if class == nil || !a.wall.IsOpen() {
		return
	}
	// Simulated students show their screens too, marked as such.
	a.sendScreen(class, a.wall.Plan(append(class.Sessions(), simSessionsOf(class)...)))
}

func (a *App) sendScreen(class classroom, cmds []screen.Command) {
	if len(cmds) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, c := range cmds {
		envID, err := class.Send(ctx, c.SessionID, c.Payload)
		if err != nil {
			a.log.Debug("screen command not sent", "type", c.Payload.MessageType(), "err", err)
			continue
		}
		if start, ok := c.Payload.(protocol.ScreenThumbStart); ok {
			a.wall.Started(c.SessionID, envID, start)
		}
	}
}

// --- Capture (hotkey and button) --------------------------------------------------

// CaptureClipboard is the on-screen equivalent of the capture hotkey.
func (a *App) CaptureClipboard() (Draft, error) { return a.capture("button") }

// capture loads the clipboard text into a Draft for the code panel. It never
// emits: the teacher reviews it and chooses the language first.
func (a *App) capture(source string) (Draft, error) {
	d, err := a.readDraft(source)
	if err != nil {
		a.emit(EventCapture, Capture{Error: err.Error()})
		return Draft{}, err
	}
	a.log.Info("clipboard captured", "source", source, "lines", d.Lines, "lang", d.Lang)
	a.emit(EventCapture, Capture{Draft: &d})
	return d, nil
}

func (a *App) readDraft(source string) (Draft, error) {
	if a.clip == nil {
		return Draft{}, codedf(codeClipboardUnavailable, "el portapapeles no está disponible: %v", a.clipErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	text, err := a.clip.ReadText(ctx)
	if err != nil {
		return Draft{}, codedf(codeClipboardEmpty, "el portapapeles no tiene texto: copia el código primero (Ctrl+C)")
	}
	text = normalizeNewlines(text)
	if strings.TrimSpace(text) == "" {
		return Draft{}, codedf(codeClipboardEmpty, "el portapapeles está vacío: copia el código primero (Ctrl+C)")
	}
	if len(text) > protocol.MaxCodeContent {
		return Draft{}, codedf(codeClipboardTooLarge, "el texto copiado pasa de %d KiB", protocol.MaxCodeContent/1024)
	}
	a.mu.Lock()
	prev := a.lastLang
	a.mu.Unlock()
	return Draft{
		Content: text, Lang: guessLang(text, prev), Source: source, CapturedAt: a.now(),
		Lines: strings.Count(strings.TrimRight(text, "\n"), "\n") + 1,
	}, nil
}

// --- Console callbacks (console goroutine) ------------------------------------------

func (a *App) onAck(refID string) {
	a.sendMu.Lock() // wait for an in-flight broadcast to register its IDs
	a.mu.Lock()
	_, ok := a.deliveries.Ack(refID)
	a.mu.Unlock()
	a.sendMu.Unlock()
	if ok {
		a.emitDeliveries()
	}
}

func (a *App) onChange() {
	class := a.currentClass()
	if class == nil {
		return
	}
	a.recordMachines(class) // before the list: who just woke up is not «Encendiendo…»
	a.emit(EventStudents, a.studentsIn(class))
	a.syncClassEnd(class) // first: who reconnects is cleaned before anything is resent (§5.25.3)
	a.syncScreens()
	a.syncRemote(class.Sessions())
	a.syncSnapshots(class)
	a.bc.Sync(class.Sessions())
	a.proj.Sync(class.Sessions())                             // a source that leaves ends it; who joins receives it
	a.sendRestrictions(class, a.restr.Plan(class.Sessions())) // students who reconnect get them again
	a.syncTimer(class)                                        // and the timer, with what is left
	a.syncPolls(class)                                        // and the open poll, or how it ended
	a.syncCollections(class)                                  // and the folders of the open collections
	a.syncPauses(class)                                       // who left is not paused any more (rule 10)
}

// onFrame forwards a thumbnail of the stream in force to the screens view.
func (a *App) onFrame(sessionID string, f protocol.ScreenThumbFrame) {
	if a.onRemoteFrame(sessionID, f) {
		return
	}
	if t, ok := a.wall.Frame(sessionID, f); ok {
		a.emit(EventFrame, t)
	}
}

// --- Helpers ------------------------------------------------------------------------

func (a *App) currentClass() classroom {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.class
}

func (a *App) emitHistory() {
	a.mu.Lock()
	h := History{Entries: a.history.Entries()}
	a.mu.Unlock()
	a.emit(EventHistory, h)
}

func (a *App) emitDeliveries() {
	a.mu.Lock()
	d := Deliveries{Deliveries: a.deliveries.Recent()}
	a.mu.Unlock()
	a.emit(EventDeliveries, d)
}

func studentsOf(list []session.Session) Students {
	out := Students{Students: make([]Student, 0, len(list))}
	for _, s := range list {
		st := Student{
			ID: s.ID, Hostname: s.Hostname, Username: s.Username, OS: string(s.OS), OSVersion: s.OSVersion,
			AgentVersion: s.AgentVersion, State: "disconnected",
			CanCode: s.Can(protocol.CapCodeView), CanNotice: s.Can(protocol.CapMsgShow),
			CanThumb: s.Can(protocol.CapScreenThumb), CanBroadcast: s.Can(protocol.CapScreenBroadcast),
			CanAppBlock: s.Can(protocol.CapAppBlock), CanWebFilter: s.Can(protocol.CapWebFilter),
			CanPower: s.Can(protocol.CapPower), CanRemote: s.Can(protocol.CapRemote),
			CanSnapshot: s.Can(protocol.CapScreenSnapshot), LastCodeAck: s.LastCodeAck,
			CanHand: s.Can(protocol.CapHand), CanMessage: s.Can(protocol.CapMsgStudent), CanTimer: s.Can(protocol.CapTimer), CanPoll: s.Can(protocol.CapPoll), CanQuiz: s.Can(protocol.CapPollQuiz),
			CanFiles: s.Can(protocol.CapFilePush), CanCollect: s.Can(protocol.CapFileCollect),
			CanOpenURL: s.Can(protocol.CapOpenURL), CanOpenApp: s.Can(protocol.CapOpenApp),
			CanPresent: s.Can(protocol.CapScreenPresent), CanPresentView: s.Can(protocol.CapScreenPresentView),
			CanPause: s.Can(protocol.CapScreenFreeze), CanClassEnd: s.Can(protocol.CapClassEnd),
		}
		for _, c := range s.Capabilities {
			st.Capabilities = append(st.Capabilities, string(c))
		}
		if !s.LastCodeAckAt.IsZero() {
			st.AckedAt = s.LastCodeAckAt.Format("15:04:05")
		}
		if !s.ConnectedAt.IsZero() {
			st.ConnectedAt = s.ConnectedAt.Format("15:04")
		}
		if s.State == session.StateConnected {
			st.State = "connected"
			out.Connected++
			if st.CanCode {
				out.CanCode++
			}
			if st.CanNotice {
				out.CanNotice++
			}
			if st.CanThumb {
				out.CanThumb++
			}
			if st.CanBroadcast {
				out.CanBroadcast++
			}
			if st.CanAppBlock {
				out.CanAppBlock++
			}
			if st.CanWebFilter {
				out.CanWebFilter++
			}
			if st.CanPower {
				out.CanPower++
			}
			if st.CanPause {
				out.CanPause++
			}
			if st.CanSnapshot {
				out.CanSnapshot++
			}
			if st.CanTimer {
				out.CanTimer++
			}
			if st.CanPoll {
				out.CanPoll++
			}
			if st.CanFiles {
				out.CanFiles++
			}
			if st.CanCollect {
				out.CanCollect++
			}
			if st.CanOpenURL {
				out.CanOpenURL++
			}
			if st.CanOpenApp {
				out.CanOpenApp++
			}
		}
		out.Students = append(out.Students, st)
	}
	return out
}

func entryLabel(e codedist.HistoryEntry) string {
	if e.Title == "" {
		return fmt.Sprintf("Ejercicio %d", e.Number)
	}
	return fmt.Sprintf("Ejercicio %d · %s", e.Number, e.Title)
}

// friendly turns protocol validation errors into Spanish for the UI.
func friendly(err error) error {
	switch {
	case errors.Is(err, protocol.ErrMessageTooLarge):
		return codedf(codeCodeTooLarge, "el código pasa del límite de %d KiB", protocol.MaxCodeContent/1024)
	case strings.Contains(err.Error(), "text must be"):
		return codedf(codeInvalidNotice, "el aviso debe tener entre 1 y %d caracteres", protocol.MaxMsgText)
	case strings.Contains(err.Error(), "level must be"):
		return codedf(codeInvalidNotice, "nivel de aviso desconocido")
	case strings.Contains(err.Error(), "malformed lang"):
		return codedf(codeUnknownLanguage, "lenguaje desconocido")
	default:
		return err
	}
}
