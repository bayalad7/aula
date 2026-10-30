package power

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Defaults of the countdown.
const (
	// DefaultFlushWait is how long the agent waits, after queueing power.ack
	// executing, for the connection to write it before the OS cuts it.
	DefaultFlushWait = 2 * time.Second
	// staleAfter clears an "executing" notice when the computer is still on:
	// without force, the student may cancel in Windows' own screen.
	staleAfter = 2 * time.Minute
	// sendTimeout bounds queueing a message on a busy connection.
	sendTimeout = 5 * time.Second
)

// Errors of Postpone.
var (
	ErrNoOrder     = errors.New("no hay ninguna orden de apagado en cuenta regresiva")
	ErrNoPostpones = errors.New("ya no se puede posponer")
)

// Sender sends a message to the console of the connection the order came by.
type Sender func(ctx context.Context, p protocol.Payload) error

// NoticeState is what the student's notice shows.
type NoticeState string

// Notice states.
const (
	NoticeCountdown NoticeState = "countdown" // counting; may postpone
	NoticeExecuting NoticeState = "executing" // the OS is doing it
	NoticeFailed    NoticeState = "failed"    // the OS refused
)

// Notice is the student's power notice (docs/privacidad.md rules 26-28).
type Notice struct {
	ActionID      string             `json:"action_id"`
	Action        protocol.PowerKind `json:"action"`
	State         NoticeState        `json:"state"`
	RemainingS    int                `json:"remaining_s"`
	PostponesLeft int                `json:"postpones_left"`
	Force         bool               `json:"force"`
}

// Countdown is the agent side of power.*: one order at a time.
type Countdown struct {
	Power platform.Power // nil: the agent does not declare power
	// Session reports the student's session; false when nobody is logged on
	// (the service without a helper): shutdown and reboot then run at once.
	// Nil means always logged on (the agent runs in the session).
	Session func() (uint32, bool)
	// Logoff, if set, ends the session without force instead of Power (the
	// service asks its helper, D-018).
	Logoff func(session uint32) error
	// OnChange is called with the notice to show, nil to remove it, before
	// the console hears about it.
	OnChange  func(*Notice)
	Clock     Clock         // real time if nil
	FlushWait time.Duration // DefaultFlushWait if 0
	Logger    *slog.Logger

	mu  sync.Mutex
	cur *order
}

// order is the power.action in force.
type order struct {
	refID     string // id of the power.action envelope
	p         protocol.PowerAction
	send      Sender
	deadline  time.Time
	left      int // postponements left
	gen       int // which timer is the current one
	timer     Timer
	executing bool
	failed    bool
}

// Supported reports whether the agent may declare power.
func (c *Countdown) Supported() bool { return c != nil && c.Power != nil }

// Handle answers power.action and power.cancel; other messages are not its.
func (c *Countdown) Handle(ctx context.Context, send Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	switch p := env.Payload.(type) {
	case protocol.PowerAction:
		if !c.Supported() {
			return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta power"), true
		}
		return c.start(ctx, send, env.ID, p), true
	case protocol.PowerCancel:
		if !c.Supported() {
			return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta power"), true
		}
		return c.cancel(env.ID, p.ActionID), true
	}
	return nil, false
}

func (c *Countdown) start(ctx context.Context, send Sender, refID string, p protocol.PowerAction) protocol.Payload {
	_, logged := c.session()
	if !logged && p.Action == protocol.PowerLogoff {
		return protocol.NewError(refID, protocol.CodeUnsupportedCapability, "no hay sesión iniciada")
	}
	c.mu.Lock()
	if c.cur != nil && c.cur.executing && !c.cur.failed {
		c.mu.Unlock()
		return protocol.NewError(refID, protocol.CodeInternal, "el equipo ya se está apagando")
	}
	old := c.replaceLocked()
	o := &order{refID: refID, p: p, send: send, left: protocol.MaxPowerPostpones}
	c.cur = o
	immediate := p.Force || !logged
	var n *Notice
	if !immediate {
		delay := time.Duration(p.Delay()) * time.Second
		o.deadline = c.clock().Now().Add(delay)
		c.armLocked(o, delay)
		n = c.noticeLocked(o)
	}
	c.mu.Unlock()

	if old != nil {
		c.ack(ctx, old, protocol.PowerCancelled, old.refID)
	}
	c.log().Info("power order", "action", p.Action, "action_id", p.ActionID, "force", p.Force, "logged_on", logged)
	if immediate {
		go c.execute(o)
		return nil
	}
	c.notify(n) // the notice first (docs/privacidad.md rule 26)
	return protocol.PowerAck{RefID: refID, ActionID: p.ActionID, Status: protocol.PowerCountdown,
		RemainingS: p.Delay(), PostponesLeft: o.left}
}

// replaceLocked stops the order in force, if any, and returns it unless it
// had already failed (nothing to cancel then).
func (c *Countdown) replaceLocked() *order {
	o := c.cur
	c.cur = nil
	if o == nil {
		return nil
	}
	if o.timer != nil {
		o.timer.Stop()
	}
	o.gen++
	if o.failed {
		return nil
	}
	return o
}

func (c *Countdown) cancel(refID, actionID string) protocol.Payload {
	c.mu.Lock()
	o := c.cur
	if o == nil || o.p.ActionID != actionID || o.executing {
		c.mu.Unlock()
		return nil // nothing in force with that ID: ignored (§5.15)
	}
	c.replaceLocked()
	c.mu.Unlock()
	c.log().Info("power order cancelled", "action_id", actionID)
	c.notify(nil)
	return protocol.PowerAck{RefID: refID, ActionID: actionID, Status: protocol.PowerCancelled}
}

// Postpone adds 5 minutes to the countdown, at most twice per order
// (docs/privacidad.md rule 27).
func (c *Countdown) Postpone() error {
	c.mu.Lock()
	o := c.cur
	if o == nil || o.executing {
		c.mu.Unlock()
		return ErrNoOrder
	}
	if o.left == 0 {
		c.mu.Unlock()
		return ErrNoPostpones
	}
	if o.timer != nil {
		o.timer.Stop()
	}
	o.left--
	o.deadline = o.deadline.Add(protocol.PowerPostponeS * time.Second)
	c.armLocked(o, o.deadline.Sub(c.clock().Now()))
	n := c.noticeLocked(o)
	c.mu.Unlock()
	c.log().Info("power order postponed", "action_id", n.ActionID, "postpones_left", n.PostponesLeft)
	c.notify(n)
	c.ack(context.Background(), o, protocol.PowerPostponed, o.refID)
	return nil
}

// Notice returns the notice to show now, nil if none.
func (c *Countdown) Notice() *Notice {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cur == nil {
		return nil
	}
	return c.noticeLocked(c.cur)
}

// Dismiss removes a failed notice.
func (c *Countdown) Dismiss() {
	c.mu.Lock()
	if c.cur == nil || !c.cur.failed {
		c.mu.Unlock()
		return
	}
	c.cur = nil
	c.mu.Unlock()
	c.notify(nil)
}

func (c *Countdown) armLocked(o *order, d time.Duration) {
	o.gen++
	gen := o.gen
	o.timer = c.clock().AfterFunc(d, func() { c.fire(o, gen) })
}

func (c *Countdown) fire(o *order, gen int) {
	c.mu.Lock()
	ok := c.cur == o && o.gen == gen && !o.executing
	c.mu.Unlock()
	if ok {
		c.execute(o)
	}
}

// execute shows that the computer is going down, logs it, tells the console
// and only then calls the OS.
func (c *Countdown) execute(o *order) {
	c.mu.Lock()
	if c.cur != o || o.executing {
		c.mu.Unlock()
		return
	}
	o.executing = true
	n := c.noticeLocked(o)
	c.mu.Unlock()
	c.notify(n)

	c.log().Info("power executing", "action", o.p.Action, "action_id", o.p.ActionID, "force", o.p.Force)
	if c.ack(context.Background(), o, protocol.PowerExecuting, o.refID) {
		time.Sleep(c.flushWait()) // the ack must leave before the OS cuts the connection
	}
	if err := c.run(o.p); err != nil {
		c.log().Warn("power action failed", "action", o.p.Action, "action_id", o.p.ActionID, "err", err)
		c.mu.Lock()
		o.failed = true
		n := c.noticeLocked(o)
		c.mu.Unlock()
		c.notify(n)
		c.sendTo(context.Background(), o, protocol.NewError(o.refID, protocol.CodeInternal, failText(o.p.Action)))
		return
	}
	// Still running after a while: the student kept the session in
	// Windows' own screen, or the log off did not happen. Clear the notice.
	c.clock().AfterFunc(staleAfter, func() {
		c.mu.Lock()
		stale := c.cur == o
		if stale {
			c.cur = nil
		}
		c.mu.Unlock()
		if stale {
			c.notify(nil)
		}
	})
}

func (c *Countdown) run(p protocol.PowerAction) error {
	switch p.Action {
	case protocol.PowerShutdown:
		return c.Power.Shutdown(p.Force)
	case protocol.PowerReboot:
		return c.Power.Reboot(p.Force)
	case protocol.PowerLogoff:
		sid, ok := c.session()
		if !ok {
			return errors.New("no session to log off")
		}
		if !p.Force && c.Logoff != nil {
			return c.Logoff(sid)
		}
		return c.Power.Logoff(sid, p.Force)
	}
	return fmt.Errorf("unknown power action %q", p.Action)
}

// ack sends a power.ack; it reports whether it was queued. A lost
// connection only goes to the log: the countdown goes on (rule 30).
func (c *Countdown) ack(ctx context.Context, o *order, st protocol.PowerStatus, refID string) bool {
	a := protocol.PowerAck{RefID: refID, ActionID: o.p.ActionID, Status: st}
	if st == protocol.PowerCountdown || st == protocol.PowerPostponed {
		c.mu.Lock()
		a.RemainingS = c.remainingLocked(o)
		a.PostponesLeft = o.left
		c.mu.Unlock()
	}
	return c.sendTo(ctx, o, a)
}

func (c *Countdown) sendTo(ctx context.Context, o *order, p protocol.Payload) bool {
	if o.send == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if err := o.send(ctx, p); err != nil {
		c.log().Info("power message not sent", "type", p.MessageType(), "action_id", o.p.ActionID, "err", err)
		return false
	}
	return true
}

func (c *Countdown) noticeLocked(o *order) *Notice {
	n := &Notice{ActionID: o.p.ActionID, Action: o.p.Action, State: NoticeCountdown,
		PostponesLeft: o.left, Force: o.p.Force}
	switch {
	case o.failed:
		n.State = NoticeFailed
	case o.executing:
		n.State = NoticeExecuting
	default:
		n.RemainingS = c.remainingLocked(o)
	}
	return n
}

func (c *Countdown) remainingLocked(o *order) int {
	if o.deadline.IsZero() {
		return 0
	}
	d := o.deadline.Sub(c.clock().Now())
	if d < 0 {
		return 0
	}
	return int((d + time.Second - 1) / time.Second) // rounded up: never shows 0 early
}

func (c *Countdown) notify(n *Notice) {
	if c.OnChange != nil {
		c.OnChange(n)
	}
}

func (c *Countdown) session() (uint32, bool) {
	if c.Session == nil {
		return 0, true
	}
	return c.Session()
}

func (c *Countdown) clock() Clock {
	if c.Clock == nil {
		return realClock{}
	}
	return c.Clock
}

func (c *Countdown) flushWait() time.Duration {
	if c.FlushWait <= 0 {
		return DefaultFlushWait
	}
	return c.FlushWait
}

func (c *Countdown) log() *slog.Logger {
	if c.Logger == nil {
		return slog.Default()
	}
	return c.Logger
}

func failText(k protocol.PowerKind) string {
	switch k {
	case protocol.PowerReboot:
		return "no se pudo reiniciar el equipo"
	case protocol.PowerLogoff:
		return "no se pudo cerrar la sesión"
	}
	return "no se pudo apagar el equipo"
}
