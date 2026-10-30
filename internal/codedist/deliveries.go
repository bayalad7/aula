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
	ID           string `json:"id"`
	Kind         string `json:"kind"`  // "code", "clear" or "notice"
	Label        string `json:"label"` // e.g. "Ejercicio 2 · Factorial"
	Sent         int    `json:"sent"`  // agents it was sent to
	Acked        int    `json:"acked"` // code.ack / msg.ack received
	Failed       int    `json:"failed"`
	NoCapability int    `json:"noCapability"` // connected agents lacking the capability
	// SimSent and SimAcked count the simulated students apart: they never
	// add to Sent or Acked (docs/v1/ux/profesor, aula simulada).
	SimSent  int `json:"simSent"`
	SimAcked int `json:"simAcked"`
	// Pending lists the sessions (real or simulated) that have not
	// acknowledged it yet, so the console can say who is missing.
	Pending []string  `json:"pending"`
	At      time.Time `json:"at"`

	pending map[string]pendingRef // envelope ID → who still owes the ack
}

// pendingRef is an envelope awaiting its ack.
type pendingRef struct {
	session   string
	simulated bool
}

// Deliveries counts the acks of recent broadcasts by their envelope IDs. It is
// safe for concurrent use: sends come from the UI, acks from the console.
type Deliveries struct {
	mu    sync.Mutex
	seq   int
	list  []*Delivery
	byRef map[string]*Delivery
}

// Start records a broadcast: sent maps each session reached to the ID of the
// envelope it got; their acks will reference those IDs.
func (d *Deliveries) Start(kind, label string, sent map[string]string, failed, noCapability int, now time.Time) Delivery {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.byRef == nil {
		d.byRef = map[string]*Delivery{}
	}
	d.seq++
	del := &Delivery{
		ID: "d" + strconv.Itoa(d.seq), Kind: kind, Label: label, Sent: len(sent),
		Failed: failed, NoCapability: noCapability, At: now, pending: map[string]pendingRef{},
	}
	for session, env := range sent {
		del.pending[env] = pendingRef{session: session}
		d.byRef[env] = del
	}
	d.list = append(d.list, del)
	if len(d.list) > maxDeliveries {
		for env := range d.list[0].pending {
			delete(d.byRef, env)
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
	ref := del.pending[refID]
	delete(d.byRef, refID)
	delete(del.pending, refID)
	if ref.simulated {
		del.SimAcked++
	} else {
		del.Acked++
	}
	return del.snapshot(), true
}

// AddSimulated records that the delivery with ID id also went to simulated
// students (session → envelope ID). Their acks count in SimAcked.
func (d *Deliveries) AddSimulated(id string, sent map[string]string) (Delivery, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	del := d.find(id)
	if del == nil {
		return Delivery{}, false
	}
	for session, env := range sent {
		del.pending[env] = pendingRef{session: session, simulated: true}
		d.byRef[env] = del
	}
	del.SimSent += len(sent)
	return del.snapshot(), true
}

// Resent records that the delivery was sent again to some of the sessions
// that had not acknowledged it (session → new envelope ID). The old envelopes
// of those sessions stop counting, so a late ack of the first send is not
// counted twice; sessions not still pending are ignored.
func (d *Deliveries) Resent(id string, sent map[string]string) (Delivery, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	del := d.find(id)
	if del == nil {
		return Delivery{}, false
	}
	for env, ref := range del.pending {
		newEnv, again := sent[ref.session]
		if !again {
			continue
		}
		delete(del.pending, env)
		delete(d.byRef, env)
		del.pending[newEnv] = ref
		d.byRef[newEnv] = del
	}
	return del.snapshot(), true
}

// Get returns one tracked delivery.
func (d *Deliveries) Get(id string) (Delivery, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if del := d.find(id); del != nil {
		return del.snapshot(), true
	}
	return Delivery{}, false
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

// find returns the delivery with ID id; d.mu is held.
func (d *Deliveries) find(id string) *Delivery {
	for _, del := range d.list {
		if del.ID == id {
			return del
		}
	}
	return nil
}

func (del *Delivery) snapshot() Delivery {
	c := *del
	c.pending = nil
	c.Pending = make([]string, 0, len(del.pending))
	for _, ref := range del.pending {
		c.Pending = append(c.Pending, ref.session)
	}
	slices.Sort(c.Pending)
	return c
}
