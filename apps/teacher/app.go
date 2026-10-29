package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/screen"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
)

var errNoClass = errors.New("primero abre una clase")

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
	a.bc = a.newBroadcaster(broadcastKbps(m))
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
		a.hotkey.Error = "Otra aplicación ya usa " + a.hotkey.Combo + ". Usa el botón Capturar."
	case errors.Is(err, platform.ErrNotSupported):
		a.hotkey.Error = "El atajo global no está disponible en este sistema. Usa el botón Capturar."
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
		return View{}, err
	}
	consoleName = strings.TrimSpace(consoleName)
	if consoleName == "" || len(consoleName) > 63 {
		return View{}, errors.New("escribe tu nombre como lo verán los alumnos (hasta 63 caracteres)")
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
		onBroadcast: a.onBroadcastMessage}, a.log)
	if err != nil {
		return View{}, err
	}
	a.mu.Lock()
	a.class, a.info, a.startErr = class, info, ""
	a.history, a.deliveries = &codedist.History{}, &codedist.Deliveries{}
	a.mu.Unlock()

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
		a.StopBroadcast() // agents also stop when the connection closes
		a.sendScreen(class, a.wall.Close())
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
		v.Students = studentsOf(a.class.Sessions())
	} else {
		v.Students = studentsOf(nil)
	}
	v.Broadcast = a.stateOf(a.bc.Status())
	return v
}

// --- Emitting -------------------------------------------------------------------

// Emit sends the code panel to every student: "replace" leaves it alone on
// their screens, "append" adds it below. Each emission is a new history entry.
func (a *App) Emit(req EmitRequest) (codedist.Delivery, error) {
	mode := protocol.CodeMode(req.Mode)
	if mode != protocol.ModeReplace && mode != protocol.ModeAppend {
		return codedist.Delivery{}, fmt.Errorf("modo %q desconocido", req.Mode)
	}
	content := normalizeNewlines(req.Content)
	if strings.TrimSpace(content) == "" {
		return codedist.Delivery{}, errors.New("el editor está vacío")
	}
	title := strings.TrimSpace(req.Title)
	if utf8.RuneCountInString(title) > protocol.MaxTitle {
		return codedist.Delivery{}, fmt.Errorf("el título pasa de %d caracteres", protocol.MaxTitle)
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
	return a.broadcast("code", entryLabel(entry), entry.Share(mode))
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
		return codedist.Delivery{}, errors.New("ese fragmento ya no está en el historial")
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
	text = strings.TrimSpace(text)
	p := protocol.MsgShow{Text: text, Level: protocol.MsgLevel(level)}
	if err := p.Validate(); err != nil {
		return codedist.Delivery{}, friendly(err)
	}
	label := "Aviso: " + text
	if utf8.RuneCountInString(label) > 60 {
		label = string([]rune(label)[:59]) + "…"
	}
	return a.broadcast("notice", label, p)
}

func (a *App) broadcast(kind, label string, p protocol.Payload) (codedist.Delivery, error) {
	class := a.currentClass()
	if class == nil {
		return codedist.Delivery{}, errNoClass
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	a.sendMu.Lock()
	sent, failed := class.Broadcast(ctx, p)
	ids := make([]string, 0, len(sent))
	for _, envID := range sent {
		ids = append(ids, envID)
	}
	var noCap int
	for _, err := range failed {
		if errors.Is(err, session.ErrUnsupportedCapability) {
			noCap++
		}
	}
	a.mu.Lock()
	d := a.deliveries.Start(kind, label, ids, len(failed)-noCap, noCap, a.now())
	a.mu.Unlock()
	a.sendMu.Unlock()

	a.log.Info("broadcast", "type", p.MessageType(), "sent", d.Sent, "failed", d.Failed, "no_capability", d.NoCapability)
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
	a.sendScreen(class, a.wall.Plan(class.Sessions()))
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
		return Draft{}, fmt.Errorf("el portapapeles no está disponible: %v", a.clipErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	text, err := a.clip.ReadText(ctx)
	if err != nil {
		return Draft{}, errors.New("el portapapeles no tiene texto: copia el código primero (Ctrl+C)")
	}
	text = normalizeNewlines(text)
	if strings.TrimSpace(text) == "" {
		return Draft{}, errors.New("el portapapeles está vacío: copia el código primero (Ctrl+C)")
	}
	if len(text) > protocol.MaxCodeContent {
		return Draft{}, fmt.Errorf("el texto copiado pasa de %d KiB", protocol.MaxCodeContent/1024)
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
	a.emit(EventStudents, studentsOf(class.Sessions()))
	a.syncScreens()
	a.bc.Sync(class.Sessions())
}

// onFrame forwards a thumbnail of the stream in force to the screens view.
func (a *App) onFrame(sessionID string, f protocol.ScreenThumbFrame) {
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
			LastCodeAck: s.LastCodeAck,
		}
		for _, c := range s.Capabilities {
			st.Capabilities = append(st.Capabilities, string(c))
		}
		if !s.LastCodeAckAt.IsZero() {
			st.AckedAt = s.LastCodeAckAt.Format("15:04:05")
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
		return fmt.Errorf("el código pasa del límite de %d KiB", protocol.MaxCodeContent/1024)
	case strings.Contains(err.Error(), "text must be"):
		return fmt.Errorf("el aviso debe tener entre 1 y %d caracteres", protocol.MaxMsgText)
	case strings.Contains(err.Error(), "level must be"):
		return errors.New("nivel de aviso desconocido")
	case strings.Contains(err.Error(), "malformed lang"):
		return errors.New("lenguaje desconocido")
	default:
		return err
	}
}
