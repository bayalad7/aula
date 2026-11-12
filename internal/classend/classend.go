// Package classend is «Terminar la clase» (docs/protocol.md §5.25): on the
// console's side, what each step did and how each student's window was left,
// and who is owed a class.end when they reconnect (Class); on the agent's
// side, what was removed from the window and the «La clase terminó» card
// (Removed, Card, Ack). It never touches the OS: each step reuses the stop
// of its own package.
package classend

import (
	"slices"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// AckWait is how long the console waits for a class.end.ack before it marks
// the student «sin acuse» (§5.25.3).
const AckWait = 10 * time.Second

// CollectWait is how long «Recoger el trabajo antes de terminar» waits for
// the collection to finish before it goes on (§5.25).
const CollectWait = 60 * time.Second

// StepKind is one of the console's own steps, in the order it runs them.
type StepKind string

// Steps of the console (§5.25, the order of the diagram).
const (
	StepCollect      StepKind = "collect"      // optional: the last collection, before anything else
	StepBroadcast    StepKind = "broadcast"    // the teacher's screen
	StepPresent      StepKind = "present"      // a student's screen shown to the class
	StepControl      StepKind = "control"      // the remote control
	StepPause        StepKind = "pause"        // the paused screens
	StepRestrictions StepKind = "restrictions" // apps and web
	StepTimer        StepKind = "timer"
	StepPoll         StepKind = "poll"
	StepPower        StepKind = "power" // only cancelled when the teacher ticks it
)

// StepStatus is how a step ended.
type StepStatus string

// Step statuses.
const (
	StepDone    StepStatus = "done"    // it was active and was stopped
	StepFailed  StepStatus = "failed"  // it could not be stopped for some; the rest went on
	StepKept    StepStatus = "kept"    // left as it was on purpose (pending power)
	StepTimeout StepStatus = "timeout" // the collection did not finish in CollectWait
)

// StepOutcomes lists how each step can end, for the console's texts.
var StepOutcomes = map[StepKind][]StepStatus{
	StepCollect:      {StepDone, StepFailed, StepTimeout},
	StepBroadcast:    {StepDone},
	StepPresent:      {StepDone},
	StepControl:      {StepDone},
	StepPause:        {StepDone, StepFailed},
	StepRestrictions: {StepDone},
	StepTimer:        {StepDone, StepFailed},
	StepPoll:         {StepDone, StepFailed},
	StepPower:        {StepDone, StepFailed, StepKept},
}

// Step is what one step did. Count is how many computers (or, for the timer
// and the poll, 1).
type Step struct {
	Kind   StepKind   `json:"kind"`
	Status StepStatus `json:"status"`
	Count  int        `json:"count"`
}

// State is how a student's window was left.
type State string

// Student states (the dialog's result).
const (
	StateWaiting State = "waiting" // class.end sent, no answer yet
	StateClean   State = "clean"   // class.end.ack clean
	StateFailed  State = "failed"  // class.end.ack failed (Reason says what stayed)
	StateNoAck   State = "noack"   // no answer in AckWait: it gets it again when it reconnects
	StateAway    State = "away"    // disconnected when the class ended: it gets it when it reconnects
	StatePartial State = "partial" // an earlier agent: only the stops and code.clear
)

// States lists every State, for the console's texts.
var States = []State{StateWaiting, StateClean, StateFailed, StateNoAck, StateAway, StatePartial}

// Student is one computer in the result.
type Student struct {
	Hostname  string    `json:"hostname"`
	SessionID string    `json:"sessionId"` // the connection class.end went to ("" while away)
	Simulated bool      `json:"simulated"`
	State     State     `json:"state"`
	Reason    string    `json:"reason,omitempty"` // with StateFailed: the ack's reason
	At        time.Time `json:"at"`               // of the last change
}

// View is the last end of class, for the dialog.
type View struct {
	EndID    string    `json:"endId"` // "" before the first
	At       time.Time `json:"at"`
	Running  bool      `json:"running"` // the steps are still running
	Steps    []Step    `json:"steps"`
	Students []Student `json:"students"` // by hostname
	Clean    int       `json:"clean"`    // clean or partial
	Missing  int       `json:"missing"`  // waiting, noack, away or failed
}

type entry struct {
	Student
	env  string    // the class.end envelope ID, for the ack
	sent time.Time // when it was sent, for AckWait
}

// Class is the console's side. It is safe for concurrent use.
type Class struct {
	now func() time.Time

	mu       sync.Mutex
	endID    string
	at       time.Time
	running  bool
	steps    []Step
	students map[string]*entry // by hostname: a reconnection is a new session
}

// NewClass returns an empty Class; now is time.Now when nil.
func NewClass(now func() time.Time) *Class {
	if now == nil {
		now = time.Now
	}
	return &Class{now: now, students: map[string]*entry{}}
}

// Begin starts a new end of class and forgets the previous one (and whoever
// it still owed). It reports false while another is running.
func (c *Class) Begin() (endID string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return "", false
	}
	c.endID, c.at, c.running = protocol.NewID(), c.now(), true
	c.steps = nil
	c.students = map[string]*entry{}
	return c.endID, true
}

// EndID is the current end of class ("" before the first).
func (c *Class) EndID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.endID
}

// AddStep records what one of the console's steps did.
func (c *Class) AddStep(s Step) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.steps = append(c.steps, s)
}

// Done marks the steps as finished: the result stays until the next Begin.
func (c *Class) Done() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
}

func (c *Class) set(host string, sim bool) *entry {
	e, ok := c.students[host]
	if !ok {
		e = &entry{Student: Student{Hostname: host, Simulated: sim}}
		c.students[host] = e
	}
	e.At = c.now()
	return e
}

// Sent records the class.end sent to a connected session (also a resend).
func (c *Class) Sent(host, sessionID, envID string, sim bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.set(host, sim)
	e.SessionID, e.env, e.sent = sessionID, envID, e.At
	e.State, e.Reason = StateWaiting, ""
}

// Partial records an earlier agent: it got the stops and code.clear only.
func (c *Class) Partial(host, sessionID string, sim bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.set(host, sim)
	e.SessionID, e.env = sessionID, ""
	e.State, e.Reason = StatePartial, ""
}

// Away records a computer that was not connected when the class ended: it
// is owed a class.end when it reconnects.
func (c *Class) Away(host string, sim bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.students[host]; ok && e.State != StateAway {
		return // a connection of the same computer already counts
	}
	e := c.set(host, sim)
	e.SessionID, e.env = "", ""
	e.State = StateAway
}

// Ack applies a class.end.ack from sessionID. It reports whether it was the
// answer to the current end of class.
func (c *Class) Ack(sessionID string, a protocol.ClassEndAck) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a.EndID != c.endID {
		return false
	}
	for _, e := range c.students {
		if e.SessionID != sessionID || e.env != a.RefID {
			continue
		}
		e.At = c.now()
		if a.Status == protocol.ClassEndClean {
			e.State, e.Reason = StateClean, ""
		} else {
			e.State, e.Reason = StateFailed, string(a.Reason)
		}
		return true
	}
	return false
}

// Expire marks «sin acuse» whoever has waited AckWait. It reports whether
// anything changed.
func (c *Class) Expire() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now, changed := c.now(), false
	for _, e := range c.students {
		if e.State == StateWaiting && now.Sub(e.sent) >= AckWait {
			e.State, e.At, changed = StateNoAck, now, true
		}
	}
	return changed
}

// Gone marks the computer of a session that left while waiting as away: it
// gets class.end when it comes back.
func (c *Class) Gone(sessionID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.students {
		if e.SessionID == sessionID && e.State == StateWaiting {
			e.State, e.At = StateAway, c.now()
			return true
		}
	}
	return false
}

// owed reports whether e still needs a class.end.
func (e *entry) owed() bool {
	switch e.State {
	case StateWaiting, StateNoAck, StateAway, StateFailed:
		return true
	}
	return false
}

// Owed reports whether host, now connected as sessionID, must get class.end:
// it did not acknowledge it and this is a new connection (§5.25.3).
func (c *Class) Owed(host, sessionID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.students[host]
	return ok && e.owed() && e.SessionID != sessionID
}

// Retry lists the computers «Reintentar a los que faltan» sends class.end to
// again: no answer or failed (those away get it when they reconnect).
func (c *Class) Retry() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for h, e := range c.students {
		if e.State == StateNoAck || e.State == StateFailed {
			out = append(out, h)
		}
	}
	slices.Sort(out)
	return out
}

// View returns the last end of class.
func (c *Class) View() View {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := View{EndID: c.endID, At: c.at, Running: c.running, Steps: slices.Clone(c.steps), Students: []Student{}}
	if v.Steps == nil {
		v.Steps = []Step{}
	}
	for _, e := range c.students {
		v.Students = append(v.Students, e.Student)
		if e.owed() {
			v.Missing++
		} else {
			v.Clean++
		}
	}
	slices.SortFunc(v.Students, func(a, b Student) int {
		switch {
		case a.Hostname < b.Hostname:
			return -1
		case a.Hostname > b.Hostname:
			return 1
		}
		return 0
	})
	return v
}

// Reset forgets everything: a change of channel or the console closing.
func (c *Class) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.endID, c.at, c.running, c.steps = "", time.Time{}, false, nil
	c.students = map[string]*entry{}
}
