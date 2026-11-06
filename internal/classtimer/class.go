package classtimer

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// Errors of the console side.
var (
	ErrInvalid    = errors.New("invalid timer")
	ErrNotRunning = errors.New("no timer in force")
	ErrTooLong    = errors.New("the timer would pass the limit")
)

// Command is a message to send to one session.
type Command struct {
	SessionID string
	Payload   protocol.Payload
}

// Receiver is a student the timer was sent to, and what their agent said.
type Receiver struct {
	SessionID string `json:"sessionId"`
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	Simulated bool   `json:"simulated"`
	// Status is "sent" (no ack yet), "running" or "ended".
	Status string `json:"status"`
}

// Status of a receiver.
const (
	ReceiverSent    = "sent"
	ReceiverRunning = "running"
	ReceiverEnded   = "ended"
)

// Snapshot is the timer for the console: the franja and «Participación».
type Snapshot struct {
	Active      bool     `json:"active"`
	TimerID     string   `json:"timerId"`
	Label       string   `json:"label"`
	DurationS   int      `json:"durationS"`
	RemainingMS int64    `json:"remainingMs"`
	Ended       bool     `json:"ended"`
	All         bool     `json:"all"`   // for the whole channel, or a selection
	Hosts       []string `json:"hosts"` // the selection; empty with All
	// MaxExtendS is how much it can still be extended without passing 4 h.
	MaxExtendS  int        `json:"maxExtendS"`
	Receivers   []Receiver `json:"receivers"`
	Eligible    int        `json:"eligible"`    // connected targets that can show it
	Unsupported []string   `json:"unsupported"` // connected targets that cannot
}

type sentState struct {
	timerID string
	version int
	status  string
}

// Class is the console side of the timer: one per class. It is safe for
// concurrent use.
type Class struct {
	mu  sync.Mutex
	now func() time.Time

	id        string
	label     string
	durationS int
	end       time.Time // monotonic
	version   int       // increases with every extension
	lastAddS  int
	hosts     map[string]bool // nil: the whole channel

	sent map[string]*sentState // by session ID
}

// NewClass returns a class without a timer. now is time.Now when nil.
func NewClass(now func() time.Time) *Class {
	if now == nil {
		now = time.Now
	}
	return &Class{now: now, sent: map[string]*sentState{}}
}

// Start replaces the timer in force by a new one of durationS seconds, for
// hosts (hostnames, without case) or the whole channel when hosts is empty.
// Plan then says what to send.
func (c *Class) Start(label string, durationS int, hosts []string) (string, error) {
	label = strings.TrimSpace(label)
	if utf8.RuneCountInString(label) > protocol.MaxTimerLabel {
		return "", fmt.Errorf("%w: label over %d characters", ErrInvalid, protocol.MaxTimerLabel)
	}
	if durationS < protocol.MinTimerDurationS || durationS > protocol.MaxTimerDurationS {
		return "", fmt.Errorf("%w: duration %d s", ErrInvalid, durationS)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.id, c.label, c.durationS, c.version, c.lastAddS = protocol.NewID(), label, durationS, 0, 0
	c.end = c.now().Add(time.Duration(durationS) * time.Second)
	c.hosts = nil
	for _, h := range hosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			if c.hosts == nil {
				c.hosts = map[string]bool{}
			}
			c.hosts[h] = true
		}
	}
	return c.id, nil
}

// Extend adds addS seconds to what is left (and revives an ended timer).
func (c *Class) Extend(addS int) error {
	if addS < protocol.MinTimerExtendS || addS > protocol.MaxTimerExtendS {
		return fmt.Errorf("%w: extension %d s", ErrInvalid, addS)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.id == "" {
		return ErrNotRunning
	}
	now := c.now()
	left := max(c.end.Sub(now), 0)
	next := left + time.Duration(addS)*time.Second
	if next.Milliseconds() > protocol.MaxTimerRemaining {
		return fmt.Errorf("%w: %d s left", ErrTooLong, next/time.Second)
	}
	c.end = now.Add(next)
	c.version++
	c.lastAddS = addS
	return nil
}

// Stop removes the timer; Plan then sends timer.stop to who had it.
func (c *Class) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.id == "" {
		return ErrNotRunning
	}
	c.id, c.label, c.durationS, c.version, c.lastAddS, c.hosts = "", "", 0, 0, 0, nil
	return nil
}

// Reset forgets everything (the class closed); nothing is sent.
func (c *Class) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.id, c.label, c.durationS, c.version, c.lastAddS, c.hosts = "", "", 0, 0, 0, nil
	c.sent = map[string]*sentState{}
}

// Remaining is what is left of the timer in force; false when there is none.
func (c *Class) Remaining() (time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.id == "" {
		return 0, false
	}
	return max(c.end.Sub(c.now()), 0), true
}

// Plan returns what must be sent so every connected session that can show it
// has the timer in force (a start, an extension or a stop), real or
// simulated, and forgets sessions that left. What it returns counts as sent:
// call Failed for a command that could not be sent.
func (c *Class) Plan(sessions []session.Session) []Command {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	left := c.end.Sub(now)
	live := map[string]bool{}
	var cmds []Command
	for _, s := range sessions {
		if s.State != session.StateConnected || !s.Can(protocol.CapTimer) {
			continue
		}
		live[s.ID] = true
		have := c.sent[s.ID]
		switch {
		case c.id == "" || !c.targets(s.Hostname):
			if have != nil {
				cmds = append(cmds, Command{s.ID, protocol.TimerStop{TimerID: have.timerID}})
				delete(c.sent, s.ID)
			}
		case have != nil && have.timerID == c.id && have.version == c.version:
			// up to date
		case have != nil && have.timerID == c.id:
			if left > 0 {
				cmds = append(cmds, Command{s.ID, protocol.TimerExtend{TimerID: c.id, AddS: c.lastAddS, RemainingMS: left.Milliseconds()}})
				have.version, have.status = c.version, ReceiverSent
			}
		case left >= time.Millisecond:
			// New, reconnected, or it had a replaced timer: the agent acks
			// stopped for the old one when the new one arrives.
			cmds = append(cmds, Command{s.ID, protocol.TimerStart{TimerID: c.id, Label: c.label, DurationS: c.durationS, RemainingMS: left.Milliseconds()}})
			c.sent[s.ID] = &sentState{timerID: c.id, version: c.version, status: ReceiverSent}
		}
	}
	for id := range c.sent {
		if !live[id] {
			delete(c.sent, id)
		}
	}
	return cmds
}

// Failed undoes what Plan recorded for a command that could not be sent, so
// the next Plan tries again.
func (c *Class) Failed(cmd Command) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := cmd.Payload.(protocol.TimerStop); ok {
		return // the agent drops it by itself if it does not hear from us
	}
	delete(c.sent, cmd.SessionID)
}

// Ack records what an agent said about the timer in force. It reports
// whether the view changed.
func (c *Class) Ack(sessionID string, p protocol.TimerAck) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	have := c.sent[sessionID]
	if have == nil || have.timerID != p.TimerID || p.TimerID != c.id {
		return false
	}
	st := ReceiverRunning
	switch p.Status {
	case protocol.TimerEnded:
		st = ReceiverEnded
	case protocol.TimerStopped:
		return false // the ack of a replaced timer
	}
	if have.status == st {
		return false
	}
	have.status = st
	return true
}

// Snapshot returns the timer for the console. real and sims are the class's
// sessions, to tell who receives it and who cannot.
func (c *Class) Snapshot(real, sims []session.Session) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := Snapshot{Hosts: []string{}, Receivers: []Receiver{}, Unsupported: []string{}}
	if c.id == "" {
		return out
	}
	left := max(c.end.Sub(c.now()), 0)
	out.Active, out.TimerID, out.Label, out.DurationS = true, c.id, c.label, c.durationS
	out.RemainingMS, out.Ended, out.All = left.Milliseconds(), left == 0, c.hosts == nil
	out.MaxExtendS = int(max(time.Duration(protocol.MaxTimerRemaining)*time.Millisecond-left, 0) / time.Second)
	for h := range c.hosts {
		out.Hosts = append(out.Hosts, h)
	}
	slices.Sort(out.Hosts)
	add := func(list []session.Session, simulated bool) {
		for _, s := range list {
			if s.State != session.StateConnected || !c.targets(s.Hostname) {
				continue
			}
			if !s.Can(protocol.CapTimer) {
				out.Unsupported = append(out.Unsupported, s.Hostname)
				continue
			}
			out.Eligible++
			if have := c.sent[s.ID]; have != nil && have.timerID == c.id {
				out.Receivers = append(out.Receivers, Receiver{SessionID: s.ID, Hostname: s.Hostname,
					Username: s.Username, Simulated: simulated, Status: have.status})
			}
		}
	}
	add(real, false)
	add(sims, true)
	slices.SortFunc(out.Receivers, func(a, b Receiver) int { return strings.Compare(a.Hostname, b.Hostname) })
	slices.Sort(out.Unsupported)
	return out
}

// targets reports whether the timer in force is for hostname; c.mu is held.
func (c *Class) targets(hostname string) bool {
	return c.hosts == nil || c.hosts[strings.ToLower(hostname)]
}
