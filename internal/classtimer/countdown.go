package classtimer

import (
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// How long the agent waits for the console to send the timer again after a
// reconnection, and how long the «terminó» badge stays without timer.stop.
const (
	ReconnectGrace = 3 * time.Second
	EndedLinger    = 5 * time.Minute
)

// State is what the student's window shows.
type State string

// States of the countdown.
const (
	StateNone    State = "none"    // no badge
	StateRunning State = "running" // the badge counts
	StateEnded   State = "ended"   // the badge says it ended
)

// View is the countdown for the window: states, never sentences.
type View struct {
	State       State  `json:"state"`
	TimerID     string `json:"timerId"`
	Label       string `json:"label"`
	RemainingMS int64  `json:"remainingMs"`
	// JustEnded is set only in the view that announces the end, so the window
	// shows the notice once.
	JustEnded bool `json:"justEnded"`
}

// Countdown is the agent side of the timer. It is safe for concurrent use.
// The app calls Tick whenever Next says so.
type Countdown struct {
	mu  sync.Mutex
	now func() time.Time

	id      string
	label   string
	end     time.Time // monotonic
	refID   string    // the last start or extend, for the ended ack
	ended   bool
	endedAt time.Time

	orphan   bool // a new session started: the console has not confirmed it yet
	orphanAt time.Time
}

// NewCountdown returns an empty countdown. now is time.Now when nil.
func NewCountdown(now func() time.Time) *Countdown {
	if now == nil {
		now = time.Now
	}
	return &Countdown{now: now}
}

// Start applies a timer.start (refID is its envelope ID) and returns the
// acks to send: a stopped one for the timer it replaces, if any, and running.
func (c *Countdown) Start(refID string, p protocol.TimerStart) []protocol.TimerAck {
	c.mu.Lock()
	defer c.mu.Unlock()
	var acks []protocol.TimerAck
	if c.id != "" && c.id != p.TimerID {
		acks = append(acks, protocol.TimerAck{RefID: c.refID, TimerID: c.id, Status: protocol.TimerStopped})
	}
	now := c.now()
	c.id, c.label, c.refID = p.TimerID, p.Label, refID
	c.end = now.Add(time.Duration(p.RemainingMS) * time.Millisecond)
	c.ended, c.orphan = false, false
	return append(acks, c.ack(refID, protocol.TimerRunning, now))
}

// Extend applies a timer.extend: it takes what is left from the message, so
// a resend never extends twice. It is ignored (false) for another timer.
func (c *Countdown) Extend(refID string, p protocol.TimerExtend) (protocol.TimerAck, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.id == "" || c.id != p.TimerID {
		return protocol.TimerAck{}, false
	}
	now := c.now()
	c.refID = refID
	c.end = now.Add(time.Duration(p.RemainingMS) * time.Millisecond)
	c.ended, c.orphan = false, false
	return c.ack(refID, protocol.TimerRunning, now), true
}

// Stop applies a timer.stop. It is ignored (false) for another timer or none.
func (c *Countdown) Stop(refID string, p protocol.TimerStop) (protocol.TimerAck, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.id == "" || c.id != p.TimerID {
		return protocol.TimerAck{}, false
	}
	ack := protocol.TimerAck{RefID: refID, TimerID: c.id, Status: protocol.TimerStopped}
	c.clear()
	return ack, true
}

// Reconnected tells that a new session with the console started: the count
// goes on, but if the console does not send the timer again within
// ReconnectGrace, it is dropped (it was stopped or ended meanwhile).
func (c *Countdown) Reconnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.id != "" {
		c.orphan, c.orphanAt = true, c.now()
	}
}

// Reset drops the timer (the class closed for good).
func (c *Countdown) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clear()
}

// Tick moves the countdown on: at zero it ends once (ended, with the ack to
// send), and it drops an ended badge after EndedLinger or an unconfirmed one
// after ReconnectGrace. changed says whether the window must be told.
func (c *Countdown) Tick() (ack protocol.TimerAck, ended, changed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.id == "" {
		return protocol.TimerAck{}, false, false
	}
	now := c.now()
	if c.orphan && now.Sub(c.orphanAt) >= ReconnectGrace {
		c.clear()
		return protocol.TimerAck{}, false, true
	}
	if !c.ended && !now.Before(c.end) {
		c.ended, c.endedAt = true, now
		return protocol.TimerAck{RefID: c.refID, TimerID: c.id, Status: protocol.TimerEnded}, true, true
	}
	if c.ended && now.Sub(c.endedAt) >= EndedLinger {
		c.clear()
		return protocol.TimerAck{}, false, true
	}
	return protocol.TimerAck{}, false, false
}

// Next says how long until Tick has something to do; false when nothing is
// pending.
func (c *Countdown) Next() (time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.id == "" {
		return 0, false
	}
	now := c.now()
	var at time.Time
	if c.ended {
		at = c.endedAt.Add(EndedLinger)
	} else {
		at = c.end
	}
	if c.orphan {
		if g := c.orphanAt.Add(ReconnectGrace); g.Before(at) {
			at = g
		}
	}
	return max(at.Sub(now), 0), true
}

// View returns what the window shows.
func (c *Countdown) View() View {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case c.id == "":
		return View{State: StateNone}
	case c.ended:
		return View{State: StateEnded, TimerID: c.id, Label: c.label}
	}
	return View{State: StateRunning, TimerID: c.id, Label: c.label, RemainingMS: c.remaining(c.now())}
}

func (c *Countdown) ack(refID string, st protocol.TimerStatus, now time.Time) protocol.TimerAck {
	return protocol.TimerAck{RefID: refID, TimerID: c.id, Status: st, RemainingMS: c.remaining(now)}
}

func (c *Countdown) remaining(now time.Time) int64 {
	if c.ended {
		return 0
	}
	return min(max(c.end.Sub(now).Milliseconds(), 0), protocol.MaxTimerRemaining)
}

// clear drops the timer; c.mu is held (so it must not be overwritten).
func (c *Countdown) clear() {
	c.id, c.label, c.refID = "", "", ""
	c.end, c.endedAt, c.orphanAt = time.Time{}, time.Time{}, time.Time{}
	c.ended, c.orphan = false, false
}
