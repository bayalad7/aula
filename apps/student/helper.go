package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/collect"
	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/openapp"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// helperReadyWait bounds how long the helper waits for its window to report
// WebRTC support before telling the service what it can do.
const helperReadyWait = 5 * time.Second

// helper is the agent's window in the student's session when the agent runs
// as a service (D-016). It shows the identity banner, the code and every
// notice, and is the only process that touches the student's screen and
// clipboard; it runs with the student's normal rights.
type helper struct {
	clip      platform.Clipboard
	capture   platform.ScreenCapture
	indicator platform.ScreenIndicator
	input     *heldInput                // nil when the OS cannot inject
	work      platform.WorkFolder       // nil when the OS has no work folder (file.push)
	read      platform.WorkFolderReader // nil without a work folder (file.collect)
	launch    platform.Launcher         // nil when the OS cannot open pages or apps (open.*)
	publish   *broadcast.LocalPublisher // nil when this session cannot be shown to the class (§5.24)
	link      *agentsvc.Link            // to the service: present.* events
	log       *slog.Logger

	presMu sync.Mutex
	pub    broadcast.Publication // the presentation in force
}

// runHelper connects to the service's channel and runs the window until the
// channel ends (the service then starts a new helper if needed).
func runHelper(channel string, log *slog.Logger) int {
	log.Info("aula-cliente helper starting", "version", version)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	conn, err := platform.DialLocal(ctx, channel)
	cancel()
	if err != nil {
		log.Error("helper cannot reach the service", "err", err)
		return 1
	}
	h := &helper{log: log}
	var clipErr, capErr, indErr error
	h.clip, clipErr = platform.NewClipboard()
	h.capture, capErr = platform.NewScreenCapture()
	h.indicator, indErr = platform.NewScreenIndicator()
	if capErr != nil || indErr != nil {
		h.capture, h.indicator = nil, nil // no capture without its notice
	}
	if in, err := platform.NewInputInjector(); err == nil {
		h.input = &heldInput{in: in}
	}
	if wf, err := platform.NewWorkFolder(); err == nil {
		h.work = wf
	}
	if wr, err := platform.NewWorkFolderReader(); err == nil {
		h.read = wr
	}
	if l, err := platform.NewLauncher(); err == nil {
		h.launch = l
	}
	if dc, err := platform.NewDisplayCapture(); err == nil && h.capture != nil {
		// The window and the badge are this process's: they leave the capture.
		h.publish = &broadcast.LocalPublisher{Capture: dc, NewEncoder: platform.NewH264Encoder,
			Exclude: platform.ExcludeFromCapture, Logger: log}
	}

	// The App only serves the window's bindings, forwarding them to the
	// agent in the service; it does not connect to any console.
	app := NewApp(Config{}, nil, nil, localHello(), h.clip, clipErr, log)
	var wctx context.Context
	var wmu sync.Mutex
	window := func() context.Context { wmu.Lock(); defer wmu.Unlock(); return wctx }
	link := agentsvc.NewLink(conn, h.call, func(name string, data json.RawMessage) {
		if ctx := window(); ctx != nil {
			h.event(ctx, app, name, data)
		}
	})
	app.remote = link
	h.link = link
	var once sync.Once
	ready := func(webrtc, h264 bool) {
		once.Do(func() {
			r := agentsvc.Ready{Capture: h.capture != nil, Clipboard: clipErr == nil, Broadcast: webrtc && h264,
				Input: h.input != nil, Files: h.work != nil && h.read != nil, Open: h.launch != nil,
				Present: h.publish.Available()}
			log.Info("helper ready", "capture", r.Capture, "clipboard", r.Clipboard, "broadcast", r.Broadcast, "input", r.Input, "files", r.Files, "open", r.Open)
			_ = link.Notify(agentsvc.EventReady, r)
		})
	}
	app.onMedia = ready

	err = runWindow(app, func(ctx context.Context) {
		wmu.Lock()
		wctx = ctx
		wmu.Unlock()
		time.AfterFunc(helperReadyWait, func() { ready(false, false) })
		go func() {
			_ = link.Run(ctx)
			log.Info("helper channel closed")
			if app.quitWindow != nil {
				app.quitWindow()
			}
			runtime.Quit(ctx)
		}()
	}, func() {
		link.Close()
		if h.input != nil {
			h.input.releaseAll() // nothing stays pressed if the service goes away
		}
		h.closePresentation() // the windows can be captured again
		if h.indicator != nil {
			_ = h.indicator.Hide()
		}
	})
	if err != nil {
		log.Error("wails run failed", "err", err)
		return 1
	}
	return 0
}

// call answers the service: the screen and the clipboard of the student.
func (h *helper) call(ctx context.Context, name string, data json.RawMessage) (any, error) {
	switch name {
	case agentsvc.CallClipboardWrite:
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, err
		}
		if h.clip == nil {
			return nil, platform.ErrNotSupported
		}
		return nil, h.clip.WriteText(ctx, s)
	case agentsvc.CallScreenCapture:
		var a agentsvc.CaptureArgs
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		if h.capture == nil {
			return nil, platform.ErrNotSupported
		}
		return h.capture.Capture(ctx, a.MaxWidth, a.Quality)
	case agentsvc.CallIndicatorShow:
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return nil, err
		}
		if h.indicator == nil {
			return nil, platform.ErrNotSupported
		}
		return nil, h.indicator.Show(text)
	case agentsvc.CallIndicatorHide:
		if h.indicator == nil {
			return nil, nil
		}
		return nil, h.indicator.Hide()
	case agentsvc.CallInputInject:
		var events []platform.InputEvent
		if err := json.Unmarshal(data, &events); err != nil {
			return nil, err
		}
		if h.input == nil {
			return nil, platform.ErrNotSupported
		}
		return nil, h.input.Inject(events)
	case agentsvc.CallInputLocal:
		if h.input == nil {
			return nil, platform.ErrNotSupported
		}
		return h.input.LocalInput()
	case agentsvc.CallFileWrite:
		// The service reassembled and checked the file; the work folder
		// checks again that it stays inside and follows no link.
		var a agentsvc.FileWriteArgs
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		if h.work == nil {
			return agentsvc.FileWriteResult{Error: agentsvc.FileWriteError(platform.ErrNotSupported)}, nil
		}
		if filepush.CheckRel(a.Rel) != nil {
			return agentsvc.FileWriteResult{Error: agentsvc.FileWriteError(platform.ErrLinkInPath)}, nil
		}
		replaced, err := h.work.Create(a.Rel, a.Data, a.Overwrite)
		if err != nil {
			h.log.Warn("file not written", "err", err)
		}
		return agentsvc.FileWriteResult{Replaced: replaced, Error: agentsvc.FileWriteError(err)}, nil
	case agentsvc.CallFolderOpen:
		var rel string
		if err := json.Unmarshal(data, &rel); err != nil {
			return nil, err
		}
		if h.work == nil {
			return nil, platform.ErrNotSupported
		}
		if rel != "" && filepush.CheckDest(rel) != nil {
			return nil, platform.ErrLinkInPath
		}
		return nil, h.work.Open(rel)
	case agentsvc.CallFolderCreate, agentsvc.CallFolderList, agentsvc.CallFileRead:
		return h.collectCall(name, data)
	case agentsvc.CallOpenURL, agentsvc.CallOpenApp:
		return h.openCall(name, data)
	case agentsvc.CallPresentOpen, agentsvc.CallPresentAnswer, agentsvc.CallPresentCandidate, agentsvc.CallPresentClose:
		return h.presentCall(name, data)
	case agentsvc.CallPower:
		var a agentsvc.PowerArgs
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		return nil, runPower(a)
	}
	return nil, fmt.Errorf("%s: unknown call", name)
}

// event shows what the agent in the service sends to the window.
func (h *helper) event(ctx context.Context, app *App, name string, data json.RawMessage) {
	switch name {
	case agentsvc.EventEmit:
		var e struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(data, &e); err != nil {
			return
		}
		runtime.EventsEmit(ctx, e.Event, e.Data) // the window sets its own title from the status
	case agentsvc.EventRaise:
		app.raise()
	case agentsvc.EventNotify:
		app.notify()
	case agentsvc.EventMaximize:
		var on bool
		if json.Unmarshal(data, &on) == nil {
			app.maximize(on)
		}
	case agentsvc.EventOnTop:
		var on bool
		if json.Unmarshal(data, &on) == nil {
			app.onTop(on)
		}
	}
}

// runPower shuts down, reboots or logs off from the student's session, as
// the service asks after the countdown (D-018). It only logs off its own
// session.
func runPower(a agentsvc.PowerArgs) error {
	p, err := platform.NewPower()
	if err != nil {
		return err
	}
	switch a.Action {
	case "shutdown":
		return p.Shutdown(a.Force)
	case "reboot":
		return p.Reboot(a.Force)
	case "logoff":
		own, err := platform.CurrentSession()
		if err != nil {
			return err
		}
		if own != a.Session {
			return fmt.Errorf("log off session %d: the helper runs in session %d", a.Session, own)
		}
		return p.Logoff(own, a.Force)
	}
	return fmt.Errorf("unknown power action %q", a.Action)
}

// heldInput injects for the service and remembers what stays pressed, so the
// helper can let go of it if the service disappears in the middle of a remote
// control (the service releases everything itself when it ends normally).
type heldInput struct {
	in platform.InputInjector

	mu      sync.Mutex
	keys    map[string]bool
	buttons map[platform.MouseButton]bool
}

func (h *heldInput) Inject(events []platform.InputEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.keys == nil {
		h.keys, h.buttons = map[string]bool{}, map[platform.MouseButton]bool{}
	}
	for _, e := range events {
		switch e.Kind {
		case platform.InputKey:
			if e.Down {
				h.keys[e.Code] = true
			} else {
				delete(h.keys, e.Code)
			}
		case platform.InputButton:
			if e.Down {
				h.buttons[e.Button] = true
			} else {
				delete(h.buttons, e.Button)
			}
		}
	}
	return h.in.Inject(events)
}

func (h *heldInput) LocalInput() (time.Time, error) { return h.in.LocalInput() }

// releaseAll lets go of every key and button still pressed.
func (h *heldInput) releaseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	var up []platform.InputEvent
	for k := range h.keys {
		up = append(up, platform.InputEvent{Kind: platform.InputKey, Code: k})
	}
	for b := range h.buttons {
		up = append(up, platform.InputEvent{Kind: platform.InputButton, Button: b})
	}
	clear(h.keys)
	clear(h.buttons)
	if len(up) > 0 {
		_ = h.in.Inject(up)
	}
}

// collectCall serves file.collect in the student's session (§5.21): the
// service checked the path; the helper checks again that it is an activity
// folder or a file inside one (docs/privacidad.md rule 60), and the work
// folder refuses links. It only reads, and creates the activity folder.
func (h *helper) collectCall(name string, data json.RawMessage) (any, error) {
	unsupported := agentsvc.FileWriteError(platform.ErrNotSupported)
	refused := agentsvc.FileWriteError(platform.ErrLinkInPath)
	switch name {
	case agentsvc.CallFolderCreate:
		var rel string
		if err := json.Unmarshal(data, &rel); err != nil {
			return nil, err
		}
		if h.read == nil {
			return agentsvc.FolderCreateResult{Error: unsupported}, nil
		}
		if collect.CheckWorkPath(rel, false) != nil {
			return agentsvc.FolderCreateResult{Error: refused}, nil
		}
		return agentsvc.FolderCreateResult{Error: agentsvc.FileWriteError(h.read.MkdirAll(rel))}, nil
	case agentsvc.CallFolderList:
		var a agentsvc.FolderListArgs
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		if h.read == nil {
			return agentsvc.FolderListResult{Error: unsupported}, nil
		}
		if collect.CheckWorkPath(a.Rel, false) != nil || a.Depth < 1 || a.Depth > protocol.MaxFileCollectDepth {
			return agentsvc.FolderListResult{Error: refused}, nil
		}
		files, err := h.read.List(a.Rel, a.Depth)
		return agentsvc.FolderListResult{Files: files, Error: agentsvc.FileWriteError(err)}, nil
	default: // CallFileRead
		var a agentsvc.FileReadArgs
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		if h.read == nil {
			return agentsvc.FileReadResult{Error: unsupported}, nil
		}
		if collect.CheckWorkPath(a.Rel, true) != nil || a.Max < 0 || a.Max > protocol.MaxFileCollectBytes {
			return agentsvc.FileReadResult{Error: refused}, nil
		}
		b, err := h.read.Read(a.Rel, a.Max)
		if err != nil {
			h.log.Info("collected file not read", "err", err)
		}
		return agentsvc.FileReadResult{Data: b, Error: agentsvc.FileWriteError(err)}, nil
	}
}

// openCall opens a page or an app in the student's session (§5.22), as the
// student. The service checked it; the helper checks the URL and the path
// again with the fixed rules and this session's folders (docs/privacidad.md
// rules 68–69). It logs only the result.
func (h *helper) openCall(name string, data json.RawMessage) (any, error) {
	var arg string
	if err := json.Unmarshal(data, &arg); err != nil {
		return nil, err
	}
	if h.launch == nil {
		return agentsvc.OpenResult{Reason: protocol.OpenLaunchFailed}, nil
	}
	s := openapp.Local{Launcher: h.launch}
	var err error
	if name == agentsvc.CallOpenURL {
		err = s.OpenURL(arg)
	} else {
		err = s.StartApp(arg)
	}
	if err != nil {
		h.log.Info("helper did not open", "kind", name, "reason", openapp.Reason(err))
	}
	return agentsvc.OpenResult{Reason: openapp.Reason(err)}, nil
}

// presentCall publishes this session's screen for the service (§5.24): the
// capture, the encoder and the WebRTC connection live here, with the
// student's rights; the service only passes the signaling.
func (h *helper) presentCall(name string, data json.RawMessage) (any, error) {
	h.presMu.Lock()
	defer h.presMu.Unlock()
	switch name {
	case agentsvc.CallPresentOpen:
		var opt broadcast.PublishOptions
		if err := json.Unmarshal(data, &opt); err != nil {
			return nil, err
		}
		if !h.publish.Available() {
			return agentsvc.PresentOpenResult{Error: agentsvc.PresentErrCapture}, nil
		}
		h.closeLocked()
		pub, err := h.publish.Open(opt, broadcast.PublishEvents{
			OnCandidate: func(attempt int, c broadcast.Candidate) {
				_ = h.link.Notify(agentsvc.EventPresentCandidate, agentsvc.PresentCandidateArgs{
					Attempt: attempt, Candidate: c.Candidate, SDPMid: c.SDPMid, SDPMLineIndex: c.SDPMLineIndex})
			},
			OnFail: func(err error) { _ = h.link.Notify(agentsvc.EventPresentFail, err.Error()) },
		})
		if err != nil {
			h.log.Warn("presentation not opened", "err", err)
			return agentsvc.PresentOpenResult{Error: agentsvc.PresentOpenError(err)}, nil
		}
		h.pub = pub
		w, ht := pub.Size()
		return agentsvc.PresentOpenResult{Width: w, Height: ht}, nil
	case agentsvc.CallPresentAnswer:
		var a agentsvc.PresentAnswerArgs
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		if h.pub == nil {
			return nil, broadcast.ErrNoPresentation
		}
		return h.pub.Answer(a.Attempt, a.Offer)
	case agentsvc.CallPresentCandidate:
		var c agentsvc.PresentCandidateArgs
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		if h.pub == nil {
			return nil, nil
		}
		return nil, h.pub.AddCandidate(c.Attempt, broadcast.Candidate{Candidate: c.Candidate, SDPMid: c.SDPMid, SDPMLineIndex: c.SDPMLineIndex})
	default: // CallPresentClose
		h.closeLocked()
		return nil, nil
	}
}

// closePresentation ends the presentation in force, if any.
func (h *helper) closePresentation() {
	h.presMu.Lock()
	defer h.presMu.Unlock()
	h.closeLocked()
}

func (h *helper) closeLocked() {
	if h.pub != nil {
		if err := h.pub.Close(); err != nil {
			h.log.Warn("presentation close", "err", err)
		}
		h.pub = nil
	}
}
