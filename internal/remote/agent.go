package remote

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/screen"
)

// IndicatorText is the native always-visible notice while the computer is
// controlled (docs/privacidad.md rule 38). It replaces "your screen is being
// viewed", which it implies.
const IndicatorText = "El profesor está controlando tu equipo"

// Defaults of the agent's timers (docs/protocol.md §5.16).
const (
	DefaultPauseFor  = 2 * time.Second        // without the student's input before resuming
	DefaultIdleAfter = 5 * time.Minute        // without remote.input before ending
	DefaultPollEvery = 200 * time.Millisecond // how often the student's own input is checked
	DefaultFailAfter = 30 * time.Second       // of refused input before ending
)

// Video is the screen stream a control borrows: screen.Streamer.
type Video interface {
	StartControl(ctx context.Context, send screen.Sender, refID string, p protocol.ScreenThumbStart, text string) error
	RetuneControl(p protocol.ScreenThumbStart)
	StopControl()
}

// State is what the student's window shows.
type State int

// States of the control.
const (
	Off    State = iota // no control
	Active              // the teacher is controlling
	Paused              // the teacher is controlling, paused while the student uses the computer
)

// Agent is the agent side of remote control.
type Agent struct {
	// Input injects in the student's session; nil when the OS cannot, and
	// then the agent does not declare remote.
	Input platform.InputInjector
	// Video is required with Input: without the screen, there is no control.
	Video Video
	// OnState shows the notice in the window (band, raise) before any input
	// is accepted, and removes it at the end. Called with the agent locked:
	// it must not call back into the Agent.
	OnState func(State)
	Logger  *slog.Logger

	// Timers; zero means the default. Tests shorten them.
	PauseFor, IdleAfter, PollEvery, FailAfter time.Duration
	Now                                       func() time.Time

	mu sync.Mutex
	c  *control
}

// control is the control in force.
type control struct {
	id, refID string
	send      screen.Sender
	sendCtx   context.Context // the connection's: acks still go out after the end
	ctx       context.Context // the watcher's, canceled at the end
	cancel    context.CancelFunc

	started, lastInput, failingSince time.Time
	paused                           bool
	lastSeq                          int
	winStart                         time.Time
	winCount                         int
	keys                             map[string]bool
	buttons                          map[platform.MouseButton]bool
	x, y                             float64
	injected, dropped                int
}

// Available reports whether this agent can be controlled.
func (a *Agent) Available() bool { return a.Input != nil && a.Video != nil }

// Capabilities lists what the agent may declare in hello.
func (a *Agent) Capabilities() []protocol.Capability {
	if !a.Available() {
		return nil
	}
	return []protocol.Capability{protocol.CapRemote}
}

// Handle processes remote.start, remote.input and remote.stop. It reports
// false for any other message. reply is the message to send back, if any.
func (a *Agent) Handle(ctx context.Context, send screen.Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	switch p := env.Payload.(type) {
	case protocol.RemoteStart:
		if !a.Available() {
			return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta remote"), true
		}
		return a.start(ctx, send, env.ID, p), true
	case protocol.RemoteInput:
		a.input(p)
		return nil, true
	case protocol.RemoteStop:
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.c == nil || a.c.id != p.ControlID {
			return nil, true
		}
		return a.endLocked(protocol.RemoteByTeacher, env.ID), true
	}
	return nil, false
}

// Stop ends the control, if any, without telling the console: the agent
// calls it when the connection closes. Idempotent.
func (a *Agent) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c != nil {
		a.c.cancel()
		a.finishLocked("connection closed")
	}
}

func videoParams(p protocol.RemoteStart) protocol.ScreenThumbStart {
	return protocol.ScreenThumbStart{IntervalMS: p.IntervalMS, Quality: p.Quality, MaxWidth: p.MaxWidth}
}

func (a *Agent) start(ctx context.Context, send screen.Sender, envID string, p protocol.RemoteStart) protocol.Payload {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c != nil && a.c.id == p.ControlID {
		a.Video.RetuneControl(videoParams(p))
		return nil
	}
	if a.c != nil {
		old := a.c
		ack := a.endLocked(protocol.RemoteReplaced, old.refID)
		if err := old.send(old.sendCtx, ack); err != nil {
			a.log().Debug("remote ack not sent", "err", err)
		}
	}
	// The notice first: no input is accepted before the student sees it.
	a.state(Active)
	if err := a.Video.StartControl(ctx, send, envID, videoParams(p), IndicatorText); err != nil {
		a.state(Off)
		a.log().Warn("remote control not started", "err", err)
		return protocol.NewError(envID, protocol.CodeInternal, "no se pudo mostrar el aviso de control remoto")
	}
	now := a.now()
	cctx, cancel := context.WithCancel(ctx)
	c := &control{id: p.ControlID, refID: envID, send: send, sendCtx: ctx, ctx: cctx, cancel: cancel,
		started: now, lastInput: now, keys: map[string]bool{}, buttons: map[platform.MouseButton]bool{}}
	a.c = c
	go a.watch(c)
	a.log().Info("remote control started", "control_id", c.id)
	return protocol.RemoteAck{RefID: envID, ControlID: c.id, Status: protocol.RemoteActive}
}

// input injects one remote.input, unless it must be dropped.
func (a *Agent) input(p protocol.RemoteInput) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c := a.c
	if c == nil || c.id != p.ControlID {
		return
	}
	now := a.now()
	if p.Seq <= c.lastSeq {
		a.drop(c)
		return
	}
	c.lastSeq = p.Seq
	if now.Sub(c.winStart) >= time.Second {
		c.winStart, c.winCount = now, 0
	}
	c.winCount++
	c.lastInput = now // the teacher is there, even while paused
	if c.winCount > protocol.MaxRemoteRate || c.paused {
		a.drop(c)
		return
	}
	events := make([]platform.InputEvent, 0, len(p.Events))
	for _, e := range p.Events {
		events = append(events, c.track(e))
	}
	err := a.Input.Inject(events)
	switch {
	case err == nil:
		c.failingSince = time.Time{}
		c.injected += len(events)
	case errors.Is(err, agentsvc.ErrNoHelper) || errors.Is(err, agentsvc.ErrClosed):
		a.endAndSend(c, protocol.RemoteSession)
	default:
		if c.failingSince.IsZero() {
			c.failingSince = now
			a.log().Warn("remote input refused by the OS", "control_id", c.id, "err", err)
		} else if now.Sub(c.failingSince) >= a.failAfter() {
			a.endAndSend(c, protocol.RemoteError)
		}
	}
}

func (a *Agent) drop(c *control) {
	if c.dropped == 0 {
		a.log().Info("remote input dropped (paused, too fast or out of order)", "control_id", c.id)
	}
	c.dropped++
}

// track converts an event and remembers what stays pressed.
func (c *control) track(e protocol.InputEvent) platform.InputEvent {
	switch e.T {
	case protocol.InputMove:
		c.x, c.y = e.X, e.Y
		return platform.InputEvent{Kind: platform.InputMove, X: e.X, Y: e.Y}
	case protocol.InputButton:
		c.x, c.y = e.X, e.Y
		b := map[string]platform.MouseButton{protocol.ButtonLeft: platform.ButtonLeft,
			protocol.ButtonRight: platform.ButtonRight, protocol.ButtonMiddle: platform.ButtonMiddle}[e.B]
		if e.Down {
			c.buttons[b] = true
		} else {
			delete(c.buttons, b)
		}
		return platform.InputEvent{Kind: platform.InputButton, Button: b, Down: e.Down, X: e.X, Y: e.Y}
	case protocol.InputWheel:
		return platform.InputEvent{Kind: platform.InputWheel, DX: e.DX, DY: e.DY}
	default: // key; the message was validated
		if e.Down {
			c.keys[e.Code] = true
		} else {
			delete(c.keys, e.Code)
		}
		return platform.InputEvent{Kind: platform.InputKey, Code: e.Code, Down: e.Down}
	}
}

// release lets go of every key and button the teacher left pressed.
func (a *Agent) release(c *control) {
	var up []platform.InputEvent
	codes := make([]string, 0, len(c.keys))
	for k := range c.keys {
		codes = append(codes, k)
	}
	sort.Strings(codes)
	for _, k := range codes {
		up = append(up, platform.InputEvent{Kind: platform.InputKey, Code: k})
	}
	for _, b := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight, platform.ButtonMiddle} {
		if c.buttons[b] {
			up = append(up, platform.InputEvent{Kind: platform.InputButton, Button: b, X: c.x, Y: c.y})
		}
	}
	clear(c.keys)
	clear(c.buttons)
	if len(up) == 0 {
		return
	}
	if err := a.Input.Inject(up); err != nil {
		a.log().Warn("cannot release the remote keys and buttons", "control_id", c.id, "err", err)
	}
}

// watch pauses and resumes the teacher while the student uses the computer,
// and ends an idle control.
func (a *Agent) watch(c *control) {
	t := time.NewTicker(a.pollEvery())
	defer t.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-t.C:
		}
		local, lerr := a.Input.LocalInput()
		a.mu.Lock()
		if a.c != c {
			a.mu.Unlock()
			return
		}
		now := a.now()
		var ack protocol.Payload
		switch {
		case now.Sub(c.lastInput) >= a.idleAfter():
			ack = a.endLocked(protocol.RemoteIdle, c.refID)
		case errors.Is(lerr, agentsvc.ErrNoHelper) || errors.Is(lerr, agentsvc.ErrClosed):
			ack = a.endLocked(protocol.RemoteSession, c.refID)
		case lerr != nil:
			// Not knowing is not a reason to pause; try again.
		default:
			recent := !local.IsZero() && !local.Before(c.started) && now.Sub(local) < a.pauseFor()
			if recent && !c.paused {
				c.paused = true
				a.release(c)
				a.state(Paused)
				ack = protocol.RemoteAck{RefID: c.refID, ControlID: c.id, Status: protocol.RemotePaused}
			} else if !recent && c.paused {
				c.paused = false
				a.state(Active)
				ack = protocol.RemoteAck{RefID: c.refID, ControlID: c.id, Status: protocol.RemoteResumed}
			}
		}
		a.mu.Unlock()
		if ack != nil {
			if err := c.send(c.sendCtx, ack); err != nil {
				a.log().Debug("remote ack not sent", "err", err)
			}
		}
	}
}

// endAndSend ends c and tells the console. a.mu is held.
func (a *Agent) endAndSend(c *control, reason protocol.RemoteReason) {
	ack := a.endLocked(reason, c.refID)
	if err := c.send(c.sendCtx, ack); err != nil {
		a.log().Debug("remote ack not sent", "err", err)
	}
}

// endLocked ends the control in force and returns its ended ack. a.mu is
// held; the watcher goroutine sees a.c change and leaves on its own.
func (a *Agent) endLocked(reason protocol.RemoteReason, refID string) protocol.RemoteAck {
	c := a.c
	c.cancel()
	a.finishLocked(string(reason))
	return protocol.RemoteAck{RefID: refID, ControlID: c.id, Status: protocol.RemoteEnded, Reason: reason}
}

// finishLocked releases what was pressed, stops the video and removes the
// notice. a.mu is held.
func (a *Agent) finishLocked(why string) {
	c := a.c
	a.c = nil
	a.release(c)
	a.Video.StopControl()
	a.state(Off)
	a.log().Info("remote control ended", "control_id", c.id, "reason", why, "events", c.injected, "dropped", c.dropped)
}

func (a *Agent) state(s State) {
	if a.OnState != nil {
		a.OnState(s)
	}
}

func (a *Agent) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func orDefault(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

func (a *Agent) pauseFor() time.Duration  { return orDefault(a.PauseFor, DefaultPauseFor) }
func (a *Agent) idleAfter() time.Duration { return orDefault(a.IdleAfter, DefaultIdleAfter) }
func (a *Agent) pollEvery() time.Duration { return orDefault(a.PollEvery, DefaultPollEvery) }
func (a *Agent) failAfter() time.Duration { return orDefault(a.FailAfter, DefaultFailAfter) }

func (a *Agent) log() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.New(slog.DiscardHandler)
}
