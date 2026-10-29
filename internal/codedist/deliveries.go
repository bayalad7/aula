package codedist

import (
	"slices"
	"strconv"
	"sync"
	"time"
)

// maxDeliveries is how many recent sends are tracked for acknowledgements.
const maxDeliveries = 20

// Delivery is one broadcast and how many agents confirmed it.
type Delivery struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`  // "code", "clear" or "notice"
	Label        string    `json:"label"` // e.g. "Ejercicio 2 · Factorial"
	Sent         int       `json:"sent"`  // agents it was sent to
	Acked        int       `json:"acked"` // code.ack / msg.ack received
	Failed       int       `json:"failed"`
	NoCapability int       `json:"noCapability"` // connected agents lacking the capability
	At           time.Time `json:"at"`

	pending map[string]bool // envelope IDs still awaiting their ack
}

// Deliveries counts the acks of recent broadcasts by their envelope IDs. It is
// safe for concurrent use: sends come from the UI, acks from the console.
type Deliveries struct {
	mu    sync.Mutex
	seq   int
	list  []*Delivery
	byRef map[string]*Delivery
}

// Start records a broadcast: envelopeIDs are the IDs of the envelopes sent,
// one per agent; their acks will reference them.
func (d *Deliveries) Start(kind, label string, envelopeIDs []string, failed, noCapability int, now time.Time) Delivery {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.byRef == nil {
		d.byRef = map[string]*Delivery{}
	}
	d.seq++
	del := &Delivery{
		ID: "d" + strconv.Itoa(d.seq), Kind: kind, Label: label, Sent: len(envelopeIDs),
		Failed: failed, NoCapability: noCapability, At: now, pending: map[string]bool{},
	}
	for _, id := range envelopeIDs {
		del.pending[id] = true
		d.byRef[id] = del
	}
	d.list = append(d.list, del)
	if len(d.list) > maxDeliveries {
		for id := range d.list[0].pending {
			delete(d.byRef, id)
		}
		d.list = slices.Clone(d.list[1:])
	}
	return del.snapshot()
}

// Ack counts the acknowledgement of envelope refID. Each envelope counts once;
// unknown or repeated references return false.
func (d *Deliveries) Ack(refID string) (Delivery, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	del, ok := d.byRef[refID]
	if !ok {
		return Delivery{}, false
	}
	delete(d.byRef, refID)
	delete(del.pending, refID)
	del.Acked++
	return del.snapshot(), true
}

// Recent returns the tracked deliveries, newest first.
func (d *Deliveries) Recent() []Delivery {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]Delivery, 0, len(d.list))
	for i := len(d.list) - 1; i >= 0; i-- {
		out = append(out, d.list[i].snapshot())
	}
	return out
}

func (del *Delivery) snapshot() Delivery {
	c := *del
	c.pending = nil
	return c
}
