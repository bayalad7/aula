package policy

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// upstreamsEvery is how often the computer's DNS servers are read again
// while the filter is on (the laptop may change network).
const upstreamsEvery = 30 * time.Second

// State is what the student's notice shows (docs/privacidad.md rules 20-21).
type State struct {
	Apps *Summary `json:"apps"` // nil without an app policy
	Web  *Summary `json:"web"`  // nil without a web filter
}

// Active reports whether any restriction is on.
func (s State) Active() bool { return s.Apps != nil || s.Web != nil }

// Manager applies the class restrictions on the agent and always undoes them
// (D-017). Apps or the web filter are supported only when their pieces are
// set; Capabilities says which.
type Manager struct {
	Apps *AppEnforcer // nil: no app.block

	DNS          platform.DNSRedirect // nil: no web.filter
	DoH          platform.BrowserDoH
	Journal      Journal
	ResolverAddr string // DefaultResolverAddr if empty
	upstreamPort int    // tests

	// OnChange is called with the new state before it is applied, so the
	// student sees the notice first, and again if applying fails.
	OnChange func(State)
	Logger   *slog.Logger

	mu       sync.Mutex
	app      *AppPolicy
	web      *WebPolicy
	resolver *Resolver
	saved    []platform.SavedSetting

	// Read by the resolver's goroutines without m.mu: teardown holds m.mu
	// while it shuts the resolver down, which waits for them.
	webNow    atomic.Pointer[WebPolicy]
	upMu      sync.Mutex
	upstreams platform.DNSConfig
	readAt    time.Time
}

// Capabilities lists what this manager lets the agent declare.
func (m *Manager) Capabilities() []protocol.Capability {
	var caps []protocol.Capability
	if m.Apps != nil {
		caps = append(caps, protocol.CapAppBlock)
	}
	if m.DNS != nil && m.DoH != nil {
		caps = append(caps, protocol.CapWebFilter)
	}
	return caps
}

// State returns the restrictions in force.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stateLocked()
}

func (m *Manager) stateLocked() State {
	var s State
	if m.app != nil {
		s.Apps = summarize(m.app.ID, m.app.Mode, m.app.Apps)
	}
	if m.web != nil {
		s.Web = summarize(m.web.ID, m.web.Mode, m.web.Domains)
	}
	return s
}

// Handle processes app.block.* and web.filter.*; it reports false for any
// other message. reply is the ack, or an error if nothing was applied.
func (m *Manager) Handle(env protocol.Envelope) (reply protocol.Payload, handled bool) {
	switch p := env.Payload.(type) {
	case protocol.AppBlockSet:
		if m.Apps == nil {
			return unsupported(env, protocol.CapAppBlock), true
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.app = NewAppPolicy(p)
		m.changed()
		m.Apps.SetPolicy(m.app)
		m.log().Info("app policy applied", "policy", p.PolicyID, "mode", p.Mode, "apps", len(p.Apps))
		return protocol.AppBlockAck{RefID: env.ID, PolicyID: p.PolicyID, Active: true}, true
	case protocol.AppBlockClear:
		if m.Apps == nil {
			return unsupported(env, protocol.CapAppBlock), true
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.clearApps("clear")
		return protocol.AppBlockAck{RefID: env.ID}, true
	case protocol.WebFilterSet:
		if m.DNS == nil || m.DoH == nil {
			return unsupported(env, protocol.CapWebFilter), true
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if err := m.applyWeb(NewWebPolicy(p)); err != nil {
			m.log().Warn("web filter not applied", "policy", p.PolicyID, "err", err)
			return protocol.NewError(env.ID, protocol.CodeInternal, "no se pudo aplicar el filtro web; el equipo quedó como estaba"), true
		}
		m.log().Info("web filter applied", "policy", p.PolicyID, "mode", p.Mode, "domains", len(p.Domains))
		return protocol.WebFilterAck{RefID: env.ID, PolicyID: p.PolicyID, Active: true}, true
	case protocol.WebFilterClear:
		if m.DNS == nil || m.DoH == nil {
			return unsupported(env, protocol.CapWebFilter), true
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if err := m.clearWeb("clear"); err != nil {
			m.log().Warn("web filter not fully removed", "err", err)
			return protocol.NewError(env.ID, protocol.CodeInternal, "no se pudo quitar del todo el filtro web; se reintentará al reiniciar el agente"), true
		}
		return protocol.WebFilterAck{RefID: env.ID}, true
	}
	return nil, false
}

func unsupported(env protocol.Envelope, c protocol.Capability) protocol.Payload {
	return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta "+string(c))
}

// Clear removes every restriction: the connection with the console closed,
// the helper left, or the service stops (docs/privacidad.md rule 24).
func (m *Manager) Clear(why string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Apps != nil {
		m.clearApps(why)
	}
	if m.DNS != nil && m.DoH != nil {
		return m.clearWeb(why)
	}
	return nil
}

// Restore undoes a web filter left by a crash or a power cut, before the
// agent connects: the NRPT rules tagged as Aula's and the browser policies
// saved in the journal.
func (m *Manager) Restore() error {
	if m.DNS == nil || m.DoH == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok, readErr := m.Journal.Read()
	errs := []error{readErr, m.DNS.Clear()} // also rules whose journal was lost
	if ok {
		m.log().Warn("restoring a web filter left by a previous run", "since", rec.Since)
		if len(rec.DoH) > 0 {
			errs = append(errs, m.DoH.Restore(rec.DoH))
		}
	}
	errs = append(errs, m.DNS.Flush())
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	return m.Journal.Remove()
}

// changed tells the notice; m.mu is held.
func (m *Manager) changed() {
	if m.OnChange != nil {
		m.OnChange(m.stateLocked())
	}
}

func (m *Manager) clearApps(why string) {
	if m.app == nil {
		return
	}
	m.Apps.SetPolicy(nil)
	m.app = nil
	m.changed()
	m.log().Info("app policy removed", "why", why)
}

// applyWeb turns the filter on, or replaces its list. Every change to the
// computer is journaled first; if a step fails, everything is undone.
// m.mu is held.
func (m *Manager) applyWeb(p *WebPolicy) error {
	prev := m.web
	m.web = p
	m.webNow.Store(p)
	m.changed() // the notice before the filter (rule 20)
	if prev != nil {
		return m.DNS.Flush() // already on: only the list changes
	}
	fail := func(err error) error {
		if uerr := m.teardown(); uerr != nil {
			err = errors.Join(err, uerr)
		}
		m.web = nil
		m.webNow.Store(nil)
		m.changed()
		return err
	}
	if cfg, err := m.DNS.Upstreams(); err == nil {
		m.upMu.Lock()
		m.upstreams, m.readAt = cfg, time.Now()
		m.upMu.Unlock()
	} else {
		return fail(fmt.Errorf("dns servers: %w", err))
	}
	addr := m.ResolverAddr
	if addr == "" {
		addr = DefaultResolverAddr
	}
	r := &Resolver{Addr: addr, Policy: m.currentWeb, Upstreams: m.currentUpstreams, UpstreamPort: m.upstreamPort}
	if err := r.Start(); err != nil {
		return fail(err)
	}
	m.resolver = r
	saved, err := m.DoH.Snapshot()
	if err != nil {
		return fail(err)
	}
	if err := m.Journal.Write(Record{NRPT: true, DoH: saved, Since: time.Now()}); err != nil {
		return fail(err)
	}
	m.saved = saved
	if err := m.DoH.Disable(); err != nil {
		return fail(err)
	}
	host, _, _ := cutHostPort(r.LocalAddr())
	if err := m.DNS.Apply(host); err != nil {
		return fail(err)
	}
	if err := m.DNS.Flush(); err != nil {
		return fail(err)
	}
	return nil
}

// clearWeb turns the filter off; m.mu is held.
func (m *Manager) clearWeb(why string) error {
	if m.web == nil && m.resolver == nil {
		return nil
	}
	m.web = nil
	m.webNow.Store(nil)
	m.changed()
	err := m.teardown()
	m.log().Info("web filter removed", "why", why, "clean", err == nil)
	return err
}

// teardown undoes the computer changes in reverse order. The journal is
// removed only if everything was undone, so a failure is retried at the
// next start. m.mu is held.
func (m *Manager) teardown() error {
	errs := []error{m.DNS.Clear()}
	if m.saved != nil {
		errs = append(errs, m.DoH.Restore(m.saved))
	}
	errs = append(errs, m.DNS.Flush())
	if m.resolver != nil {
		_ = m.resolver.Close()
		m.resolver = nil
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	m.saved = nil
	return m.Journal.Remove()
}

func (m *Manager) currentWeb() *WebPolicy { return m.webNow.Load() }

// currentUpstreams reads the DNS servers again every 30 s. It runs on the
// resolver's goroutines.
func (m *Manager) currentUpstreams() platform.DNSConfig {
	m.upMu.Lock()
	defer m.upMu.Unlock()
	if time.Since(m.readAt) > upstreamsEvery && m.DNS != nil {
		if fresh, err := m.DNS.Upstreams(); err == nil {
			m.upstreams = fresh
		}
		m.readAt = time.Now() // also on failure: keep the last ones, retry later
	}
	return m.upstreams
}

func cutHostPort(addr string) (host, port string, ok bool) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], true
		}
	}
	return addr, "", false
}

func (m *Manager) log() *slog.Logger {
	if m.Logger != nil {
		return m.Logger
	}
	return slog.New(slog.DiscardHandler)
}
