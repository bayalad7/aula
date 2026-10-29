package policy

import (
	"slices"
	"strings"
	"sync"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// Kind is one of the two restrictions.
type Kind string

// Kinds.
const (
	KindApps Kind = "apps"
	KindWeb  Kind = "web"
)

func (k Kind) capability() protocol.Capability {
	if k == KindApps {
		return protocol.CapAppBlock
	}
	return protocol.CapWebFilter
}

// Command is a message the console must send to one agent.
type Command struct {
	SessionID string
	Kind      Kind
	Payload   protocol.Payload
}

// classRule is a restriction of the class and whom it applies to.
type classRule struct {
	set   protocol.Payload // AppBlockSet or WebFilterSet
	id    string
	hosts map[string]bool // nil: the whole channel
}

// ClassPolicy is the console side of the restrictions: what the class has,
// who must have it, what each agent was sent and what it acknowledged. Plan
// sends the policy to agents that join (a student who reconnects gets it
// again) and the clear to those that no longer must have it. Students are
// identified by hostname, which survives reconnections.
type ClassPolicy struct {
	mu    sync.Mutex
	rules map[Kind]*classRule
	sent  map[string]map[Kind]string // session → kind → policy sent ("" a clear)
	acked map[string]map[Kind]string // session → kind → policy acknowledged active ("" cleared)
}

// NewClassPolicy returns a class without restrictions.
func NewClassPolicy() *ClassPolicy {
	return &ClassPolicy{rules: map[Kind]*classRule{}, sent: map[string]map[Kind]string{}, acked: map[string]map[Kind]string{}}
}

func hostSet(hosts []string) map[string]bool {
	if len(hosts) == 0 {
		return nil
	}
	m := map[string]bool{}
	for _, h := range hosts {
		m[strings.ToLower(h)] = true
	}
	return m
}

// SetApps makes p the app policy of the class, for the given hostnames (none:
// the whole channel). Plan sends it.
func (c *ClassPolicy) SetApps(p protocol.AppBlockSet, hosts []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules[KindApps] = &classRule{set: p, id: p.PolicyID, hosts: hostSet(hosts)}
}

// SetWeb makes p the web filter of the class, for the given hostnames.
func (c *ClassPolicy) SetWeb(p protocol.WebFilterSet, hosts []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules[KindWeb] = &classRule{set: p, id: p.PolicyID, hosts: hostSet(hosts)}
}

// Clear removes a restriction of the class; Plan sends the clears.
func (c *ClassPolicy) Clear(k Kind) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rules, k)
}

// Rule is a restriction of the class as the console shows it.
type Rule struct {
	PolicyID string              `json:"policyId"`
	Mode     protocol.PolicyMode `json:"mode"`
	Items    []string            `json:"items"`
	Hosts    []string            `json:"hosts"` // empty: the whole channel
}

// Rules returns the restrictions of the class.
func (c *ClassPolicy) Rules() map[Kind]*Rule {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[Kind]*Rule{}
	for k, r := range c.rules {
		rule := &Rule{PolicyID: r.id, Hosts: []string{}}
		switch p := r.set.(type) {
		case protocol.AppBlockSet:
			rule.Mode, rule.Items = p.Mode, p.Apps
		case protocol.WebFilterSet:
			rule.Mode, rule.Items = p.Mode, p.Domains
		}
		for h := range r.hosts {
			rule.Hosts = append(rule.Hosts, h)
		}
		slices.Sort(rule.Hosts)
		out[k] = rule
	}
	return out
}

// Plan returns what must be sent so every connected agent has the
// restrictions it should, and forgets agents that left. Record each command
// sent with Sent.
func (c *ClassPolicy) Plan(sessions []session.Session) []Command {
	c.mu.Lock()
	defer c.mu.Unlock()
	live := map[string]bool{}
	var cmds []Command
	for _, s := range sessions {
		if s.State != session.StateConnected {
			continue
		}
		live[s.ID] = true
		for _, k := range []Kind{KindApps, KindWeb} {
			if !s.Can(k.capability()) {
				continue
			}
			want := ""
			r := c.rules[k]
			if r != nil && (r.hosts == nil || r.hosts[strings.ToLower(s.Hostname)]) {
				want = r.id
			}
			have, known := c.sent[s.ID][k]
			switch {
			case want != "" && have != want:
				cmds = append(cmds, Command{SessionID: s.ID, Kind: k, Payload: r.set})
			case want == "" && known && have != "":
				var clear protocol.Payload = protocol.AppBlockClear{}
				if k == KindWeb {
					clear = protocol.WebFilterClear{}
				}
				cmds = append(cmds, Command{SessionID: s.ID, Kind: k, Payload: clear})
			}
		}
	}
	for id := range c.sent {
		if !live[id] {
			delete(c.sent, id)
			delete(c.acked, id)
		}
	}
	return cmds
}

// Sent records that cmd was sent.
func (c *ClassPolicy) Sent(cmd Command) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := ""
	switch p := cmd.Payload.(type) {
	case protocol.AppBlockSet:
		id = p.PolicyID
	case protocol.WebFilterSet:
		id = p.PolicyID
	}
	if c.sent[cmd.SessionID] == nil {
		c.sent[cmd.SessionID] = map[Kind]string{}
	}
	c.sent[cmd.SessionID][cmd.Kind] = id
}

// Ack records an agent's acknowledgement (app.block.ack or web.filter.ack).
func (c *ClassPolicy) Ack(sessionID string, p protocol.Payload) {
	var k Kind
	var ack protocol.PolicyAck
	switch a := p.(type) {
	case protocol.AppBlockAck:
		k, ack = KindApps, protocol.PolicyAck(a)
	case protocol.WebFilterAck:
		k, ack = KindWeb, protocol.PolicyAck(a)
	default:
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.acked[sessionID] == nil {
		c.acked[sessionID] = map[Kind]string{}
	}
	if ack.Active {
		c.acked[sessionID][k] = ack.PolicyID
	} else {
		c.acked[sessionID][k] = ""
	}
}

// Active reports whether the agent of sessionID acknowledged the class's
// current policy of kind k.
func (c *ClassPolicy) Active(sessionID string, k Kind) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.rules[k]
	return r != nil && c.acked[sessionID][k] == r.id
}

// Reset forgets every rule and what was sent: the class closed or changed
// channel (agents clear themselves when the connection closes). It returns
// the clears to send to the agents that have something, for a clean end.
func (c *ClassPolicy) Reset() []Command {
	c.mu.Lock()
	defer c.mu.Unlock()
	var cmds []Command
	for id, kinds := range c.sent {
		for k, policyID := range kinds {
			if policyID == "" {
				continue
			}
			var clear protocol.Payload = protocol.AppBlockClear{}
			if k == KindWeb {
				clear = protocol.WebFilterClear{}
			}
			cmds = append(cmds, Command{SessionID: id, Kind: k, Payload: clear})
		}
	}
	c.rules = map[Kind]*classRule{}
	c.sent = map[string]map[Kind]string{}
	c.acked = map[string]map[Kind]string{}
	return cmds
}
