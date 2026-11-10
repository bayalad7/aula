package openapp

import (
	"slices"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// MaxOrders is how many orders the console keeps for the panel.
const MaxOrders = 5

// Console-side statuses of a target besides opened and failed.
const (
	StatusSent  = "sent"  // no ack yet
	StatusError = "error" // the agent answered error
)

// Target is one session an order went to.
type Target struct {
	SessionID string              `json:"sessionId"`
	Hostname  string              `json:"hostname"`
	Status    string              `json:"status"` // StatusSent, "opened", "failed" or StatusError
	Reason    protocol.OpenReason `json:"reason,omitempty"`
	envID     string
}

// Order is one «Abrir» of the teacher: what and to whom.
type Order struct {
	ID          string    `json:"id"` // open_id
	Kind        string    `json:"kind"`
	Label       string    `json:"label"` // the URL or the app's name, for the teacher only
	At          time.Time `json:"at"`
	Targets     []Target  `json:"targets"`
	Unsupported []string  `json:"unsupported"` // hostnames whose agent cannot open it
	Failed      []string  `json:"failed"`      // hostnames the message did not reach
	// Counts, for the panel.
	Opened  int `json:"opened"`
	Pending int `json:"pending"`
	Refused int `json:"refused"` // failed or error
}

func (o *Order) count() {
	o.Opened, o.Pending, o.Refused = 0, 0, 0
	for _, t := range o.Targets {
		switch t.Status {
		case string(protocol.OpenOpened):
			o.Opened++
		case StatusSent:
			o.Pending++
		default:
			o.Refused++
		}
	}
}

// Tracker is the console's record of the last orders and their acks.
type Tracker struct {
	mu     sync.Mutex
	orders []*Order // newest first
}

// Start records a new order before it is sent.
func (t *Tracker) Start(id, kind, label string, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	o := &Order{ID: id, Kind: kind, Label: label, At: at, Targets: []Target{}, Unsupported: []string{}, Failed: []string{}}
	t.orders = append([]*Order{o}, t.orders...)
	if len(t.orders) > MaxOrders {
		t.orders = t.orders[:MaxOrders]
	}
}

func (t *Tracker) find(id string) *Order {
	for _, o := range t.orders {
		if o.ID == id {
			return o
		}
	}
	return nil
}

// Sent records that the order went to a session in envelope envID.
func (t *Tracker) Sent(id, sessionID, hostname, envID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if o := t.find(id); o != nil {
		o.Targets = append(o.Targets, Target{SessionID: sessionID, Hostname: hostname, Status: StatusSent, envID: envID})
		o.count()
	}
}

// Skipped records a session the order did not go to: unsupported (its agent
// cannot open it) or failed (the message did not reach it).
func (t *Tracker) Skipped(id, hostname string, unsupported bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if o := t.find(id); o != nil {
		if unsupported {
			o.Unsupported = append(o.Unsupported, hostname)
		} else {
			o.Failed = append(o.Failed, hostname)
		}
	}
}

// Ack applies an open.ack; it reports whether it was for an order on record.
func (t *Tracker) Ack(sessionID string, a protocol.OpenAck) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	o := t.find(a.OpenID)
	if o == nil {
		return false
	}
	for i := range o.Targets {
		if o.Targets[i].SessionID == sessionID && o.Targets[i].Status == StatusSent {
			o.Targets[i].Status, o.Targets[i].Reason = string(a.Status), a.Reason
			o.count()
			return true
		}
	}
	return false
}

// Error applies an error whose ref_id is refID; it reports whether it
// answered an order on record.
func (t *Tracker) Error(sessionID, refID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, o := range t.orders {
		for i := range o.Targets {
			if tg := &o.Targets[i]; tg.SessionID == sessionID && tg.envID == refID && tg.Status == StatusSent {
				tg.Status = StatusError
				o.count()
				return true
			}
		}
	}
	return false
}

// Recent returns copies of the orders, newest first.
func (t *Tracker) Recent() []Order {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Order, 0, len(t.orders))
	for _, o := range t.orders {
		c := *o
		c.Targets = slices.Clone(o.Targets)
		c.Unsupported = slices.Clone(o.Unsupported)
		c.Failed = slices.Clone(o.Failed)
		out = append(out, c)
	}
	return out
}

// Clear forgets every order (the class closed).
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.orders = nil
}
