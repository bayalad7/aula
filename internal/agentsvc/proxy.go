package agentsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/platform"
)

// Calls and events on the local channel. Service → helper: the screen of the
// student, which only the helper can touch.
const (
	CallClipboardWrite = "clipboard.write"
	CallScreenCapture  = "screen.capture"
	CallIndicatorShow  = "indicator.show"
	CallIndicatorHide  = "indicator.hide"
	CallPower          = "power.run"    // shut down, reboot or log off from the student's session: PowerArgs
	CallInputInject    = "input.inject" // remote control: []platform.InputEvent, injected in the session
	CallInputLocal     = "input.local"  // remote control: time of the student's last own input
	CallFileWrite      = "file.write"   // file.push: FileWriteArgs, saved as the student in the work folder
	CallFolderOpen     = "folder.open"  // file.push: open a folder of the work folder in Explorer
	EventEmit          = "emit"         // a Wails event for the window
	EventRaise         = "raise"        // bring the window forward
	EventNotify        = "notify"       // something arrived: tell without taking the focus
	EventHold          = "hold"         // the teacher's broadcast holds the window up, or lets it go: HoldArgs
	EventOnTop         = "ontop"        // keep the window above the others (true) or not
)

// HoldArgs is EventHold: On holds the window in Mode ("window",
// "fullscreen", or "" for v0.4's maximized window); !On puts it back as it
// was (docs/privacidad.md rule 12).
type HoldArgs struct {
	On   bool   `json:"on"`
	Mode string `json:"mode,omitempty"`
}

// Helper → service.
const (
	// EventReady carries Ready once the helper's window has reported what it
	// can do: the service then declares the matching capabilities.
	EventReady = "ready"
	// CallBinding forwards a call of the window to the agent: Binding.
	CallBinding = "binding"
)

// Ready is what the helper can do in the student's session.
type Ready struct {
	Capture   bool `json:"capture"`   // screen capture and the "being viewed" notice
	Clipboard bool `json:"clipboard"` // system clipboard
	Broadcast bool `json:"broadcast"` // WebRTC with H.264 in the window
	Input     bool `json:"input"`     // keyboard and mouse injection, for remote control
	Files     bool `json:"files"`     // the student's work folder, for file.push
	Open      bool `json:"open"`      // open a page or an app in the session, for open.*
	// Present: monitor capture, an H.264 encoder and windows that can be
	// excluded from the capture, for student.present.* (§5.24).
	Present bool `json:"present"`
	// Pause: the pause window on every monitor, for screen.freeze (§5.9).
	Pause bool `json:"pause"`
}

// Binding is a call of the window (View, Copy…) forwarded to the agent.
type Binding struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}

// NewBinding encodes a call of method with its arguments.
func NewBinding(method string, args ...any) (Binding, error) {
	b := Binding{Method: method, Args: make([]json.RawMessage, len(args))}
	for i, a := range args {
		raw, err := json.Marshal(a)
		if err != nil {
			return Binding{}, fmt.Errorf("%s argument %d: %w", method, i, err)
		}
		b.Args[i] = raw
	}
	return b, nil
}

// Arg decodes argument i into out.
func (b Binding) Arg(i int, out any) error {
	if i >= len(b.Args) {
		return fmt.Errorf("%s: missing argument %d", b.Method, i)
	}
	if err := json.Unmarshal(b.Args[i], out); err != nil {
		return fmt.Errorf("%s argument %d: %w", b.Method, i, err)
	}
	return nil
}

// CaptureArgs are the arguments of CallScreenCapture.
type CaptureArgs struct {
	MaxWidth int `json:"maxWidth"`
	Quality  int `json:"quality"`
}

// ErrNoHelper is returned while there is no helper in the student's session:
// nothing can be shown or captured.
var ErrNoHelper = errors.New("no helper in the student's session")

// Current holds the link with the helper in force, if any.
type Current struct {
	mu sync.Mutex
	l  *Link
}

// Set replaces the link (nil when the helper is gone).
func (c *Current) Set(l *Link) {
	c.mu.Lock()
	c.l = l
	c.mu.Unlock()
}

// Get returns the link in force, or nil.
func (c *Current) Get() *Link {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l
}

// callTimeout bounds a call to the helper: a hung window must not hang the
// agent.
const callTimeout = 10 * time.Second

func (c *Current) call(ctx context.Context, name string, args, out any) error {
	l := c.Get()
	if l == nil {
		return ErrNoHelper
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return l.Call(ctx, name, args, out)
}

// Notify sends an event to the helper; it is dropped without one (the window
// asks for the whole state when it connects).
func (c *Current) Notify(name string, data any) {
	if l := c.Get(); l != nil {
		_ = l.Notify(name, data)
	}
}

// Clipboard writes on the student's clipboard through the helper.
type Clipboard struct{ Helper *Current }

// WriteText implements platform.Clipboard.
func (c Clipboard) WriteText(ctx context.Context, s string) error {
	return c.Helper.call(ctx, CallClipboardWrite, s, nil)
}

// ReadText implements platform.Clipboard; the agent never reads it.
func (Clipboard) ReadText(context.Context) (string, error) { return "", platform.ErrNotSupported }

// Capture grabs the student's screen through the helper, which shows the
// "being viewed" notice: the service itself cannot capture (docs/privacidad.md
// rule 17).
type Capture struct{ Helper *Current }

// Capture implements platform.ScreenCapture.
func (c Capture) Capture(ctx context.Context, maxWidth, quality int) (platform.Frame, error) {
	var f platform.Frame
	err := c.Helper.call(ctx, CallScreenCapture, CaptureArgs{MaxWidth: maxWidth, Quality: quality}, &f)
	return f, err
}

// Indicator shows the "being viewed" notice through the helper.
type Indicator struct{ Helper *Current }

// Show implements platform.ScreenIndicator.
func (i Indicator) Show(text string) error {
	return i.Helper.call(context.Background(), CallIndicatorShow, text, nil)
}

// Hide implements platform.ScreenIndicator. Without a helper there is
// nothing on screen to hide.
func (i Indicator) Hide() error {
	err := i.Helper.call(context.Background(), CallIndicatorHide, nil, nil)
	if errors.Is(err, ErrNoHelper) || errors.Is(err, ErrClosed) {
		return nil
	}
	return err
}

// PowerArgs are the arguments of CallPower.
type PowerArgs struct {
	Action  string `json:"action"` // "shutdown" | "reboot" | "logoff"
	Session uint32 `json:"session"`
	Force   bool   `json:"force"`
}

// Power shuts down, reboots or logs off from the student's session through
// the helper (D-018): from SYSTEM, Windows refuses a shutdown without force
// while the student is logged on, and ExitWindowsEx only logs off the
// caller's own session.
type Power struct{ Helper *Current }

// Shutdown implements platform.Power.
func (p Power) Shutdown(force bool) error { return p.run("shutdown", 0, force) }

// Reboot implements platform.Power.
func (p Power) Reboot(force bool) error { return p.run("reboot", 0, force) }

// Logoff implements platform.Power; the helper checks it is its session.
func (p Power) Logoff(session uint32, force bool) error { return p.run("logoff", session, force) }

func (p Power) run(action string, session uint32, force bool) error {
	return p.Helper.call(context.Background(), CallPower, PowerArgs{Action: action, Session: session, Force: force}, nil)
}

// Input injects the teacher's keyboard and mouse in the student's session
// through the helper during a remote control (docs/protocol.md §5.16): the
// service, in session 0, cannot reach the student's desktop.
type Input struct{ Helper *Current }

// Inject implements platform.InputInjector.
func (i Input) Inject(events []platform.InputEvent) error {
	return i.Helper.call(context.Background(), CallInputInject, events, nil)
}

// LocalInput implements platform.InputInjector.
func (i Input) LocalInput() (time.Time, error) {
	var t time.Time
	err := i.Helper.call(context.Background(), CallInputLocal, nil, &t)
	return t, err
}

// FileWriteArgs are the arguments of CallFileWrite: a file of file.push,
// already reassembled and checked by the service.
type FileWriteArgs struct {
	Rel       string `json:"rel"` // "/"-separated, inside the work folder
	Data      []byte `json:"data"`
	Overwrite bool   `json:"overwrite"`
}

// FileWriteResult is the reply of CallFileWrite. Error is one of the
// fileErr* codes, so the service tells the reasons apart (the channel only
// carries error text).
type FileWriteResult struct {
	Replaced bool   `json:"replaced"`
	Error    string `json:"error,omitempty"`
}

// Codes of FileWriteResult.Error.
const (
	fileErrExists      = "exists"
	fileErrLink        = "link"
	fileErrNoSpace     = "no_space"
	fileErrDenied      = "denied"
	fileErrTooLong     = "too_long"
	fileErrUnsupported = "unsupported"
	fileErrOther       = "other"
	fileErrTooLarge    = "too_large" // file.collect reads from here on
	fileErrOffline     = "offline"
	fileErrNotFound    = "not_found"
)

var fileErrs = []struct {
	code string
	err  error
}{
	{fileErrExists, platform.ErrFileExists}, {fileErrLink, platform.ErrLinkInPath},
	{fileErrNoSpace, platform.ErrNoSpace}, {fileErrDenied, platform.ErrDenied},
	{fileErrTooLong, platform.ErrPathTooLong}, {fileErrUnsupported, platform.ErrNotSupported},
	{fileErrTooLarge, platform.ErrFileTooLarge}, {fileErrOffline, platform.ErrOffline},
	{fileErrNotFound, fs.ErrNotExist},
}

// FileWriteError turns an error of platform.WorkFolder.Create into the code
// the helper replies with ("" for nil).
func FileWriteError(err error) string {
	if err == nil {
		return ""
	}
	for _, e := range fileErrs {
		if errors.Is(err, e.err) {
			return e.code
		}
	}
	return fileErrOther
}

// fileWriteErr is the inverse of FileWriteError.
func fileWriteErr(code string) error {
	if code == "" {
		return nil
	}
	for _, e := range fileErrs {
		if e.code == code {
			return e.err
		}
	}
	return errors.New("the helper could not write the file")
}

// fileCallTimeout bounds a write: 16 MiB to a Documents folder redirected
// to a network share can take longer than callTimeout.
const fileCallTimeout = 60 * time.Second

// WorkFolder saves the files the teacher sends through the helper, in the
// student's session and with the student's identity: the file belongs to the
// student, never to SYSTEM (docs/privacidad.md rule 55). It implements
// platform.WorkFolder and filepush.Writer.
type WorkFolder struct{ Helper *Current }

// Root implements platform.WorkFolder. The service does not need the path:
// the helper resolves it in each call.
func (WorkFolder) Root() (string, error) { return "", platform.ErrNotSupported }

// Create implements platform.WorkFolder.
func (w WorkFolder) Create(rel string, data []byte, overwrite bool) (bool, error) {
	l := w.Helper.Get()
	if l == nil {
		return false, ErrNoHelper
	}
	ctx, cancel := context.WithTimeout(context.Background(), fileCallTimeout)
	defer cancel()
	var r FileWriteResult
	if err := l.Call(ctx, CallFileWrite, FileWriteArgs{Rel: rel, Data: data, Overwrite: overwrite}, &r); err != nil {
		return false, err
	}
	return r.Replaced, fileWriteErr(r.Error)
}

// Open implements platform.WorkFolder.
func (w WorkFolder) Open(rel string) error {
	return w.Helper.call(context.Background(), CallFolderOpen, rel, nil)
}
