// Package connectcheck is the console's connection checker (prompt 18,
// Fase 6.3): from the teacher's computer it walks the path a student takes to
// reach this console (mDNS announcement, TCP port, TLS and the site key) and
// reads this computer's network profile and firewall, and says at which step
// a student would fail. Each failure carries the same session.Cause the agent
// reports for it.
//
// It never calls OS APIs: the network profile and firewall come through
// platform.NetworkInspector. What can only be seen from the student's
// computer (its channel, its key, its firewall) is left to the "¿Falta un
// alumno?" questions and to "aula-cliente diagnostico".
package connectcheck

import (
	"context"
	"crypto/ed25519"
	"errors"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/session"
)

// Status of a step.
type Status string

// Step statuses.
const (
	StatusPending Status = "pending" // not started
	StatusRunning Status = "running"
	StatusOK      Status = "ok"
	StatusWarn    Status = "warn"    // a student can connect, with a caveat
	StatusFail    Status = "fail"    // a student would fail here
	StatusUnknown Status = "unknown" // cannot be checked from here
)

// StepID names a step; the interface has a text per step and status.
type StepID string

// The steps, in the order they run.
const (
	StepAnnounce StepID = "announce" // the channel's announcement is on the network
	StepPort     StepID = "port"     // the TCP port answers on the LAN addresses
	StepSecure   StepID = "secure"   // TLS and the site key, as an agent
	StepNetwork  StepID = "network"  // network profile and firewall of this computer
)

// Reasons refine a status for the interface text.
const (
	ReasonOtherConsoles = "otherConsoles" // announce warn: more consoles on the channel
	ReasonBrowseError   = "browseError"   // announce fail: mDNS could not start
	ReasonNoAddress     = "noAddress"     // port fail: no LAN address to try
	ReasonSomeFailed    = "someFailed"    // port warn: some addresses did not answer
	ReasonSkipped       = "skipped"       // secure unknown: nothing answered
	ReasonPublic        = "public"        // network warn: a Public network the firewall allows
	ReasonPublicBlocked = "publicBlocked" // network fail: a Public network, blocked
	ReasonBlocked       = "blocked"       // network fail: the firewall blocks the console
	ReasonNoNetwork     = "noNetwork"     // network warn: no connected network
	ReasonNotSupported  = "notSupported"  // network unknown: not on this OS
	ReasonInspectError  = "inspectError"  // network unknown: reading failed
)

// Step is one step's result. The interface text of a step is keyed by
// "<id>.<reason>", else "<id>.<cause>", else "<id>.<status>". The fields
// after Reason are the values that text shows.
type Step struct {
	ID     StepID        `json:"id"`
	Status Status        `json:"status"`
	Cause  session.Cause `json:"cause,omitempty"`
	Reason string        `json:"reason,omitempty"`
	// Addrs are the addresses involved: those that answered (ok) or failed.
	Addrs []string `json:"addrs,omitempty"`
	Port  int      `json:"port,omitempty"`
	// Others are the names of other consoles announcing this channel.
	Others []string `json:"others,omitempty"`
	// Networks are the networks a network step is about.
	Networks []string `json:"networks,omitempty"`
}

// Statuses lists every status, for the interface texts.
var Statuses = []Status{StatusPending, StatusRunning, StatusOK, StatusWarn, StatusFail, StatusUnknown}

// Outcomes lists, per step, every "<reason | cause | status>" it can end
// with: the console needs a text for each (apps/teacher textos test), and
// this package's tests check no step ends with another.
var Outcomes = map[StepID][]string{
	StepAnnounce: {string(StatusOK), ReasonOtherConsoles, string(session.CauseNoConsole), ReasonBrowseError},
	StepPort:     {string(StatusOK), ReasonSomeFailed, string(session.CauseUnreachable), ReasonNoAddress},
	StepSecure: {string(StatusOK), ReasonSkipped, string(session.CauseCertMismatch), string(session.CauseWrongKey),
		string(session.CauseAuthIncomplete), string(session.CauseUnreachable)},
	StepNetwork: {string(StatusOK), ReasonPublic, ReasonPublicBlocked, ReasonBlocked, ReasonNoNetwork,
		ReasonNotSupported, ReasonInspectError},
}

// Outcome is the "<reason | cause | status>" of a finished step.
func (s Step) Outcome() string {
	switch {
	case s.Reason != "":
		return s.Reason
	case s.Cause != session.CauseNone:
		return string(s.Cause)
	default:
		return string(s.Status)
	}
}

// Report is the whole check. Cause is the first failing step's cause.
type Report struct {
	Channel string        `json:"channel"`
	Steps   []Step        `json:"steps"`
	Cause   session.Cause `json:"cause,omitempty"`
	Done    bool          `json:"done"`
}

// Default durations.
const (
	DefaultBrowseFor   = 6 * time.Second
	DefaultDialTimeout = 3 * time.Second
	// settle is how long the browse goes on after this console was seen, to
	// notice other consoles of the channel.
	settle = 1500 * time.Millisecond
)

// Checker checks the open class of this console.
type Checker struct {
	Channel     string
	PublicKey   ed25519.PublicKey // the key agents are installed with: this console's
	Fingerprint string            // this console's certificate
	Port        int
	Program     string // this console's executable, for the firewall rules
	// LocalAddrs are this computer's addresses: they tell this console's
	// announcement apart, and are tried when it is not seen.
	LocalAddrs []net.IP
	// Source finds the consoles of Channel (discovery.Browser in the app).
	Source session.ConsoleSource
	Net    platform.NetworkInspector // nil: the network step is unknown
	// Loopback accepts 127.0.0.1 addresses (tests). In the app it is off: a
	// student cannot reach a loopback address, and the aula simulada's
	// listener lives there.
	Loopback    bool
	BrowseFor   time.Duration // 0 means DefaultBrowseFor
	DialTimeout time.Duration // 0 means DefaultDialTimeout
	Logger      *slog.Logger
}

// Run runs every step in order. progress, if set, receives a copy of the
// report whenever a step starts or ends.
func (c *Checker) Run(ctx context.Context, progress func(Report)) Report {
	r := Report{Channel: c.Channel}
	for _, id := range []StepID{StepAnnounce, StepPort, StepSecure, StepNetwork} {
		r.Steps = append(r.Steps, Step{ID: id, Status: StatusPending})
	}
	report := func() {
		if progress != nil {
			cp := r
			cp.Steps = slices.Clone(r.Steps)
			progress(cp)
		}
	}
	set := func(i int, s Step) {
		r.Steps[i] = s
		if s.Status == StatusFail && r.Cause == session.CauseNone {
			r.Cause = s.Cause
		}
		report()
	}
	running := func(i int) {
		r.Steps[i].Status = StatusRunning
		report()
	}

	running(0)
	announce, target := c.announce(ctx)
	set(0, announce)

	running(1)
	port, reachable := c.port(ctx, target)
	set(1, port)

	running(2)
	secure := Step{ID: StepSecure, Status: StatusUnknown, Reason: ReasonSkipped}
	if len(reachable) > 0 {
		t := target
		t.Addrs = reachable
		secure = c.secure(ctx, t)
	}
	set(2, secure)

	running(3)
	set(3, c.network(ctx))

	r.Done = true
	report()
	return r
}

func (c *Checker) log() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}

// usable keeps the addresses a student could use.
func (c *Checker) usable(ips []net.IP) []net.IP {
	var out []net.IP
	for _, ip := range ips {
		if ip.IsLoopback() && !c.Loopback {
			continue
		}
		out = append(out, ip)
	}
	return out
}

// own reports whether an announcement is this console's: same port and one
// of this computer's addresses.
func (c *Checker) own(con discovery.Console) bool {
	if con.Port != c.Port {
		return false
	}
	for _, a := range con.Addrs {
		if a.IsLoopback() && c.Loopback {
			return true
		}
		if slices.ContainsFunc(c.LocalAddrs, a.Equal) {
			return true
		}
	}
	return false
}

// announce browses the channel. It returns the step and the console to try
// next: the announcement as seen, or this computer's addresses if it was
// not seen.
func (c *Checker) announce(ctx context.Context) (Step, discovery.Console) {
	step := Step{ID: StepAnnounce, Status: StatusFail, Cause: session.CauseNoConsole}
	target := discovery.Console{Channel: c.Channel, Addrs: c.LocalAddrs, Port: c.Port, CertFingerprint: c.Fingerprint}
	browseFor := c.BrowseFor
	if browseFor <= 0 {
		browseFor = DefaultBrowseFor
	}
	bctx, cancel := context.WithTimeout(ctx, browseFor)
	defer cancel()
	found, err := c.Source.Browse(bctx)
	if err != nil {
		c.log().Warn("connection check: browse failed", "err", err)
		step.Reason = ReasonBrowseError
		return step, target
	}
	var (
		seen     bool
		others   = map[string]string{} // instance → name
		deadline <-chan time.Time
	)
	for done := false; !done; {
		select {
		case con, ok := <-found:
			if !ok {
				done = true
				break
			}
			if c.own(con) {
				if !seen {
					seen = true
					deadline = time.After(settle)
				}
				target = con
				continue
			}
			name := con.Name
			if name == "" {
				name = con.Instance
			}
			others[con.Instance] = name
		case <-deadline:
			done = true
		case <-bctx.Done():
			done = true
		}
	}
	if !seen {
		return step, target
	}
	step = Step{ID: StepAnnounce, Status: StatusOK, Addrs: addrStrings(c.usable(target.Addrs))}
	if len(others) > 0 {
		step.Status, step.Reason = StatusWarn, ReasonOtherConsoles
		for _, n := range others {
			step.Others = append(step.Others, n)
		}
		slices.Sort(step.Others)
	}
	return step, target
}

// port connects over TCP to each usable address of target, at once.
func (c *Checker) port(ctx context.Context, target discovery.Console) (Step, []net.IP) {
	step := Step{ID: StepPort, Port: target.Port}
	addrs := c.usable(target.Addrs)
	if len(addrs) == 0 {
		step.Status, step.Cause, step.Reason = StatusFail, session.CauseUnreachable, ReasonNoAddress
		return step, nil
	}
	timeout := c.DialTimeout
	if timeout <= 0 {
		timeout = DefaultDialTimeout
	}
	ok := make([]bool, len(addrs))
	var wg sync.WaitGroup
	for i, ip := range addrs {
		wg.Go(func() {
			d := net.Dialer{Timeout: timeout}
			conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(target.Port)))
			if err == nil {
				ok[i] = true
				_ = conn.Close()
			}
		})
	}
	wg.Wait()
	var good, bad []net.IP
	for i, ip := range addrs {
		if ok[i] {
			good = append(good, ip)
		} else {
			bad = append(bad, ip)
		}
	}
	switch {
	case len(good) == 0:
		step.Status, step.Cause, step.Addrs = StatusFail, session.CauseUnreachable, addrStrings(bad)
	case len(bad) > 0:
		step.Status, step.Reason, step.Addrs = StatusWarn, ReasonSomeFailed, addrStrings(bad)
	default:
		step.Status, step.Addrs = StatusOK, addrStrings(good)
	}
	return step, good
}

// secure does what an agent does, up to auth.ok.
func (c *Checker) secure(ctx context.Context, target discovery.Console) Step {
	probe := session.Agent{Channel: c.Channel, ConsolePublicKey: c.PublicKey, Logger: c.log()}
	cause := probe.Probe(ctx, target)
	if cause != session.CauseNone {
		return Step{ID: StepSecure, Status: StatusFail, Cause: cause}
	}
	return Step{ID: StepSecure, Status: StatusOK}
}

// network reads the network profile and the firewall.
func (c *Checker) network(ctx context.Context) Step {
	step := Step{ID: StepNetwork, Status: StatusUnknown, Reason: ReasonNotSupported, Port: c.Port}
	if c.Net == nil {
		return step
	}
	st, err := c.Net.Inspect(ctx, c.Program, c.Port)
	if errors.Is(err, platform.ErrNotSupported) {
		return step
	}
	if err != nil {
		c.log().Warn("connection check: network inspect failed", "err", err)
		step.Reason = ReasonInspectError
		return step
	}
	return Evaluate(st, c.Port)
}

// Evaluate decides the network step from the network profile and firewall:
// a network whose firewall profile blocks the console fails (the cause is
// noConsole on a Public network, the usual case, and unreachable otherwise);
// a Public network that is allowed is a warning.
func Evaluate(st platform.NetworkState, port int) Step {
	step := Step{ID: StepNetwork, Status: StatusOK, Port: port}
	if len(st.Networks) == 0 {
		step.Status, step.Reason = StatusWarn, ReasonNoNetwork
		return step
	}
	var blocked, public []string
	for _, n := range st.Networks {
		name := n.Name
		if name == "" {
			name = n.Interface
		}
		if !allowed(st, n.Category) {
			blocked = append(blocked, name)
		}
		if n.Category == platform.NetworkPublic {
			public = append(public, name)
		}
	}
	switch {
	case len(blocked) > 0 && len(public) > 0:
		step.Status, step.Cause, step.Reason, step.Networks = StatusFail, session.CauseNoConsole, ReasonPublicBlocked, public
	case len(blocked) > 0:
		step.Status, step.Cause, step.Reason, step.Networks = StatusFail, session.CauseUnreachable, ReasonBlocked, blocked
	case len(public) > 0:
		step.Status, step.Reason, step.Networks = StatusWarn, ReasonPublic, public
	}
	return step
}

// allowed reports whether the firewall profile lets students in: off, an
// allow rule and no block rule, or inbound allowed by default.
func allowed(st platform.NetworkState, profile string) bool {
	i := slices.IndexFunc(st.Firewall, func(f platform.FirewallProfile) bool { return f.Name == profile })
	if i < 0 || !st.Firewall[i].Enabled {
		return true
	}
	allow := st.Firewall[i].InboundAllowed
	for _, r := range st.Rules {
		if len(r.Profiles) > 0 && !slices.Contains(r.Profiles, profile) {
			continue
		}
		if !r.Allow {
			return false // a block rule wins
		}
		allow = true
	}
	return allow
}

func addrStrings(ips []net.IP) []string {
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip.String()
	}
	return out
}
