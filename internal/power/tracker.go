package power

import (
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Console-side statuses besides those of power.ack.
const (
	StatusSent  protocol.PowerStatus = "sent"  // no ack yet
	StatusError protocol.PowerStatus = "error" // the agent answered error
)

// Entry is the state of the last order sent to one session.
type Entry struct {
	ActionID      string               `json:"action_id"`
	Action        protocol.PowerKind   `json:"action"`
	Force         bool                 `json:"force"`
	Status        protocol.PowerStatus `json:"status"`
	DeadlineMs    int64                `json:"deadline_ms"` // Unix ms when it runs; 0 if unknown
	PostponesLeft int                  `json:"postpones_left"`
	envID         string               // envelope of the power.action, for error ref_id
}

// Pending reports whether the order may still be cancelled.
func (e Entry) Pending() bool {
	return e.Status == StatusSent || e.Status == protocol.PowerCountdown || e.Status == protocol.PowerPostponed
}

// Tracker is the console's record of power orders, by session ID.
type Tracker struct {
	Now func() time.Time // time.Now if nil

	mu sync.Mutex
	m  map[string]Entry
}

// Sent records an order sent to a session in the envelope envID.
func (t *Tracker) Sent(sessionID, envID string, a protocol.PowerAction) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.m == nil {
		t.m = map[string]Entry{}
	}
	t.m[sessionID] = Entry{ActionID: a.ActionID, Action: a.Action, Force: a.Force, Status: StatusSent,
		PostponesLeft: protocol.MaxPowerPostpones, envID: envID}
}

// Ack applies a power.ack; it reports whether it was for the order on record.
func (t *Tracker) Ack(sessionID string, a protocol.PowerAck) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.m[sessionID]
	if !ok || e.ActionID != a.ActionID {
		return false
	}
	e.Status = a.Status
	e.PostponesLeft = a.PostponesLeft
	e.DeadlineMs = 0
	if a.Status == protocol.PowerCountdown || a.Status == protocol.PowerPostponed {
		e.DeadlineMs = t.now().Add(time.Duration(a.RemainingS) * time.Second).UnixMilli()
	}
	t.m[sessionID] = e
	return true
}

// Error applies an error whose ref_id is refID; it reports whether it was
// for the order on record.
func (t *Tracker) Error(sessionID, refID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.m[sessionID]
	if !ok || e.envID != refID {
		return false
	}
	e.Status, e.DeadlineMs = StatusError, 0
	t.m[sessionID] = e
	return true
}

// Get returns the order on record for a session.
func (t *Tracker) Get(sessionID string) (Entry, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.m[sessionID]
	return e, ok
}

// Forget drops a session (it left, or the class closed).
func (t *Tracker) Forget(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.m, sessionID)
}

// Reset drops everything.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.m = nil
}

func (t *Tracker) now() time.Time {
	if t.Now == nil {
		return time.Now()
	}
	return t.Now()
}
