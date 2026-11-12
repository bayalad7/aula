package screen

import (
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// AckWait is how long the console waits for a screen.freeze.ack before it
// counts a student as "sin acusar" (docs/protocol.md §5.11).
const AckWait = 5 * time.Second

// PauseStatus is how a student's screen stands for the console.
type PauseStatus string

// Pause statuses.
const (
	PauseSent     PauseStatus = "sent"     // screen.freeze sent, no ack yet
	PausePaused   PauseStatus = "paused"   // the agent acked frozen: true
	PauseResuming PauseStatus = "resuming" // screen.unfreeze sent, no ack yet
	PauseFailed   PauseStatus = "failed"   // the agent answered with an error
)

// PauseEntry is one student's pause.
type PauseEntry struct {
	Status PauseStatus
	RefID  string    // the message waiting for its ack
	SentAt time.Time // when it was sent
}

// Unacked reports whether the agent has not answered within AckWait.
func (e PauseEntry) Unacked(now time.Time) bool {
	return (e.Status == PauseSent || e.Status == PauseResuming) && now.Sub(e.SentAt) >= AckWait
}

// Pauses is the console side of screen.freeze: who was paused, who acked and
// who did not. It lives in memory and never keeps the message's history,
// only the message in force so the teacher can change it.
type Pauses struct {
	Now func() time.Time // nil: time.Now

	mu      sync.Mutex
	entries map[string]PauseEntry // by session ID
	message string                // the message of the last pause
}

func (p *Pauses) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// Sent records a screen.freeze (freeze true) or screen.unfreeze sent to a
// session as envelope refID.
func (p *Pauses) Sent(sessionID, refID string, freeze bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.entries == nil {
		p.entries = map[string]PauseEntry{}
	}
	st := PauseSent
	if !freeze {
		st = PauseResuming
	}
	p.entries[sessionID] = PauseEntry{Status: st, RefID: refID, SentAt: p.now()}
}

// SetMessage keeps the message of the pause in force.
func (p *Pauses) SetMessage(m string) {
	p.mu.Lock()
	p.message = m
	p.mu.Unlock()
}

// Message is the message of the pause in force.
func (p *Pauses) Message() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.message
}

// Ack applies a screen.freeze.ack. It reports whether something changed; an
// ack of an older message is ignored.
func (p *Pauses) Ack(sessionID string, a protocol.ScreenFreezeAck) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[sessionID]
	if !ok || e.RefID != a.RefID {
		return false
	}
	if !a.Frozen {
		delete(p.entries, sessionID)
		return true
	}
	e.Status = PausePaused
	p.entries[sessionID] = e
	return true
}

// Error applies an error that answers a screen.freeze or screen.unfreeze.
func (p *Pauses) Error(sessionID, refID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[sessionID]
	if !ok || e.RefID != refID {
		return false
	}
	if e.Status == PauseResuming {
		delete(p.entries, sessionID) // nothing is up there
		return true
	}
	e.Status = PauseFailed
	p.entries[sessionID] = e
	return true
}

// Get returns a session's pause.
func (p *Pauses) Get(sessionID string) (PauseEntry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[sessionID]
	return e, ok
}

// Active reports whether a session is paused or about to be: what «Reanudar»
// must undo.
func (p *Pauses) Active(sessionID string) bool {
	e, ok := p.Get(sessionID)
	return ok && (e.Status == PauseSent || e.Status == PausePaused)
}

// Prune forgets the sessions that are not connected any more: a session
// that reconnects starts without a pause, because its agent removed it
// (docs/privacidad.md rule 10). It reports whether something was forgotten.
func (p *Pauses) Prune(connected map[string]bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	changed := false
	for id := range p.entries {
		if !connected[id] {
			delete(p.entries, id)
			changed = true
		}
	}
	if len(p.entries) == 0 {
		p.message = ""
	}
	return changed
}

// Reset forgets everything (another class).
func (p *Pauses) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries, p.message = nil, ""
}

// PauseSummary is what the global strip shows.
type PauseSummary struct {
	Paused  int    `json:"paused"`  // paused or about to be
	Unacked int    `json:"unacked"` // of those, without an ack after AckWait
	Failed  int    `json:"failed"`  // the agent could not show it
	Message string `json:"message"` // the message in force
}

// Summary counts the pauses.
func (p *Pauses) Summary() PauseSummary {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	s := PauseSummary{Message: p.message}
	for _, e := range p.entries {
		switch e.Status {
		case PauseSent, PausePaused:
			s.Paused++
		case PauseFailed:
			s.Failed++
		}
		if e.Status == PauseSent && e.Unacked(now) {
			s.Unacked++
		}
	}
	return s
}
