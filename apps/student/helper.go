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
	"github.com/bayalad7/aula/internal/platform"
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
	log       *slog.Logger
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
	var once sync.Once
	ready := func(webrtc, h264 bool) {
		once.Do(func() {
			r := agentsvc.Ready{Capture: h.capture != nil, Clipboard: clipErr == nil, Broadcast: webrtc && h264}
			log.Info("helper ready", "capture", r.Capture, "clipboard", r.Clipboard, "broadcast", r.Broadcast)
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
			runtime.Quit(ctx)
		}()
	}, func() {
		link.Close()
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
	case agentsvc.CallPowerLogoff:
		var session uint32
		if err := json.Unmarshal(data, &session); err != nil {
			return nil, err
		}
		return nil, logoffOwnSession(session)
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
		runtime.EventsEmit(ctx, e.Event, e.Data)
		if e.Event == EventStatus {
			var s Status
			if json.Unmarshal(e.Data, &s) == nil {
				runtime.WindowSetTitle(ctx, s.Banner) // visible in the taskbar too
			}
		}
	case agentsvc.EventRaise:
		app.raise()
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

// logoffOwnSession logs off the helper's session without force, as the
// service asks after the countdown (D-018); it refuses any other session.
func logoffOwnSession(session uint32) error {
	own, err := platform.CurrentSession()
	if err != nil {
		return err
	}
	if own != session {
		return fmt.Errorf("log off session %d: the helper runs in session %d", session, own)
	}
	p, err := platform.NewPower()
	if err != nil {
		return err
	}
	return p.Logoff(own, false)
}
