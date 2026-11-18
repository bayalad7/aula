package session

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// DefaultSessionTimeout marks a session disconnected after this long without
// any message from the agent (3 missed heartbeats, docs/protocol.md §3.4).
const DefaultSessionTimeout = 15 * time.Second

// State of a session in the registry.
type State int

// Session states.
const (
	StateConnected State = iota + 1
	StateDisconnected
)

func (s State) String() string {
	switch s {
	case StateConnected:
		return "conectado"
	case StateDisconnected:
		return "desconectado"
	default:
		return "desconocido"
	}
}

// Session is a snapshot of one agent connection.
type Session struct {
	ID           string // transport connection ID
	Channel      string
	RemoteAddr   string
	Hostname     string
	Username     string
	OS           protocol.OS
	OSVersion    string
	AgentVersion string
	Capabilities []protocol.Capability
	// MACs are the hello.macs, to wake the computer later (§5.15.2).
	MACs        []string
	ConnectedAt time.Time
	LastSeen    time.Time
	State       State
	// LastCodeAck is the snippet_id of the last code.ack ("" after code.clear);
	// LastCodeAckAt is zero until the agent acknowledges something.
	LastCodeAck   string
	LastCodeAckAt time.Time
	// ClockSkew is how far the agent's clock is ahead of the console's
	// (negative: behind): the median of the last ClockSamples heartbeats
	// (docs/protocol.md §5.27.6). ClockSamples is 0 until one arrives.
	ClockSkew    time.Duration
	ClockSamples int
	// DuplicateOf lists the remote addresses of the other connected sessions
	// of the channel that claim the same computer: the same hostname and user
	// (without case) or a MAC in common. Agents are not authenticated
	// (docs/protocol.md §10.3), so two of them may be one impersonating the
	// other; the console marks both and decides nothing. Nil if none.
	DuplicateOf []string

	clock []time.Duration // the last clockWindow samples, oldest first
}

// sameComputer reports whether a and b claim the same computer.
func sameComputer(a, b *Session) bool {
	if strings.EqualFold(a.Hostname, b.Hostname) && strings.EqualFold(a.Username, b.Username) {
		return true
	}
	for _, m := range a.MACs {
		if slices.ContainsFunc(b.MACs, func(n string) bool { return strings.EqualFold(m, n) }) {
			return true
		}
	}
	return false
}

// duplicatesLocked fills DuplicateOf in c, the snapshot of s. r.mu is held.
func (r *Registry) duplicatesLocked(s *Session, c *Session) {
	c.DuplicateOf = nil
	if s.State != StateConnected {
		return
	}
	for _, o := range r.sessions {
		if o != s && o.State == StateConnected && o.Channel == s.Channel && sameComputer(s, o) {
			c.DuplicateOf = append(c.DuplicateOf, o.RemoteAddr)
		}
	}
	slices.Sort(c.DuplicateOf)
}

// clockWindow is how many heartbeats the clock skew is the median of.
const clockWindow = 5

// Can reports whether the agent declared capability c.
func (s Session) Can(c protocol.Capability) bool { return protocol.Has(s.Capabilities, c) }

// Registry holds the sessions of every channel. It is safe for concurrent use.
type Registry struct {
	timeout time.Duration
	now     func() time.Time // replaced in tests

	mu       sync.Mutex
	sessions map[string]*Session
}

// NewRegistry returns an empty registry. timeout <= 0 means
// DefaultSessionTimeout.
func NewRegistry(timeout time.Duration) *Registry {
	if timeout <= 0 {
		timeout = DefaultSessionTimeout
	}
	return &Registry{timeout: timeout, now: time.Now, sessions: map[string]*Session{}}
}

// Connect records an agent that sent hello. A disconnected session of the same
// machine and user in the channel is replaced, so a reconnecting agent shows up
// once.
func (r *Registry) Connect(id, channel, remote string, h protocol.HelloInfo) Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	for oldID, s := range r.sessions {
		if s.State == StateDisconnected && s.Channel == channel &&
			s.Hostname == h.Hostname && s.Username == h.Username {
			delete(r.sessions, oldID)
		}
	}
	now := r.now()
	s := &Session{
		ID:           id,
		Channel:      channel,
		RemoteAddr:   remote,
		Hostname:     h.Hostname,
		Username:     h.Username,
		OS:           h.OS,
		OSVersion:    h.OSVersion,
		AgentVersion: h.AgentVersion,
		Capabilities: slices.Clone(h.Capabilities),
		MACs:         slices.Clone(h.MACs),
		ConnectedAt:  now,
		LastSeen:     now,
		State:        StateConnected,
	}
	r.sessions[id] = s
	return r.snapshotLocked(s)
}

// Touch records a sign of life (any message) from a connected session. It
// returns false if the session is unknown or already disconnected.
func (r *Registry) Touch(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok || s.State != StateConnected {
		return false
	}
	s.LastSeen = r.now()
	return true
}

// RecordCodeAck notes that the agent showed snippetID ("" for code.clear).
func (r *Registry) RecordCodeAck(id, snippetID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok {
		return false
	}
	s.LastCodeAck, s.LastCodeAckAt = snippetID, r.now()
	return true
}

// RecordClock notes the send time ts (Unix ms) of a heartbeat the agent
// just sent: the skew of this sample is ts minus the arrival time, and the
// session's ClockSkew is the median of the last five, so one slow packet
// does not move it.
func (r *Registry) RecordClock(id string, ts int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok || ts <= 0 {
		return false
	}
	s.clock = append(s.clock, time.UnixMilli(ts).Sub(r.now()))
	if len(s.clock) > clockWindow {
		s.clock = s.clock[len(s.clock)-clockWindow:]
	}
	sorted := slices.Clone(s.clock)
	slices.Sort(sorted)
	s.ClockSkew, s.ClockSamples = sorted[len(sorted)/2], len(sorted)
	return true
}

// Disconnect marks a session disconnected. It reports whether its state
// changed.
func (r *Registry) Disconnect(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok || s.State == StateDisconnected {
		return false
	}
	s.State = StateDisconnected
	return true
}

// Sweep marks disconnected every connected session silent for longer than the
// timeout, and returns their IDs.
func (r *Registry) Sweep() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	var stale []string
	for id, s := range r.sessions {
		if s.State == StateConnected && now.Sub(s.LastSeen) > r.timeout {
			s.State = StateDisconnected
			stale = append(stale, id)
		}
	}
	slices.Sort(stale)
	return stale
}

// Get returns a snapshot of one session.
func (r *Registry) Get(id string) (Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok {
		return Session{}, false
	}
	return r.snapshotLocked(s), true
}

// List returns snapshots of the sessions of a channel, sorted by hostname and
// user.
func (r *Registry) List(channel string) []Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Session
	for _, s := range r.sessions {
		if s.Channel == channel {
			out = append(out, r.snapshotLocked(s))
		}
	}
	slices.SortFunc(out, func(a, b Session) int {
		if c := strings.Compare(a.Hostname, b.Hostname); c != 0 {
			return c
		}
		if c := strings.Compare(a.Username, b.Username); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func (s *Session) snapshot() Session {
	c := *s
	c.Capabilities = slices.Clone(s.Capabilities)
	c.MACs = slices.Clone(s.MACs)
	c.clock = nil
	return c
}

// snapshotLocked is s.snapshot with DuplicateOf. r.mu is held.
func (r *Registry) snapshotLocked(s *Session) Session {
	c := s.snapshot()
	r.duplicatesLocked(s, &c)
	return c
}
